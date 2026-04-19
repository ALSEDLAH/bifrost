// Tenancy-aware GORM repository helpers.
//
// Every repository function in tables-enterprise/ MUST scope its queries
// by the caller's TenantContext. ScopedDB returns a *gorm.DB pre-filtered
// by organization_id (and optionally workspace_id), so callers cannot
// accidentally leak cross-tenant rows.
//
// Constitution Principle V — multi-tenancy first.
package tenancy

import (
	"context"
	"errors"
	"fmt"

	"github.com/maximhq/bifrost/core/schemas"
	"gorm.io/gorm"
)

// ErrNoTenantContext is returned when a repository call is made without
// a resolved TenantContext in the BifrostContext. This is a programming
// error — every enterprise endpoint must run behind the enterprise-gate
// middleware that resolves tenancy.
var ErrNoTenantContext = errors.New("tenancy: no TenantContext in request — enterprise-gate middleware did not run")

// FromContext extracts the TenantContext written by the enterprise-gate
// plugin's HTTP pre-hook. Returns ErrNoTenantContext when missing.
func FromContext(bctx *schemas.BifrostContext) (TenantContext, error) {
	if bctx == nil {
		return TenantContext{}, ErrNoTenantContext
	}
	v := bctx.GetValue(BifrostContextKeyTenantContext)
	if v == nil {
		return TenantContext{}, ErrNoTenantContext
	}
	tc, ok := v.(TenantContext)
	if !ok {
		return TenantContext{}, fmt.Errorf("tenancy: TenantContext key has unexpected type %T", v)
	}
	if tc.IsZero() {
		return TenantContext{}, ErrNoTenantContext
	}
	return tc, nil
}

// FromGoContext is the std-library context.Context variant for code paths
// that don't have a BifrostContext (e.g., background workers reading the
// same scoped state passed via a Go context value).
func FromGoContext(ctx context.Context) (TenantContext, error) {
	if ctx == nil {
		return TenantContext{}, ErrNoTenantContext
	}
	v := ctx.Value(BifrostContextKeyTenantContext)
	if v == nil {
		return TenantContext{}, ErrNoTenantContext
	}
	tc, ok := v.(TenantContext)
	if !ok {
		return TenantContext{}, fmt.Errorf("tenancy: TenantContext value has unexpected type %T", v)
	}
	return tc, nil
}

// ScopedDB returns a *gorm.DB session pre-filtered by the caller's
// organization_id. When workspaceScoped is true, also adds the
// workspace_id filter — for queries on tables that are workspace-scoped
// (most non-admin-keys / non-org-config tables).
//
// Usage:
//
//	db, err := tenancy.ScopedDB(bctx, configstore.RawDB(), true)
//	if err != nil { return err }
//	var workspaces []TableWorkspace
//	db.Find(&workspaces)
//
// Always returns a NEW session (db.Session{}) — caller does not need to
// worry about modifying a shared *gorm.DB.
func ScopedDB(bctx *schemas.BifrostContext, db *gorm.DB, workspaceScoped bool) (*gorm.DB, error) {
	tc, err := FromContext(bctx)
	if err != nil {
		return nil, err
	}
	scoped := db.Session(&gorm.Session{}).Where("organization_id = ?", tc.OrganizationID)
	if workspaceScoped && tc.WorkspaceID != "" {
		scoped = scoped.Where("workspace_id = ?", tc.WorkspaceID)
	}
	return scoped, nil
}

// ScopedDBOrgOnly is a convenience for callers that want the
// organization-scoped form regardless of workspace context (e.g., audit
// log queries from an admin-key context where workspace_id is empty).
func ScopedDBOrgOnly(bctx *schemas.BifrostContext, db *gorm.DB) (*gorm.DB, error) {
	return ScopedDB(bctx, db, false)
}

// MustHaveScope returns ErrInsufficientScope when the caller does not
// hold the requested resource:verb. Used at the top of write endpoints.
func MustHaveScope(bctx *schemas.BifrostContext, resourceVerb string) error {
	tc, err := FromContext(bctx)
	if err != nil {
		return err
	}
	if !tc.HasScope(resourceVerb) {
		return fmt.Errorf("tenancy: missing required scope %q (resolved via %s)", resourceVerb, tc.ResolvedVia)
	}
	return nil
}
