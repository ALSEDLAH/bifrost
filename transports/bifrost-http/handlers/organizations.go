// Organizations HTTP handler — /v1/admin/organizations/* endpoints.
//
// Per the Admin API OpenAPI contract (see specs/001-enterprise-parity/
// contracts/admin-api.openapi.yaml). In v1 (single-org mode) only
// /current is exposed; multi-org Create/List endpoints activate when
// config.enterprise.deployment.multi_org_enabled == true.

package handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/deploymentmode"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// OrganizationsHandler serves /v1/admin/organizations/*.
type OrganizationsHandler struct {
	orgRepo *tenancy.OrgRepo
	logger  schemas.Logger
}

// NewOrganizationsHandler constructs the handler. The configStore
// param is the SAME configstore that backs the rest of Bifrost —
// we reuse its *gorm.DB so migrations and reads share one connection
// pool.
func NewOrganizationsHandler(configStore configstore.ConfigStore, logger schemas.Logger) *OrganizationsHandler {
	return &OrganizationsHandler{
		orgRepo: tenancy.NewOrgRepo(configStore.DB()),
		logger:  logger,
	}
}

// RegisterRoutes wires the organization endpoints. Caller supplies
// the per-route middleware chain (tenant-resolve + RBAC + any other
// common middleware) via middlewares.
func (h *OrganizationsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	// GET /v1/admin/organizations/current — every authenticated
	// caller can read their own org; no specific scope needed.
	r.GET("/v1/admin/organizations/current",
		lib.ChainMiddlewares(h.handleGetCurrent, middlewares...))

	// PATCH /v1/admin/organizations/current — Admin-or-Owner only.
	r.PATCH("/v1/admin/organizations/current", lib.ChainMiddlewares(
		h.handlePatchCurrent,
		append(middlewares, lib.RequireScope("team_mgmt:write"))...,
	))
}

// handleGetCurrent returns the org resolved from the caller's
// TenantContext. In single-org mode, this is always the default org.
func (h *OrganizationsHandler) handleGetCurrent(ctx *fasthttp.RequestCtx) {
	tc, ok := ctx.UserValue(string(tenancy.BifrostContextKeyTenantContext)).(tenancy.TenantContext)
	if !ok {
		writeJSONError(ctx, fasthttp.StatusUnauthorized, "authentication_error",
			"tenant context missing — tenant-resolve middleware did not run")
		return
	}

	org, err := h.orgRepo.GetByID(context.Background(), tc.OrganizationID)
	if err != nil {
		if errors.Is(err, tenancy.ErrOrgNotFound) {
			writeJSONError(ctx, fasthttp.StatusNotFound, "not_found", "organization not found")
			return
		}
		h.logWarn("orgs.GetByID: %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"id":                     org.ID,
		"name":                   org.Name,
		"is_default":             org.IsDefault,
		"sso_required":           org.SSORequired,
		"break_glass_enabled":    org.BreakGlassEnabled,
		"default_retention_days": org.DefaultRetentionDays,
		"data_residency_region":  org.DataResidencyRegion,
		"created_at":             org.CreatedAt,
		"multi_org_enabled":      deploymentmode.MultiOrgEnabled(),
	})
}

// handlePatchCurrent applies a partial update. Writes an audit entry.
func (h *OrganizationsHandler) handlePatchCurrent(ctx *fasthttp.RequestCtx) {
	tc, ok := ctx.UserValue(string(tenancy.BifrostContextKeyTenantContext)).(tenancy.TenantContext)
	if !ok {
		writeJSONError(ctx, fasthttp.StatusUnauthorized, "authentication_error", "tenant context missing")
		return
	}

	var body struct {
		Name                 *string `json:"name,omitempty"`
		SSORequired          *bool   `json:"sso_required,omitempty"`
		BreakGlassEnabled    *bool   `json:"break_glass_enabled,omitempty"`
		DefaultRetentionDays *int    `json:"default_retention_days,omitempty"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &body); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	changes := map[string]any{}
	if body.Name != nil {
		changes["name"] = *body.Name
	}
	if body.SSORequired != nil {
		changes["sso_required"] = *body.SSORequired
	}
	if body.BreakGlassEnabled != nil {
		changes["break_glass_enabled"] = *body.BreakGlassEnabled
	}
	if body.DefaultRetentionDays != nil {
		changes["default_retention_days"] = *body.DefaultRetentionDays
	}

	// Capture before-state for the audit entry.
	before, err := h.orgRepo.GetByID(context.Background(), tc.OrganizationID)
	if err != nil {
		h.logWarn("orgs.GetByID (before): %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}

	updated, err := h.orgRepo.Update(context.Background(), tc.OrganizationID, changes)
	if err != nil {
		h.logWarn("orgs.Update: %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}

	// Emit audit — best-effort BifrostContext reconstruction for the
	// emit helper (the rest of Bifrost uses the real BifrostContext;
	// for the handler side we build a minimal one carrying tenancy).
	bctx := newAuditBctxFromFasthttp(ctx, tc)
	_ = audit.Emit(context.Background(), bctx, audit.Entry{
		Action:       "organization.update",
		ResourceType: "organization",
		ResourceID:   updated.ID,
		Before:       summarizeOrg(before),
		After:        summarizeOrg(updated),
		ActorIP:      ctx.RemoteIP().String(),
	})

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"id":                     updated.ID,
		"name":                   updated.Name,
		"sso_required":           updated.SSORequired,
		"break_glass_enabled":    updated.BreakGlassEnabled,
		"default_retention_days": updated.DefaultRetentionDays,
		"data_residency_region":  updated.DataResidencyRegion,
		"updated_at":             updated.UpdatedAt,
	})
}

func summarizeOrg(o *tables_enterprise.TableOrganization) map[string]any {
	if o == nil {
		return nil
	}
	return map[string]any{
		"name":                   o.Name,
		"sso_required":           o.SSORequired,
		"break_glass_enabled":    o.BreakGlassEnabled,
		"default_retention_days": o.DefaultRetentionDays,
	}
}

func (h *OrganizationsHandler) logWarn(format string, args ...any) {
	if h.logger != nil {
		h.logger.Warn(formatMsg(format, args...))
	}
}
