// Audit emission helpers for the RBAC admin handlers (C2 remediation —
// FR-010 "System MUST record an audit entry for every administrative
// action").
//
// The rbac handlers have no per-request tenant resolution middleware
// wired yet, so this file synthesizes a minimum-viable TenantContext
// from the default org UUID (same one the handlers already read from
// ent_system_defaults). When real session → tenant middleware lands,
// callers can swap `buildAuditCtx` for a middleware-populated context
// without changing the audit.Emit signatures.

package handlers

import (
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
	"github.com/valyala/fasthttp"
)

func buildAuditCtx(ctx *fasthttp.RequestCtx, orgID string) *schemas.BifrostContext {
	// 10s deadline is plenty for a synchronous audit write; callers pass
	// the request ctx so deadline cancellation still propagates.
	bctx := schemas.NewBifrostContext(ctx, time.Now().Add(10*time.Second))
	tc := tenancy.TenantContext{
		OrganizationID: orgID,
		ResolvedVia:    tenancy.ResolverDefault,
	}
	bctx.SetValue(tenancy.BifrostContextKeyTenantContext, tc)
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, orgID)
	bctx.SetValue(tenancy.BifrostContextKeyResolvedVia, tenancy.ResolverDefault)
	return bctx
}

// emitAudit is a thin wrapper around audit.Emit that swallows errors
// after logging (admin handlers shouldn't fail their primary work just
// because the audit pipeline is momentarily unavailable — the sync
// sink failure is itself logged at Warn level).
func emitAudit(ctx *fasthttp.RequestCtx, orgID, action, resourceType, resourceID string, before, after any) {
	bctx := buildAuditCtx(ctx, orgID)
	entry := audit.Entry{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      "allowed",
		ActorIP:      ctx.RemoteIP().String(),
		Before:       before,
		After:        after,
	}
	if err := audit.Emit(ctx, bctx, entry); err != nil {
		logger.Warn("audit.Emit %s: %v (action not audited)", action, err)
	}
}
