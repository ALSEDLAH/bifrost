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
