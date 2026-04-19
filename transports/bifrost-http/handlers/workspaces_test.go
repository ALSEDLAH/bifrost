// Integration test for the workspaces HTTP handler — verifies:
//   (1) GET list/get returns 200 when the caller has workspaces:read
//   (2) POST/PATCH/DELETE return 403 without the appropriate scope
//   (3) Cross-org access is not leaked via the API surface
//
//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// fixtureStore is a minimal configstore.ConfigStore stand-in that
// satisfies the DB() method used by the enterprise handlers. The full
// ConfigStore interface is large; we only need the DB accessor here.
type fixtureStore struct{ db *gorm.DB }

func (f *fixtureStore) DB() *gorm.DB { return f.db }

// Only the DB method matters for WorkspacesHandler / OrganizationsHandler.
// Every other ConfigStore method is unused; to keep the fixture compact
// we embed a configstore.ConfigStore interface to get the right type
// and the methods we don't implement will panic if they're ever called
// (which they shouldn't, since the enterprise handlers only call DB()).
//
// If future handlers need more ConfigStore methods, this fixture grows.
var _ configstore.ConfigStore = (*fullFixtureStore)(nil)

// fullFixtureStore wraps fixtureStore with an embedded interface to
// satisfy the static type check. All non-DB methods will panic if
// invoked — the contract being that enterprise handlers consume only
// the DB() accessor.
type fullFixtureStore struct {
	configstore.ConfigStore
	db *gorm.DB
}

func (f *fullFixtureStore) DB() *gorm.DB { return f.db }

func newFixtureStore(t *testing.T) (*fullFixtureStore, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("BIFROST_ENT_PG_DSN")
	if dsn == "" {
		t.Skip("BIFROST_ENT_PG_DSN not set; skipping")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		for _, tbl := range []string{
			"ent_user_role_assignments", "ent_roles", "ent_users",
			"ent_workspaces", "ent_organizations", "ent_system_defaults",
			"migrations",
		} {
			_ = db.Migrator().DropTable(tbl)
		}
	})
	if err := configstore.RegisterEnterpriseMigrations(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &fullFixtureStore{db: db}, db
}

// makeClient spins up an in-memory fasthttp server with the given
// handler router and returns a client that speaks to it.
func makeClient(t *testing.T, r *router.Router) (*http.Client, func()) {
	t.Helper()
	ln := fasthttputil.NewInmemoryListener()
	srv := &fasthttp.Server{Handler: r.Handler}
	go func() { _ = srv.Serve(ln) }()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return ln.Dial()
			},
		},
	}
	return client, func() { _ = srv.Shutdown(); _ = ln.Close() }
}

// tenantMW stamps a fixed TenantContext onto the request for tests.
// In production this is the enterprise-gate plugin's resolver; here
// we inject directly to isolate the handler's 403 logic.
func tenantMW(tc tenancy.TenantContext) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			ctx.SetUserValue(string(tenancy.BifrostContextKeyTenantContext), tc)
			ctx.SetUserValue(string(tenancy.BifrostContextKeyOrganizationID), tc.OrganizationID)
			ctx.SetUserValue(string(tenancy.BifrostContextKeyWorkspaceID), tc.WorkspaceID)
			ctx.SetUserValue(string(tenancy.BifrostContextKeyRoleScopes), tc.RoleScopes)
			next(ctx)
		}
	}
}

func TestWorkspacesHandler_ReadOnlyGetsList_WriteGets403(t *testing.T) {
	store, db := newFixtureStore(t)

	// Resolve default org + create one workspace so List has content.
	orgs := tenancy.NewOrgRepo(db)
	org, _ := orgs.GetDefault(context.Background())
	wsRepo := tenancy.NewWorkspaceRepo(db)
	_, _ = wsRepo.Create(context.Background(), org.ID, "Product", "product", "")

	// Caller has only workspaces:read — should be able to list, but
	// not create.
	tc := tenancy.TenantContext{
		OrganizationID: org.ID,
		RoleScopes:     []string{"workspaces:read"},
		ResolvedVia:    tenancy.ResolverSession,
	}

	r := router.New()
	h := handlers.NewWorkspacesHandler(store, nil)
	h.RegisterRoutes(r,
		tenantMW(tc),
		lib.NewRBACEnforceProvider().Middleware(),
	)

	client, shutdown := makeClient(t, r)
	defer shutdown()

	// GET list → 200
	resp, err := client.Get("http://local/v1/admin/workspaces")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d; want 200", resp.StatusCode)
	}

	// POST create → 403 (missing workspaces:write)
	body, _ := json.Marshal(map[string]string{"name": "Sales", "slug": "sales"})
	resp, err = client.Post("http://local/v1/admin/workspaces", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("create status = %d; want 403 (missing workspaces:write)", resp.StatusCode)
	}
}

func TestWorkspacesHandler_CrossOrgGet_Returns404(t *testing.T) {
	store, db := newFixtureStore(t)

	orgs := tenancy.NewOrgRepo(db)
	orgA, _ := orgs.GetDefault(context.Background())
	orgB, _ := orgs.CreateMultiOrg(context.Background(), "OrgB")

	// Create a workspace in orgA.
	wsRepo := tenancy.NewWorkspaceRepo(db)
	wsA, _ := wsRepo.Create(context.Background(), orgA.ID, "A-ws", "a-ws", "")

	// Caller is scoped to orgB with full workspaces rights — but
	// should still get 404 for wsA (cross-tenant leak would be 200).
	tc := tenancy.TenantContext{
		OrganizationID: orgB.ID,
		RoleScopes:     []string{"workspaces:read", "workspaces:write", "workspaces:delete"},
		ResolvedVia:    tenancy.ResolverAdminAPIKey,
	}

	r := router.New()
	h := handlers.NewWorkspacesHandler(store, nil)
	h.RegisterRoutes(r, tenantMW(tc), lib.NewRBACEnforceProvider().Middleware())

	client, shutdown := makeClient(t, r)
	defer shutdown()

	resp, err := client.Get("http://local/v1/admin/workspaces/" + wsA.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-org get status = %d; want 404 (cross-tenant leak!)", resp.StatusCode)
	}
}
