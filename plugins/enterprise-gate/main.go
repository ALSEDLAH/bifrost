// Package enterprisegate is the gateway plugin that resolves tenant
// context for every incoming HTTP request and seeds BifrostContext
// keys consumed by every downstream enterprise plugin.
//
// Constitution Principles V (multi-tenancy first), VI (audit emit per
// resolution), and XI (sibling-file extension — no upstream edits).
//
// Resolution order (research R-02):
//
//  1. Authorization: Bearer bf-admin-…  -> Admin API key  (org scope)
//  2. Authorization: Bearer bf-svc-…    -> Service-account key (workspace scope)
//  3. Cookie: bifrost_session=…         -> User session (user + org + workspace)
//  4. x-api-key: sk-bf-…                -> Virtual key   (workspace scope)
//  5. fall-through                      -> Default (synthetic single-org)
//
// On resolution, the plugin sets BifrostContext keys defined in
// framework/tenancy/keys.go. Downstream plugins (audit, license,
// guardrails-central, governance, metering, billing) read them.
//
// Per-resolution audit entries land via plugins/audit.Emit.
package enterprisegate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/deploymentmode"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
	"github.com/maximhq/bifrost/plugins/license"
	"gorm.io/gorm"
)

// PluginName is the registered plugin identifier.
const PluginName = "enterprise-gate"

// Resolver is the public hook that translates an HTTP request into a
// TenantContext. The plugin ships with built-in resolvers for the four
// auth-token types listed above; downstream plugins (sso for session
// cookies) can register additional resolvers via RegisterResolver.
type Resolver interface {
	Name() string
	// Resolve inspects the request; returns (ctx, true, nil) on match,
	// (TenantContext{}, false, nil) on no-match (try next resolver),
	// or (TenantContext{}, true, err) on match-with-error (e.g. expired
	// admin key) — the gate denies the request with HTTP 401.
	Resolve(ctx context.Context, req *schemas.HTTPRequest) (tenancy.TenantContext, bool, error)
}

// Plugin is the enterprise-gate. Holds the configstore + logstore DB
// handles for resolver lookups and the registered Resolver chain.
type Plugin struct {
	configstoreDB *gorm.DB
	logstoreDB    *gorm.DB
	logger        schemas.Logger
	licensePlugin license.LicensePlugin
	resolvers     []Resolver
	defaultOrg    string
	defaultWS     string
}

// Init constructs the plugin, loads the synthetic-default org/workspace
// from ent_system_defaults, and runs the enterprise migration suite for
// both stores so the rest of Phase 2 can rely on the tables existing.
func Init(
	ctx context.Context,
	configstoreDB *gorm.DB,
	logstoreDB *gorm.DB,
	logger schemas.Logger,
	lic license.LicensePlugin,
) (*Plugin, error) {
	if configstoreDB == nil || logstoreDB == nil {
		return nil, errors.New("enterprise-gate: nil db handle")
	}

	// Run enterprise migrations on both stores. Idempotent.
	if err := configstore.RegisterEnterpriseMigrations(ctx, configstoreDB); err != nil {
		return nil, fmt.Errorf("configstore migrations: %w", err)
	}
	if err := logstore.RegisterEnterpriseMigrations(ctx, logstoreDB); err != nil {
		return nil, fmt.Errorf("logstore migrations: %w", err)
	}

	// Read default org/workspace from ent_system_defaults via a typed
	// query. The configstore tables-enterprise package is the canonical
	// home for this struct — we just pull values here.
	var defOrg, defWS string
	row := struct {
		DefaultOrganizationID string
		DefaultWorkspaceID    string
	}{}
	if err := configstoreDB.WithContext(ctx).
		Raw(`SELECT default_organization_id, default_workspace_id FROM ent_system_defaults WHERE id = 'system'`).
		Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("read system_defaults: %w", err)
	}
	defOrg, defWS = row.DefaultOrganizationID, row.DefaultWorkspaceID
	if defOrg == "" || defWS == "" {
		return nil, errors.New("enterprise-gate: system_defaults row not seeded (E001 must run before plugin init)")
	}

	p := &Plugin{
		configstoreDB: configstoreDB,
		logstoreDB:    logstoreDB,
		logger:        logger,
		licensePlugin: lic,
		defaultOrg:    defOrg,
		defaultWS:     defWS,
	}

	// Register the four built-in resolvers in priority order.
	p.resolvers = []Resolver{
		newAdminAPIKeyResolver(configstoreDB),
		newServiceAccountKeyResolver(configstoreDB),
		// Session resolver registers itself from plugins/sso when SSO is
		// enabled — skipped here to keep enterprise-gate independent of
		// the sso plugin (Principle X: no cross-plugin imports).
		newVirtualKeyResolver(configstoreDB),
	}

	return p, nil
}

// GetName satisfies BasePlugin.
func (p *Plugin) GetName() string { return PluginName }

// Cleanup releases resources at shutdown.
func (p *Plugin) Cleanup() error { return nil }

// RegisterResolver appends a resolver to the chain. Used by plugins/sso
// to add the session-cookie resolver after enterprise-gate has booted.
// Registration order = resolution order.
func (p *Plugin) RegisterResolver(r Resolver) {
	p.resolvers = append(p.resolvers, r)
}

// HTTPTransportPreHook is the gateway hook. Runs first in the chain
// (registered as PluginPlacementPreBuiltin with order -1) so every
// downstream plugin sees a populated TenantContext on its bctx.
//
// On any resolver error, returns HTTP 401 to the client.
// On no-resolver-match, falls through to the synthetic default tenant.
func (p *Plugin) HTTPTransportPreHook(bctx *schemas.BifrostContext, req *schemas.HTTPRequest) (*schemas.HTTPResponse, error) {
	if bctx == nil {
		return nil, errors.New("enterprise-gate: nil bctx")
	}

	tc, err := p.resolveOne(bctx.Context(), req)
	if err != nil {
		// Audit the auth failure — best-effort; we can't depend on
		// tenant context (since auth failed) so emit via the audit
		// plugin's lower-level path.
		_ = audit.Emit(bctx.Context(), p.bootstrapBctx(p.defaultBctxValues()), audit.Entry{
			Action:       "auth.denied",
			ResourceType: "request",
			ResourceID:   reqID(req),
			Outcome:      "denied",
			Reason:       err.Error(),
			ActorIP:      remoteIP(req),
		})
		return &schemas.HTTPResponse{
			StatusCode: 401,
			Headers: map[string]string{
				"content-type":           "application/json",
				"x-bifrost-deny-reason":  err.Error(),
				"x-bifrost-resolved-via": "none",
			},
			Body: []byte(`{"error":{"type":"authentication_error","message":"` + jsonEscape(err.Error()) + `"}}`),
		}, nil
	}

	// Set BifrostContext keys for downstream plugins.
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, tc.OrganizationID)
	bctx.SetValue(tenancy.BifrostContextKeyWorkspaceID, tc.WorkspaceID)
	bctx.SetValue(tenancy.BifrostContextKeyUserID, tc.UserID)
	bctx.SetValue(tenancy.BifrostContextKeyRoleScopes, tc.RoleScopes)
	bctx.SetValue(tenancy.BifrostContextKeyResolvedVia, string(tc.ResolvedVia))
	bctx.SetValue(tenancy.BifrostContextKeyTenantContext, tc)

	// Seed license context so downstream gated features can call
	// IsEntitled() without a global license-plugin lookup.
	if p.licensePlugin != nil {
		bctx.SetValue(tenancy.BifrostContextKeyLicense, p.licensePlugin)
	}

	return nil, nil
}

// HTTPTransportPostHook is a no-op for the gate; downstream plugins
// (governance, audit) handle response-side concerns.
func (p *Plugin) HTTPTransportPostHook(bctx *schemas.BifrostContext, req *schemas.HTTPRequest, resp *schemas.HTTPResponse) error {
	return nil
}

// HTTPTransportStreamChunkHook is a no-op for the gate.
func (p *Plugin) HTTPTransportStreamChunkHook(bctx *schemas.BifrostContext, req *schemas.HTTPRequest, chunk *schemas.BifrostStreamChunk) (*schemas.BifrostStreamChunk, error) {
	return chunk, nil
}

// resolveOne runs the resolver chain and returns the first match. If
// no resolver matches AND the deployment is in single-org mode, falls
// through to the synthetic default tenant (so OSS-style anonymous
// requests still work). In multi-org mode, no-match is a 401.
func (p *Plugin) resolveOne(ctx context.Context, req *schemas.HTTPRequest) (tenancy.TenantContext, error) {
	for _, r := range p.resolvers {
		tc, ok, err := r.Resolve(ctx, req)
		if err != nil {
			return tenancy.TenantContext{}, fmt.Errorf("auth via %s: %w", r.Name(), err)
		}
		if ok {
			return tc, nil
		}
	}

	// No resolver matched.
	if deploymentmode.CurrentDefaults().MultiOrgEnabled {
		return tenancy.TenantContext{}, errors.New("no enterprise auth credential found (admin-api-key, service-account-key, session, or virtual-key required)")
	}
	return tenancy.TenantContext{
		OrganizationID: p.defaultOrg,
		WorkspaceID:    p.defaultWS,
		ResolvedVia:    tenancy.ResolverDefault,
	}, nil
}

// defaultBctxValues constructs the tenant fields used by the gate for
// internal audit emits when no auth resolved.
type defaultBctxFields struct {
	org string
	ws  string
}

func (p *Plugin) defaultBctxValues() defaultBctxFields {
	return defaultBctxFields{org: p.defaultOrg, ws: p.defaultWS}
}

// bootstrapBctx creates a synthetic BifrostContext with the default
// tenant set, used when we need to audit auth-time failures (the
// caller's bctx may not have any tenant set yet).
func (p *Plugin) bootstrapBctx(d defaultBctxFields) *schemas.BifrostContext {
	bctx := schemas.NewBifrostContext(context.Background())
	tc := tenancy.TenantContext{
		OrganizationID: d.org,
		WorkspaceID:    d.ws,
		ResolvedVia:    tenancy.ResolverDefault,
	}
	bctx.SetValue(tenancy.BifrostContextKeyTenantContext, tc)
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, tc.OrganizationID)
	bctx.SetValue(tenancy.BifrostContextKeyWorkspaceID, tc.WorkspaceID)
	return bctx
}

// ─── helpers ────────────────────────────────────────────────────────

func reqID(req *schemas.HTTPRequest) string {
	if req == nil {
		return ""
	}
	if v, ok := req.Headers["x-request-id"]; ok {
		return v
	}
	return ""
}

func remoteIP(req *schemas.HTTPRequest) string {
	if req == nil {
		return ""
	}
	if v, ok := req.Headers["x-forwarded-for"]; ok {
		// Take the first IP in the chain.
		if i := strings.IndexByte(v, ','); i >= 0 {
			return strings.TrimSpace(v[:i])
		}
		return v
	}
	if v, ok := req.Headers["x-real-ip"]; ok {
		return v
	}
	return ""
}

func jsonEscape(s string) string {
	// Minimal JSON-string escaping for the inline error body. For
	// anything more, callers should marshal a struct.
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return r.Replace(s)
}

// _ time.Time keeps the time import live in case future resolvers use it.
var _ = time.Time{}
