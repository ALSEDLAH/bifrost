// Enterprise middleware registration hook.
//
// Constitution Principle XI rule 4 — single hook anchor on upstream files.
// Upstream's middleware.go provides ChainMiddlewares. The enterprise fork
// adds enterprise-specific middlewares (tenant resolution, RBAC enforce,
// license entitlement check) here without modifying middleware.go's body.
//
// The chain assembly point in upstream's HTTP server invokes
// RegisterEnterpriseMiddlewares once per route group; the registration
// function appends the enterprise middlewares to the supplied chain.
//
// This is a sibling file: nothing in upstream's lib/*.go is modified.

package lib

import (
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/valyala/fasthttp"
)

// EnterpriseMiddlewareProvider lets each enterprise plugin contribute its
// middleware to the chain at registration time, without coupling the
// transport layer to specific plugins.
type EnterpriseMiddlewareProvider interface {
	// Name returns the provider name for logging (e.g. "enterprise-gate",
	// "license-enforce", "tenant-resolve").
	Name() string

	// Middleware returns the middleware function to install. The order of
	// providers in the registered list determines installation order:
	// providers added earlier wrap later ones (i.e., they execute first).
	Middleware() schemas.BifrostHTTPMiddleware
}

// providers is the package-level registry of enterprise middleware
// providers. Plugins call RegisterEnterpriseMiddlewareProvider during
// their own Init() to add themselves; the transport layer then calls
// EnterpriseMiddlewares() at chain-assembly time.
//
// Read after the plugin manager has finished initializing all plugins.
var providers []EnterpriseMiddlewareProvider

// RegisterEnterpriseMiddlewareProvider adds a provider to the registry.
// Called from plugin Init() functions. Order of registration determines
// execution order (first registered runs first).
func RegisterEnterpriseMiddlewareProvider(p EnterpriseMiddlewareProvider) {
	providers = append(providers, p)
}

// EnterpriseMiddlewares returns the registered enterprise middlewares
// in registration order. The transport's route assembler prepends these
// to each route's middleware chain via:
//
//	allMiddlewares := append(lib.EnterpriseMiddlewares(), upstreamMiddlewares...)
//	handler := lib.ChainMiddlewares(handler, allMiddlewares...)
//
// That single-line anchor is the only enterprise touch in upstream's
// route assembly code (Principle XI rule 4).
func EnterpriseMiddlewares() []schemas.BifrostHTTPMiddleware {
	mws := make([]schemas.BifrostHTTPMiddleware, 0, len(providers))
	for _, p := range providers {
		mws = append(mws, p.Middleware())
	}
	return mws
}

// ResetEnterpriseMiddlewareProviders is for tests only — clears the
// registry so each test starts from a clean state.
func ResetEnterpriseMiddlewareProviders() {
	providers = nil
}

// ─────────────────────────────────────────────────────────────────────
// Concrete enterprise middlewares (T028) — tenant resolve + RBAC enforce.
// These are reference implementations; the gate plugin (T024) supplies
// the actual resolver list at runtime via NewTenantResolveProvider.
// ─────────────────────────────────────────────────────────────────────

// TenantResolveFunc is the contract the enterprise-gate plugin
// implements. It receives a fasthttp request context and returns
// either a populated TenantContext + nil error (continue), or a non-
// nil error (deny with HTTP 401).
type TenantResolveFunc func(ctx *fasthttp.RequestCtx) (tenancy.TenantContext, error)

// NewTenantResolveProvider wraps the gate-plugin's resolve function as
// an EnterpriseMiddlewareProvider. Wired during plugin Init().
func NewTenantResolveProvider(resolve TenantResolveFunc) EnterpriseMiddlewareProvider {
	return &tenantResolveProvider{resolve: resolve}
}

type tenantResolveProvider struct {
	resolve TenantResolveFunc
}

func (p *tenantResolveProvider) Name() string { return "tenant-resolve" }

func (p *tenantResolveProvider) Middleware() schemas.BifrostHTTPMiddleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			tc, err := p.resolve(ctx)
			if err != nil {
				ctx.SetStatusCode(fasthttp.StatusUnauthorized)
				ctx.SetContentType("application/json")
				ctx.SetBodyString(`{"error":{"type":"authentication_error","message":"` + err.Error() + `"}}`)
				return
			}
			// Stash on the fasthttp request context for downstream handlers.
			ctx.SetUserValue(string(tenancy.BifrostContextKeyTenantContext), tc)
			ctx.SetUserValue(string(tenancy.BifrostContextKeyOrganizationID), tc.OrganizationID)
			ctx.SetUserValue(string(tenancy.BifrostContextKeyWorkspaceID), tc.WorkspaceID)
			ctx.SetUserValue(string(tenancy.BifrostContextKeyRoleScopes), tc.RoleScopes)
			next(ctx)
		}
	}
}

// NewRBACEnforceProvider returns a middleware that checks every
// request against the route's required scope. Routes register their
// required scope by setting a user-value `enterprise.required_scope`
// before the chain executes; the middleware reads it and denies
// requests whose TenantContext lacks the scope.
//
// Routes WITHOUT a required_scope are allowed through (e.g., the
// org-creation endpoint that bootstraps the very first tenant).
func NewRBACEnforceProvider() EnterpriseMiddlewareProvider {
	return &rbacEnforceProvider{}
}

type rbacEnforceProvider struct{}

func (p *rbacEnforceProvider) Name() string { return "rbac-enforce" }

func (p *rbacEnforceProvider) Middleware() schemas.BifrostHTTPMiddleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			req := ctx.UserValue("enterprise.required_scope")
			if req == nil {
				// Route doesn't require any scope; pass through.
				next(ctx)
				return
			}
			required, ok := req.(string)
			if !ok || required == "" {
				next(ctx)
				return
			}

			tcv := ctx.UserValue(string(tenancy.BifrostContextKeyTenantContext))
			tc, ok := tcv.(tenancy.TenantContext)
			if !ok {
				ctx.SetStatusCode(fasthttp.StatusUnauthorized)
				ctx.SetContentType("application/json")
				ctx.SetBodyString(`{"error":{"type":"authentication_error","message":"tenant context missing — tenant-resolve middleware did not run"}}`)
				return
			}

			// Wildcard "*" satisfies any scope.
			for _, s := range tc.RoleScopes {
				if s == "*" || s == required {
					next(ctx)
					return
				}
			}

			ctx.SetStatusCode(fasthttp.StatusForbidden)
			ctx.SetContentType("application/json")
			ctx.SetBodyString(`{"error":{"type":"permission_error","message":"missing required scope: ` + required + `"}}`)
		}
	}
}

// RequireScope marks the current route as requiring `scope` (e.g.
// "workspaces:write"). Call this once when registering a handler:
//
//	router.POST("/v1/admin/workspaces", lib.ChainMiddlewares(
//	    handler,
//	    append(lib.EnterpriseMiddlewares(), lib.RequireScope("workspaces:write"))...,
//	))
func RequireScope(scope string) schemas.BifrostHTTPMiddleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			ctx.SetUserValue("enterprise.required_scope", scope)
			next(ctx)
		}
	}
}
