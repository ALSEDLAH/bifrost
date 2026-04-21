// Unit tests for the SCIM 2.0 Users read-only handler (spec 020).
// Bypasses the fasthttp router for the reasons documented in
// compliance_reports_test.go.

package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func newSCIMTestStore(t *testing.T) configstore.ConfigStore {
	t.Helper()
	dir := t.TempDir()
	store, err := configstore.NewConfigStore(context.Background(), &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: filepath.Join(dir, "cs.db")},
	}, &mockLogger{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	require.NoError(t, configstore.RegisterEnterpriseMigrations(context.Background(), store.DB()))
	return store
}

// seedSCIMEnabled stores a SCIM config with known token + hash.
// Returns the plaintext bearer.
func seedSCIMEnabled(t *testing.T, store configstore.ConfigStore, enabled bool) string {
	t.Helper()
	plain := "scim_test_token_abc123"
	sum := sha256.Sum256([]byte(plain))
	cfg := &tables_enterprise.TableSCIMConfig{
		Enabled:         enabled,
		BearerTokenHash: hex.EncodeToString(sum[:]),
		TokenPrefix:     plain[:8],
	}
	now := time.Now().UTC()
	cfg.TokenCreatedAt = &now
	require.NoError(t, store.UpsertSCIMConfig(context.Background(), cfg))
	return plain
}

func seedUser(t *testing.T, store configstore.ConfigStore, email, display, status string) string {
	t.Helper()
	id := uuid.NewString()
	u := &tables_enterprise.TableUser{
		ID:             id,
		OrganizationID: "org-1",
		Email:          email,
		DisplayName:    display,
		Status:         status,
		// Unique idp_subject per user — the table has a unique index
		// on it and empty strings collide.
		IdpSubject: "sub-" + id,
	}
	require.NoError(t, store.DB().Create(u).Error)
	return u.ID
}

func invokeSCIMUsers(t *testing.T, store configstore.ConfigStore, method, path, bearer string) (int, map[string]any) {
	t.Helper()
	h := NewSCIMUsersHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(path)
	if bearer != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+bearer)
	}
	// Route to the right method based on path shape.
	switch {
	case path == "/scim/v2/Users" || len(path) >= 14 && path[:14] == "/scim/v2/Users":
		if id := extractSCIMUserID(path); id != "" {
			ctx.SetUserValue("id", id)
			h.get(ctx)
		} else {
			h.list(ctx)
		}
	default:
		t.Fatalf("unexpected path %q", path)
	}
	var body map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &body)
	return ctx.Response.StatusCode(), body
}

// extractSCIMUserID pulls the id from `/scim/v2/Users/<id>`, or "" when
// the path is the list endpoint (optionally with query string).
func extractSCIMUserID(path string) string {
	prefix := "/scim/v2/Users/"
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		return ""
	}
	rest := path[len(prefix):]
	// strip query string
	for i := 0; i < len(rest); i++ {
		if rest[i] == '?' {
			return rest[:i]
		}
	}
	return rest
}

func TestSCIMUsers_MissingBearer_Returns401(t *testing.T) {
	store := newSCIMTestStore(t)
	status, body := invokeSCIMUsers(t, store, "GET", "/scim/v2/Users", "")
	if status != 401 {
		t.Fatalf("expected 401; got %d: %+v", status, body)
	}
	schemas, _ := body["schemas"].([]any)
	if len(schemas) == 0 || schemas[0] != scimErrorSchema {
		t.Errorf("expected SCIM error schema; got %+v", body)
	}
}

func TestSCIMUsers_WrongToken_Returns401(t *testing.T) {
	store := newSCIMTestStore(t)
	seedSCIMEnabled(t, store, true)
	status, _ := invokeSCIMUsers(t, store, "GET", "/scim/v2/Users", "wrong-token")
	if status != 401 {
		t.Errorf("wrong token must 401; got %d", status)
	}
}

func TestSCIMUsers_DisabledConfig_Returns401(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, false)
	status, _ := invokeSCIMUsers(t, store, "GET", "/scim/v2/Users", token)
	if status != 401 {
		t.Errorf("disabled scim must 401 even with valid token; got %d", status)
	}
}

func TestSCIMUsers_ListHappyPath(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	seedUser(t, store, "alice@example.com", "Alice Jones", "active")
	seedUser(t, store, "bob@example.com", "Bob Smith", "pending")
	seedUser(t, store, "carol@example.com", "", "active")

	status, body := invokeSCIMUsers(t, store, "GET", "/scim/v2/Users", token)
	if status != 200 {
		t.Fatalf("expected 200; got %d: %+v", status, body)
	}
	if body["totalResults"] != float64(3) {
		t.Errorf("totalResults: %v", body["totalResults"])
	}
	resources, _ := body["Resources"].([]any)
	if len(resources) != 3 {
		t.Fatalf("Resources len: %d", len(resources))
	}
	// Alphabetical order by email — alice, bob, carol.
	first, _ := resources[0].(map[string]any)
	if first["userName"] != "alice@example.com" {
		t.Errorf("first user should be alice; got %+v", first)
	}
	// Name splitting: "Alice Jones" → given=Alice, family=Jones.
	name, _ := first["name"].(map[string]any)
	if name["givenName"] != "Alice" || name["familyName"] != "Jones" {
		t.Errorf("name split wrong: %+v", name)
	}
	// Emails array + primary flag.
	emails, _ := first["emails"].([]any)
	if len(emails) != 1 {
		t.Fatalf("expected 1 email; got %d", len(emails))
	}
	email0, _ := emails[0].(map[string]any)
	if email0["value"] != "alice@example.com" || email0["primary"] != true {
		t.Errorf("email[0] shape wrong: %+v", email0)
	}
	// Active flag: alice=active, bob=pending → false.
	if first["active"] != true {
		t.Errorf("alice should be active; got %v", first["active"])
	}
	second, _ := resources[1].(map[string]any)
	if second["active"] != false {
		t.Errorf("bob is pending, active should be false; got %v", second["active"])
	}
}

func TestSCIMUsers_FilterUserNameEq(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	seedUser(t, store, "alice@example.com", "Alice J", "active")
	seedUser(t, store, "bob@example.com", "Bob S", "active")

	status, body := invokeSCIMUsers(t, store, "GET",
		`/scim/v2/Users?filter=userName%20eq%20%22bob%40example.com%22`,
		token)
	if status != 200 {
		t.Fatalf("status %d, body %+v", status, body)
	}
	if body["totalResults"] != float64(1) {
		t.Errorf("filter should match 1; got totalResults=%v", body["totalResults"])
	}
	resources, _ := body["Resources"].([]any)
	first, _ := resources[0].(map[string]any)
	if first["userName"] != "bob@example.com" {
		t.Errorf("filter returned wrong user: %+v", first)
	}
}

func TestSCIMUsers_UnsupportedFilter_Returns400(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	status, body := invokeSCIMUsers(t, store, "GET",
		`/scim/v2/Users?filter=active%20eq%20true`, token)
	if status != 400 {
		t.Errorf("expected 400 for unsupported filter; got %d: %+v", status, body)
	}
}

func TestSCIMUsers_Pagination(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	for i := 0; i < 5; i++ {
		seedUser(t, store, fmt.Sprintf("user%02d@example.com", i), "U", "active")
	}
	// startIndex=3, count=2 → expect users u02, u03
	status, body := invokeSCIMUsers(t, store, "GET",
		`/scim/v2/Users?startIndex=3&count=2`, token)
	if status != 200 {
		t.Fatalf("status %d", status)
	}
	if body["totalResults"] != float64(5) {
		t.Errorf("total should still be 5; got %v", body["totalResults"])
	}
	if body["itemsPerPage"] != float64(2) {
		t.Errorf("itemsPerPage: %v", body["itemsPerPage"])
	}
	resources, _ := body["Resources"].([]any)
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources on page; got %d", len(resources))
	}
}

func TestSCIMUsers_GetById(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	id := seedUser(t, store, "alice@example.com", "Alice J", "active")

	status, body := invokeSCIMUsers(t, store, "GET", "/scim/v2/Users/"+id, token)
	if status != 200 {
		t.Fatalf("status %d body %+v", status, body)
	}
	if body["id"] != id {
		t.Errorf("expected id=%s; got %v", id, body["id"])
	}
	if body["userName"] != "alice@example.com" {
		t.Errorf("userName wrong: %v", body["userName"])
	}
}

func TestSCIMUsers_GetById_NotFound(t *testing.T) {
	store := newSCIMTestStore(t)
	token := seedSCIMEnabled(t, store, true)
	status, body := invokeSCIMUsers(t, store, "GET", "/scim/v2/Users/no-such-id", token)
	if status != 404 {
		t.Errorf("expected 404; got %d: %+v", status, body)
	}
	schemas, _ := body["schemas"].([]any)
	if len(schemas) == 0 || schemas[0] != scimErrorSchema {
		t.Errorf("404 body should carry SCIM error schema; got %+v", body)
	}
}
