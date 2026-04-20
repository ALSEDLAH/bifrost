// Accessors used by plugins (audit, etc.) to extract a TenantContext
// from either a BifrostContext (plugin chain) or a plain context.Context
// (background workers). Both variants fail with a typed error when no
// tenant context has been populated — callers typically log and fall
// through to a synthetic-default attribution rather than panic.

package tenancy

import (
	"context"
	"errors"

	"github.com/maximhq/bifrost/core/schemas"
)

// ErrNoTenantContext is returned when a tenant context lookup finds
// nothing in the context chain.
var ErrNoTenantContext = errors.New("tenancy: no tenant context in context chain")

// FromContext extracts a TenantContext from a BifrostContext. The
// BifrostContextKeyTenantContext key is authoritative; individual
// organization/workspace/user keys are read as a fallback when the
// typed struct isn't present (e.g. an older plugin populated only the
// individual keys).
func FromContext(bctx *schemas.BifrostContext) (TenantContext, error) {
	if bctx == nil {
		return TenantContext{}, ErrNoTenantContext
	}
	if v := bctx.Value(BifrostContextKeyTenantContext); v != nil {
		if tc, ok := v.(TenantContext); ok && !tc.IsZero() {
			return tc, nil
		}
	}
	// Fallback: reconstruct from individual keys if the typed struct
	// was not set. Required at minimum an OrganizationID.
	orgID, _ := bctx.Value(BifrostContextKeyOrganizationID).(string)
	if orgID == "" {
		return TenantContext{}, ErrNoTenantContext
	}
	tc := TenantContext{OrganizationID: orgID}
	if wsID, ok := bctx.Value(BifrostContextKeyWorkspaceID).(string); ok {
		tc.WorkspaceID = wsID
	}
	if userID, ok := bctx.Value(BifrostContextKeyUserID).(string); ok {
		tc.UserID = userID
	}
	if scopes, ok := bctx.Value(BifrostContextKeyRoleScopes).([]string); ok {
		tc.RoleScopes = scopes
	}
	if rv, ok := bctx.Value(BifrostContextKeyResolvedVia).(Resolver); ok {
		tc.ResolvedVia = rv
	}
	return tc, nil
}

// FromGoContext extracts a TenantContext from a plain context.Context.
// Workers that don't carry a BifrostContext (async cleanups, periodic
// tasks) use this path. Absence is normal — returns ErrNoTenantContext
// so the caller can fall back to synthetic-default attribution.
func FromGoContext(ctx context.Context) (TenantContext, error) {
	if ctx == nil {
		return TenantContext{}, ErrNoTenantContext
	}
	if v := ctx.Value(BifrostContextKeyTenantContext); v != nil {
		if tc, ok := v.(TenantContext); ok && !tc.IsZero() {
			return tc, nil
		}
	}
	return TenantContext{}, ErrNoTenantContext
}
