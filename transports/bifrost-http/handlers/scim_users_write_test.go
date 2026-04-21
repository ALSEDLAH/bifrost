// SCIM Users write-side tests (spec 021). Reuses helpers from
// scim_users_test.go (same package): newSCIMTestStore, seedSCIMEnabled,
// seedUser, extractSCIMUserID.

package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// seedDefaultOrg pins the singleton ent_system_defaults row to the
// test's expected org/workspace. Migrations may pre-seed the row, so
// we update in place rather than insert.
func seedDefaultOrg(t *testing.T, store configstore.ConfigStore) {
	t.Helper()
	require.NoError(t, store.DB().
		Model(&tables_enterprise.TableSystemDefaults{}).
		Where("id = ?", tables_enterprise.SystemDefaultsRowID).
		Updates(map[string]any{
			"default_organization_id": "org-synth",
			"default_workspace_id":    "ws-synth",
			"seeded_at":               time.Now().UTC(),
		}).Error)
}

func TestSCIMWrite_CreateHappyPath(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrg(t, store)
	token := seedSCIMEnabled(t, store, true)

	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],
	          "userName":"new@example.com",
	          "name":{"givenName":"New","familyName":"User"},
	          "externalId":"subj-123",
	          "active":true}`
	h := NewSCIMUsersHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/scim/v2/Users")
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.SetBody([]byte(body))
	h.create(ctx)

	if ctx.Response.StatusCode() != 201 {
		t.Fatalf("status %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var out map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &out)
	if out["userName"] != "new@example.com" {
		t.Errorf("userName wrong: %+v", out)
	}
	if out["active"] != true {
		t.Errorf("active wrong: %+v", out)
	}
	var u tables_enterprise.TableUser
	require.NoError(t, store.DB().Where("email = ?", "new@example.com").First(&u).Error)
	if u.OrganizationID != "org-synth" {
		t.Errorf("org not set: %+v", u)
	}
}

func TestSCIMWrite_CreateDuplicate_Returns409(t *testing.T) {
	store := newSCIMTestStore(t)
	seedDefaultOrg(t, store)
	token := seedSCIMEnabled(t, store, true)
	// Pre-existing user must share the default org — the unique
	// index is (organization_id, email).
	existing := &tables_enterprise.TableUser{
		ID: "dup-seed", OrganizationID: "org-synth",
		Email: "dup@example.com", Status: "active",
		IdpSubject: "sub-dup-seed",
	}
	require.NoError(t, store.DB().Create(existing).Error)

	body := `{"schemas":["x"],"userName":"dup@example.com"}`
	h := NewSCIMUsersHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/scim/v2/Users")
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.SetBody([]byte(body))
	h.create(ctx)

	if ctx.Response.StatusCode() != 409 {
		t.Fatalf("expected 409; got %d: %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestSCIMWrite_PatchActiveFalse_SuspendsUser(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	id := seedUser(t, store, "alice@example.com", "Alice J", "active")

	body := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
	          "Operations":[{"op":"Replace","path":"active","value":false}]}`
	h := NewSCIMUsersHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("PATCH")
	ctx.Request.SetRequestURI("/scim/v2/Users/" + id)
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.SetUserValue("id", id)
	ctx.Request.SetBody([]byte(body))
	h.patch(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("status %d body %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var out map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &out)
	if out["active"] != false {
		t.Errorf("active should be false; got %+v", out)
	}
	var u tables_enterprise.TableUser
	require.NoError(t, store.DB().Where("id = ?", id).First(&u).Error)
	if u.Status != "suspended" {
		t.Errorf("status should be suspended; got %q", u.Status)
	}
}

func TestSCIMWrite_PatchRenamesUserName(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	id := seedUser(t, store, "old@example.com", "X", "active")

	body := `{"Operations":[{"op":"Replace","path":"userName","value":"new@example.com"}]}`
	h := NewSCIMUsersHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("PATCH")
	ctx.Request.SetRequestURI("/scim/v2/Users/" + id)
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.SetUserValue("id", id)
	ctx.Request.SetBody([]byte(body))
	h.patch(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("status %d", ctx.Response.StatusCode())
	}
	var u tables_enterprise.TableUser
	require.NoError(t, store.DB().Where("id = ?", id).First(&u).Error)
	if u.Email != "new@example.com" {
		t.Errorf("rename didn't apply; got %q", u.Email)
	}
}

func TestSCIMWrite_PatchUnsupportedPath_Returns400(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	id := seedUser(t, store, "x@example.com", "X", "active")

	body := `{"Operations":[{"op":"Replace","path":"emails","value":"x"}]}`
	h := NewSCIMUsersHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("PATCH")
	ctx.Request.SetRequestURI("/scim/v2/Users/" + id)
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.SetUserValue("id", id)
	ctx.Request.SetBody([]byte(body))
	h.patch(ctx)

	if ctx.Response.StatusCode() != 400 {
		t.Errorf("expected 400; got %d: %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestSCIMWrite_Delete_SoftSuspends(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	id := seedUser(t, store, "gone@example.com", "Gone Bye", "active")

	h := NewSCIMUsersHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("DELETE")
	ctx.Request.SetRequestURI("/scim/v2/Users/" + id)
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.SetUserValue("id", id)
	h.del(ctx)

	if ctx.Response.StatusCode() != 204 {
		t.Fatalf("expected 204; got %d", ctx.Response.StatusCode())
	}
	var u tables_enterprise.TableUser
	require.NoError(t, store.DB().Where("id = ?", id).First(&u).Error)
	if u.Status != "suspended" {
		t.Errorf("status should be suspended; got %q", u.Status)
	}
}

func TestSCIMWrite_DeleteMissingId_IsIdempotent204(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	h := NewSCIMUsersHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("DELETE")
	ctx.Request.SetRequestURI("/scim/v2/Users/no-such-id")
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.SetUserValue("id", "no-such-id")
	h.del(ctx)
	if ctx.Response.StatusCode() != 204 {
		t.Errorf("missing id should be idempotent 204; got %d", ctx.Response.StatusCode())
	}
}
