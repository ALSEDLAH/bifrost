// Unit tests for SCIM Groups (spec 022). Reuses helpers from
// scim_users_test.go (newSCIMTestStore, seedSCIMEnabled, seedUser).

package handlers

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// seedDefaultOrgForGroups pins the default org — same as the users
// write-test helper, duplicated here so either file can compile
// independently.
func seedDefaultOrgForGroups(t *testing.T, store configstore.ConfigStore) {
	t.Helper()
	require.NoError(t, store.DB().
		Model(&tables_enterprise.TableSystemDefaults{}).
		Where("id = ?", tables_enterprise.SystemDefaultsRowID).
		Update("default_organization_id", "org-synth").Error)
}

// seedUserInOrg is like seedUser but pins the user to org-synth so
// group-member resolution finds them.
func seedUserInOrg(t *testing.T, store configstore.ConfigStore, id, email string) {
	t.Helper()
	u := &tables_enterprise.TableUser{
		ID: id, OrganizationID: "org-synth",
		Email: email, Status: "active", DisplayName: email,
		IdpSubject: "sub-" + id,
	}
	require.NoError(t, store.DB().Create(u).Error)
}

func invokeGroups(t *testing.T, store configstore.ConfigStore, method, path, bearer, body string) (int, map[string]any) {
	t.Helper()
	h := NewSCIMGroupsHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(path)
	if bearer != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != "" {
		ctx.Request.SetBody([]byte(body))
	}
	switch method {
	case "GET":
		if id := extractGroupID(path); id != "" {
			ctx.SetUserValue("id", id)
			h.get(ctx)
		} else {
			h.list(ctx)
		}
	case "POST":
		h.create(ctx)
	case "PATCH":
		if id := extractGroupID(path); id != "" {
			ctx.SetUserValue("id", id)
		}
		h.patch(ctx)
	case "DELETE":
		if id := extractGroupID(path); id != "" {
			ctx.SetUserValue("id", id)
		}
		h.del(ctx)
	default:
		t.Fatalf("unsupported method %q", method)
	}
	var out map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &out)
	return ctx.Response.StatusCode(), out
}

func extractGroupID(path string) string {
	const prefix = "/scim/v2/Groups/"
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		return ""
	}
	return path[len(prefix):]
}

func TestSCIMGroups_MissingBearer_Returns401(t *testing.T) {
	store := newSCIMTestStore(t)
	status, _ := invokeGroups(t, store, "GET", "/scim/v2/Groups", "", "")
	if status != 401 {
		t.Errorf("expected 401; got %d", status)
	}
}

func TestSCIMGroups_CreateHappyPath(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	seedUserInOrg(t, store, "u1", "a@example.com")
	seedUserInOrg(t, store, "u2", "b@example.com")

	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],
	          "displayName":"Engineering",
	          "externalId":"ext-eng",
	          "members":[{"value":"u1"},{"value":"u2"}]}`
	status, out := invokeGroups(t, store, "POST", "/scim/v2/Groups", token, body)
	if status != 201 {
		t.Fatalf("status %d body %+v", status, out)
	}
	if out["displayName"] != "Engineering" {
		t.Errorf("displayName wrong: %+v", out["displayName"])
	}
	members, _ := out["members"].([]any)
	if len(members) != 2 {
		t.Errorf("expected 2 members; got %d (%+v)", len(members), members)
	}

	// DB rows exist.
	var cnt int64
	require.NoError(t, store.DB().Model(&tables_enterprise.TableSCIMGroupMember{}).Count(&cnt).Error)
	if cnt != 2 {
		t.Errorf("expected 2 member rows; got %d", cnt)
	}
}

func TestSCIMGroups_Create_DropsUnknownMembers(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	seedUserInOrg(t, store, "u-known", "k@example.com")

	body := `{"displayName":"Mixed",
	          "members":[{"value":"u-known"},{"value":"u-missing"}]}`
	status, out := invokeGroups(t, store, "POST", "/scim/v2/Groups", token, body)
	if status != 201 {
		t.Fatalf("status %d body %+v", status, out)
	}
	members, _ := out["members"].([]any)
	if len(members) != 1 {
		t.Errorf("expected 1 member (missing ones silently dropped); got %d", len(members))
	}
}

func TestSCIMGroups_Create_Empty_DisplayName_Returns400(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	body := `{"displayName":""}`
	status, _ := invokeGroups(t, store, "POST", "/scim/v2/Groups", token, body)
	if status != 400 {
		t.Errorf("expected 400; got %d", status)
	}
}

func TestSCIMGroups_List_Pagination(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf(`{"displayName":"G%02d"}`, i)
		status, _ := invokeGroups(t, store, "POST", "/scim/v2/Groups", token, body)
		if status != 201 {
			t.Fatalf("seed create %d failed", i)
		}
	}
	status, out := invokeGroups(t, store, "GET", "/scim/v2/Groups?startIndex=1&count=2", token, "")
	if status != 200 {
		t.Fatalf("status %d body %+v", status, out)
	}
	if out["totalResults"] != float64(5) {
		t.Errorf("totalResults wrong: %+v", out["totalResults"])
	}
	if out["itemsPerPage"] != float64(2) {
		t.Errorf("itemsPerPage wrong: %+v", out["itemsPerPage"])
	}
}

func TestSCIMGroups_GetById_NotFound(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	status, _ := invokeGroups(t, store, "GET", "/scim/v2/Groups/no-such", token, "")
	if status != 404 {
		t.Errorf("expected 404; got %d", status)
	}
}

func TestSCIMGroups_Patch_AddMembers(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	seedUserInOrg(t, store, "u1", "a@example.com")
	seedUserInOrg(t, store, "u2", "b@example.com")

	createStatus, created := invokeGroups(t, store, "POST", "/scim/v2/Groups", token,
		`{"displayName":"G","members":[{"value":"u1"}]}`)
	if createStatus != 201 {
		t.Fatalf("create failed: %+v", created)
	}
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)

	patchBody := `{"Operations":[{"op":"Add","path":"members","value":[{"value":"u2"}]}]}`
	status, out := invokeGroups(t, store, "PATCH", "/scim/v2/Groups/"+id, token, patchBody)
	if status != 200 {
		t.Fatalf("patch status %d body %+v", status, out)
	}
	members, _ := out["members"].([]any)
	if len(members) != 2 {
		t.Errorf("expected 2 members after Add; got %d", len(members))
	}
}

func TestSCIMGroups_Patch_RemoveMember_ByFilter(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	seedUserInOrg(t, store, "u1", "a@example.com")
	seedUserInOrg(t, store, "u2", "b@example.com")

	_, created := invokeGroups(t, store, "POST", "/scim/v2/Groups", token,
		`{"displayName":"G","members":[{"value":"u1"},{"value":"u2"}]}`)
	id := created["id"].(string)

	patchBody := `{"Operations":[{"op":"Remove","path":"members[value eq \"u1\"]"}]}`
	status, out := invokeGroups(t, store, "PATCH", "/scim/v2/Groups/"+id, token, patchBody)
	if status != 200 {
		t.Fatalf("patch status %d body %+v", status, out)
	}
	members, _ := out["members"].([]any)
	if len(members) != 1 {
		t.Errorf("expected 1 member left; got %d", len(members))
	}
}

func TestSCIMGroups_Patch_ReplaceDisplayName(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	_, created := invokeGroups(t, store, "POST", "/scim/v2/Groups", token,
		`{"displayName":"Old"}`)
	id := created["id"].(string)

	patchBody := `{"Operations":[{"op":"Replace","path":"displayName","value":"New"}]}`
	status, out := invokeGroups(t, store, "PATCH", "/scim/v2/Groups/"+id, token, patchBody)
	if status != 200 {
		t.Fatalf("status %d body %+v", status, out)
	}
	if out["displayName"] != "New" {
		t.Errorf("displayName not renamed: %+v", out)
	}
}

func TestSCIMGroups_Patch_UnsupportedPath_Returns400(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	_, created := invokeGroups(t, store, "POST", "/scim/v2/Groups", token,
		`{"displayName":"G"}`)
	id := created["id"].(string)

	patchBody := `{"Operations":[{"op":"Replace","path":"externalId","value":"x"}]}`
	status, _ := invokeGroups(t, store, "PATCH", "/scim/v2/Groups/"+id, token, patchBody)
	if status != 400 {
		t.Errorf("expected 400; got %d", status)
	}
}

func TestSCIMGroups_Delete_CascadesMembers(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	seedUserInOrg(t, store, "u1", "a@example.com")

	_, created := invokeGroups(t, store, "POST", "/scim/v2/Groups", token,
		`{"displayName":"G","members":[{"value":"u1"}]}`)
	id := created["id"].(string)

	status, _ := invokeGroups(t, store, "DELETE", "/scim/v2/Groups/"+id, token, "")
	if status != 204 {
		t.Errorf("expected 204; got %d", status)
	}
	var cnt int64
	store.DB().Model(&tables_enterprise.TableSCIMGroupMember{}).
		Where("group_id = ?", id).Count(&cnt)
	if cnt != 0 {
		t.Errorf("expected member rows cascaded; got %d", cnt)
	}
	// User still exists.
	var userCnt int64
	store.DB().Model(&tables_enterprise.TableUser{}).Where("id = ?", "u1").Count(&userCnt)
	if userCnt != 1 {
		t.Errorf("delete cascaded to users — it shouldn't; found %d", userCnt)
	}
}

func TestSCIMGroups_Delete_MissingId_Idempotent(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrgForGroups(t, store)
	token := seedSCIMEnabled(t, store, true)
	status, _ := invokeGroups(t, store, "DELETE", "/scim/v2/Groups/no-such", token, "")
	if status != 204 {
		t.Errorf("expected 204; got %d", status)
	}
}
