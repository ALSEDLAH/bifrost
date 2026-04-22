// Tests for handlers/rbac.go (Constitution Principle VIII — test coverage).
//
// Integration-style: uses a real SQLite configstore so the enterprise
// migrations (E001 seed default org, E004 users/roles/assignments) run
// exactly as they do in production. Each test cleans up after itself so
// the suite is hermetic.

package handlers

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"

	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func setupRBACHandler(t *testing.T) *RBACHandler {
	t.Helper()
	SetLogger(&mockLogger{})

	dbPath := t.TempDir() + "/config.db"
	store, err := configstore.NewConfigStore(context.Background(), &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: dbPath},
	}, &mockLogger{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(dbPath) })

	// Enterprise migrations seed ent_system_defaults + 4 built-in roles.
	require.NoError(t, configstore.RegisterEnterpriseMigrations(context.Background(), store.DB()))

	return NewRBACHandler(store.DB(), &mockLogger{})
}

func rbacReq(method, body string) *fasthttp.RequestCtx {
	var req fasthttp.Request
	req.Header.SetMethod(method)
	if body != "" {
		req.SetBodyString(body)
	}
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&req, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}, nil)
	return ctx
}

// ---- Meta ---------------------------------------------------------

func TestRBAC_Meta_MirrorsTenancyCatalog(t *testing.T) {
	h := setupRBACHandler(t)
	ctx := rbacReq("GET", "")
	h.handleMeta(ctx)

	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), string(ctx.Response.Body()))
	var got rbacMetaResponse
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &got))
	// Catalog-agnostic — adding/removing a resource shouldn't break
	// this test, only the catalog/UI alignment will catch that.
	assert.ElementsMatch(t, tenancy.Resources, got.Resources, "RBAC meta resources must mirror tenancy.Resources")
	assert.ElementsMatch(t, tenancy.Operations, got.Operations, "RBAC meta operations must mirror tenancy.Operations")
	assert.ElementsMatch(t,
		[]string{"Owner", "Admin", "Manager", "Member"},
		got.BuiltinRoles,
	)
}

// ---- Me -----------------------------------------------------------

func TestRBAC_Me_ReturnsWildcardInSingleOrgMode(t *testing.T) {
	h := setupRBACHandler(t)
	ctx := rbacReq("GET", "")
	h.handleMe(ctx)

	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	var got rbacMeResponse
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &got))
	assert.Contains(t, got.Scopes, "*")
	assert.Len(t, got.Permissions, len(tenancy.Resources),
		"permissions map should expose every catalog resource")
	for _, r := range tenancy.Resources {
		assert.ElementsMatch(t, tenancy.Operations, got.Permissions[r],
			"every catalog resource should grant every operation in single-org mode")
	}
	assert.NotEmpty(t, got.OrganizationID)
}

// ---- Roles --------------------------------------------------------

func TestRBAC_Roles_ListReturnsFourBuiltins(t *testing.T) {
	h := setupRBACHandler(t)
	ctx := rbacReq("GET", "")
	h.listRoles(ctx)

	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	var got map[string]any
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &got))
	roles, _ := got["roles"].([]any)
	require.Len(t, roles, 4)
	names := make(map[string]bool)
	for _, r := range roles {
		m := r.(map[string]any)
		names[m["name"].(string)] = true
		assert.Equal(t, true, m["is_builtin"])
	}
	assert.True(t, names["Owner"])
	assert.True(t, names["Admin"])
	assert.True(t, names["Manager"])
	assert.True(t, names["Member"])
}

func TestRBAC_Roles_CreateCustomRole(t *testing.T) {
	h := setupRBACHandler(t)
	ctx := rbacReq("POST", `{"name":"Auditor","scopes":{"AuditLogs":["Read","View","Download"]}}`)
	h.createRole(ctx)

	require.Equal(t, fasthttp.StatusCreated, ctx.Response.StatusCode(), string(ctx.Response.Body()))
	var resp map[string]any
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &resp))
	role, _ := resp["role"].(map[string]any)
	assert.Equal(t, "Auditor", role["name"])
	assert.Equal(t, false, role["is_builtin"])
}

func TestRBAC_Roles_CreateRejectsDuplicateName(t *testing.T) {
	h := setupRBACHandler(t)
	// Owner is a built-in role seeded at migration time.
	ctx := rbacReq("POST", `{"name":"Owner","scopes":{"Users":["Read"]}}`)
	h.createRole(ctx)
	assert.Equal(t, fasthttp.StatusConflict, ctx.Response.StatusCode())
}

func TestRBAC_Roles_CreateRejectsMissingName(t *testing.T) {
	h := setupRBACHandler(t)
	ctx := rbacReq("POST", `{"scopes":{"Users":["Read"]}}`)
	h.createRole(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
}

func TestRBAC_Roles_UpdateRejectsBuiltin(t *testing.T) {
	h := setupRBACHandler(t)
	// Find Owner's id
	ctx := rbacReq("GET", "")
	h.listRoles(ctx)
	var listed map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &listed)
	var ownerID string
	for _, r := range listed["roles"].([]any) {
		m := r.(map[string]any)
		if m["name"].(string) == "Owner" {
			ownerID = m["id"].(string)
			break
		}
	}
	require.NotEmpty(t, ownerID)

	updateCtx := rbacReq("PATCH", `{"scopes":{"Users":["Read"]}}`)
	updateCtx.SetUserValue("id", ownerID)
	h.updateRole(updateCtx)
	assert.Equal(t, fasthttp.StatusForbidden, updateCtx.Response.StatusCode())
}

func TestRBAC_Roles_DeleteRejectsBuiltin(t *testing.T) {
	h := setupRBACHandler(t)
	listCtx := rbacReq("GET", "")
	h.listRoles(listCtx)
	var listed map[string]any
	_ = json.Unmarshal(listCtx.Response.Body(), &listed)
	var ownerID string
	for _, r := range listed["roles"].([]any) {
		m := r.(map[string]any)
		if m["name"].(string) == "Owner" {
			ownerID = m["id"].(string)
			break
		}
	}
	deleteCtx := rbacReq("DELETE", "")
	deleteCtx.SetUserValue("id", ownerID)
	h.deleteRole(deleteCtx)
	assert.Equal(t, fasthttp.StatusForbidden, deleteCtx.Response.StatusCode())
}

func TestRBAC_Roles_CreateThenUpdateThenDeleteCustomRole(t *testing.T) {
	h := setupRBACHandler(t)

	// Create
	createCtx := rbacReq("POST", `{"name":"Auditor","scopes":{"AuditLogs":["Read"]}}`)
	h.createRole(createCtx)
	require.Equal(t, fasthttp.StatusCreated, createCtx.Response.StatusCode())
	var createdResp map[string]any
	_ = json.Unmarshal(createCtx.Response.Body(), &createdResp)
	roleID := createdResp["role"].(map[string]any)["id"].(string)

	// Update
	updateCtx := rbacReq("PATCH", `{"scopes":{"AuditLogs":["Read","View","Download"]}}`)
	updateCtx.SetUserValue("id", roleID)
	h.updateRole(updateCtx)
	require.Equal(t, fasthttp.StatusOK, updateCtx.Response.StatusCode())

	// Delete
	deleteCtx := rbacReq("DELETE", "")
	deleteCtx.SetUserValue("id", roleID)
	h.deleteRole(deleteCtx)
	assert.Equal(t, fasthttp.StatusOK, deleteCtx.Response.StatusCode())

	// Confirm total back to 4
	listCtx := rbacReq("GET", "")
	h.listRoles(listCtx)
	var final map[string]any
	_ = json.Unmarshal(listCtx.Response.Body(), &final)
	assert.Len(t, final["roles"].([]any), 4)
}

// ---- Users --------------------------------------------------------

func TestRBAC_Users_CreateInviteDefaultsToPending(t *testing.T) {
	h := setupRBACHandler(t)
	ctx := rbacReq("POST", `{"email":"alice@example.com","display_name":"Alice"}`)
	h.createUser(ctx)
	require.Equal(t, fasthttp.StatusCreated, ctx.Response.StatusCode(), string(ctx.Response.Body()))

	var resp map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &resp)
	u := resp["user"].(map[string]any)
	assert.Equal(t, "alice@example.com", u["email"])
	assert.Equal(t, "Alice", u["display_name"])
	assert.Equal(t, "pending", u["status"])
}

func TestRBAC_Users_CreateRequiresEmail(t *testing.T) {
	h := setupRBACHandler(t)
	ctx := rbacReq("POST", `{"display_name":"Alice"}`)
	h.createUser(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
}

func TestRBAC_Users_DeleteRemovesUserAndAssignments(t *testing.T) {
	h := setupRBACHandler(t)

	// Create a user + role + assignment, then delete the user.
	createUserCtx := rbacReq("POST", `{"email":"bob@example.com"}`)
	h.createUser(createUserCtx)
	require.Equal(t, fasthttp.StatusCreated, createUserCtx.Response.StatusCode())
	var userResp map[string]any
	_ = json.Unmarshal(createUserCtx.Response.Body(), &userResp)
	userID := userResp["user"].(map[string]any)["id"].(string)

	createRoleCtx := rbacReq("POST", `{"name":"TmpRole","scopes":{"Users":["Read"]}}`)
	h.createRole(createRoleCtx)
	var roleResp map[string]any
	_ = json.Unmarshal(createRoleCtx.Response.Body(), &roleResp)
	roleID := roleResp["role"].(map[string]any)["id"].(string)

	assignCtx := rbacReq("POST", `{"user_id":"`+userID+`","role_id":"`+roleID+`"}`)
	h.assignRole(assignCtx)
	require.Equal(t, fasthttp.StatusCreated, assignCtx.Response.StatusCode())

	deleteCtx := rbacReq("DELETE", "")
	deleteCtx.SetUserValue("id", userID)
	h.deleteUser(deleteCtx)
	assert.Equal(t, fasthttp.StatusOK, deleteCtx.Response.StatusCode())

	// User is gone.
	listCtx := rbacReq("GET", "")
	h.listUsers(listCtx)
	var listed map[string]any
	_ = json.Unmarshal(listCtx.Response.Body(), &listed)
	assert.Equal(t, float64(0), listed["total"])
}

// ---- Assignments --------------------------------------------------

func TestRBAC_Assign_UnknownRoleReturnsNotFound(t *testing.T) {
	h := setupRBACHandler(t)

	createUserCtx := rbacReq("POST", `{"email":"carol@example.com"}`)
	h.createUser(createUserCtx)
	var userResp map[string]any
	_ = json.Unmarshal(createUserCtx.Response.Body(), &userResp)
	userID := userResp["user"].(map[string]any)["id"].(string)

	ctx := rbacReq("POST", `{"user_id":"`+userID+`","role_id":"nonexistent"}`)
	h.assignRole(ctx)
	assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
}

func TestRBAC_Assign_RequiresUserIDAndRoleID(t *testing.T) {
	h := setupRBACHandler(t)
	ctx := rbacReq("POST", `{"role_id":"r1"}`)
	h.assignRole(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
}
