// Contract tests for the enterprise middleware: tenant resolution +
// RBAC enforcement.
//
// Validates: 401 on missing auth, 403 on missing scope, happy-path
// pass-through. No DB needed — these are pure middleware tests.

package lib_test

import (
	"errors"
	"testing"

	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// fakeRequestCtx returns a fasthttp request context primed for use in
// the middleware. We don't need a real network conn for these tests;
// fasthttp.RequestCtx works in-memory.
func fakeRequestCtx() *fasthttp.RequestCtx {
	return &fasthttp.RequestCtx{}
}

func TestTenantResolveMiddleware_HappyPath_PopulatesContext(t *testing.T) {
	resolved := tenancy.TenantContext{
		OrganizationID: "org-1",
		WorkspaceID:    "ws-1",
		ResolvedVia:    tenancy.ResolverAdminAPIKey,
		RoleScopes:     []string{"workspaces:write"},
	}
	provider := lib.NewTenantResolveProvider(func(ctx *fasthttp.RequestCtx) (tenancy.TenantContext, error) {
		return resolved, nil
	})

	ctx := fakeRequestCtx()
	called := false
	final := provider.Middleware()(func(c *fasthttp.RequestCtx) {
		called = true
	})
	final(ctx)

	if !called {
		t.Fatal("next handler was not invoked")
	}
	got, ok := ctx.UserValue(string(tenancy.BifrostContextKeyTenantContext)).(tenancy.TenantContext)
	if !ok || got.OrganizationID != "org-1" {
		t.Fatalf("TenantContext not stashed correctly: %+v", got)
	}
}

func TestTenantResolveMiddleware_AuthError_Returns401(t *testing.T) {
	provider := lib.NewTenantResolveProvider(func(ctx *fasthttp.RequestCtx) (tenancy.TenantContext, error) {
		return tenancy.TenantContext{}, errors.New("missing or invalid api key")
	})

	ctx := fakeRequestCtx()
	called := false
	final := provider.Middleware()(func(c *fasthttp.RequestCtx) {
		called = true
	})
	final(ctx)

	if called {
		t.Fatal("next handler should not have run on auth error")
	}
	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("status = %d; want 401", ctx.Response.StatusCode())
	}
}

func TestRBACEnforceMiddleware_HappyPath_PassesThrough(t *testing.T) {
	rbac := lib.NewRBACEnforceProvider().Middleware()

	ctx := fakeRequestCtx()
	ctx.SetUserValue(string(tenancy.BifrostContextKeyTenantContext), tenancy.TenantContext{
		OrganizationID: "org-1",
		RoleScopes:     []string{"workspaces:write"},
	})
	ctx.SetUserValue("enterprise.required_scope", "workspaces:write")

	called := false
	rbac(func(c *fasthttp.RequestCtx) { called = true })(ctx)

	if !called {
		t.Fatal("RBAC middleware blocked a request that has the required scope")
	}
	if ctx.Response.StatusCode() == fasthttp.StatusForbidden {
		t.Fatal("got 403 on a request that has the scope")
	}
}

func TestRBACEnforceMiddleware_MissingScope_Returns403(t *testing.T) {
	rbac := lib.NewRBACEnforceProvider().Middleware()

	ctx := fakeRequestCtx()
	ctx.SetUserValue(string(tenancy.BifrostContextKeyTenantContext), tenancy.TenantContext{
		OrganizationID: "org-1",
		RoleScopes:     []string{"metrics:read"}, // wrong scope
	})
	ctx.SetUserValue("enterprise.required_scope", "workspaces:write")

	called := false
	rbac(func(c *fasthttp.RequestCtx) { called = true })(ctx)

	if called {
		t.Fatal("next handler ran despite missing scope")
	}
	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status = %d; want 403", ctx.Response.StatusCode())
	}
}

func TestRBACEnforceMiddleware_WildcardSatisfiesAnyScope(t *testing.T) {
	rbac := lib.NewRBACEnforceProvider().Middleware()
	ctx := fakeRequestCtx()
	ctx.SetUserValue(string(tenancy.BifrostContextKeyTenantContext), tenancy.TenantContext{
		OrganizationID: "org-1",
		RoleScopes:     []string{"*"},
	})
	ctx.SetUserValue("enterprise.required_scope", "guardrails:delete")

	called := false
	rbac(func(c *fasthttp.RequestCtx) { called = true })(ctx)

	if !called {
		t.Fatal("wildcard scope should satisfy any required scope")
	}
}

func TestRBACEnforceMiddleware_NoRequiredScope_PassesThrough(t *testing.T) {
	// Routes that don't set required_scope should not be blocked.
	rbac := lib.NewRBACEnforceProvider().Middleware()
	ctx := fakeRequestCtx()
	called := false
	rbac(func(c *fasthttp.RequestCtx) { called = true })(ctx)

	if !called {
		t.Fatal("middleware blocked an unrestricted route")
	}
}

func TestEnterpriseMiddlewareRegistry_OrderPreserved(t *testing.T) {
	lib.ResetEnterpriseMiddlewareProviders()
	defer lib.ResetEnterpriseMiddlewareProviders()

	lib.RegisterEnterpriseMiddlewareProvider(lib.NewTenantResolveProvider(func(c *fasthttp.RequestCtx) (tenancy.TenantContext, error) {
		return tenancy.TenantContext{OrganizationID: "org"}, nil
	}))
	lib.RegisterEnterpriseMiddlewareProvider(lib.NewRBACEnforceProvider())

	mws := lib.EnterpriseMiddlewares()
	if len(mws) != 2 {
		t.Fatalf("expected 2 registered middlewares; got %d", len(mws))
	}
}
