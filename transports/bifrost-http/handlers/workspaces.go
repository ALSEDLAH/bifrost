// Workspaces HTTP handler — /v1/admin/workspaces/* endpoints.
//
// Per the Admin API OpenAPI contract (specs/001-enterprise-parity/
// contracts/admin-api.openapi.yaml).
//
// Scope requirements (enforced by lib.RequireScope middleware wired at
// route-registration time):
//   GET    /v1/admin/workspaces          -> "workspaces:read"
//   POST   /v1/admin/workspaces          -> "workspaces:write"
//   GET    /v1/admin/workspaces/{id}     -> "workspaces:read"
//   PATCH  /v1/admin/workspaces/{id}     -> "workspaces:write"
//   DELETE /v1/admin/workspaces/{id}     -> "workspaces:delete"

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// WorkspacesHandler serves /v1/admin/workspaces/*.
type WorkspacesHandler struct {
	wsRepo *tenancy.WorkspaceRepo
	logger schemas.Logger
}

// NewWorkspacesHandler constructs the handler.
func NewWorkspacesHandler(configStore configstore.ConfigStore, logger schemas.Logger) *WorkspacesHandler {
	return &WorkspacesHandler{
		wsRepo: tenancy.NewWorkspaceRepo(configStore.DB()),
		logger: logger,
	}
}

// RegisterRoutes wires all workspace endpoints with the appropriate
// scope middleware prepended.
func (h *WorkspacesHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/v1/admin/workspaces", lib.ChainMiddlewares(
		h.handleList,
		append(middlewares, lib.RequireScope("workspaces:read"))...,
	))
	r.POST("/v1/admin/workspaces", lib.ChainMiddlewares(
		h.handleCreate,
		append(middlewares, lib.RequireScope("workspaces:write"))...,
	))
	r.GET("/v1/admin/workspaces/{id}", lib.ChainMiddlewares(
		h.handleGet,
		append(middlewares, lib.RequireScope("workspaces:read"))...,
	))
	r.PATCH("/v1/admin/workspaces/{id}", lib.ChainMiddlewares(
		h.handlePatch,
		append(middlewares, lib.RequireScope("workspaces:write"))...,
	))
	r.DELETE("/v1/admin/workspaces/{id}", lib.ChainMiddlewares(
		h.handleDelete,
		append(middlewares, lib.RequireScope("workspaces:delete"))...,
	))
}

// ─── list ────────────────────────────────────────────────────────────

func (h *WorkspacesHandler) handleList(ctx *fasthttp.RequestCtx) {
	tc, err := h.tenant(ctx)
	if err != nil {
		writeJSONError(ctx, fasthttp.StatusUnauthorized, "authentication_error", err.Error())
		return
	}
	rows, err := h.wsRepo.List(context.Background(), tc.OrganizationID)
	if err != nil {
		h.logWarn("workspaces.List: %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		out = append(out, h.summarize(&rows[i]))
	}
	writeJSON(ctx, fasthttp.StatusOK, out)
}

// ─── create ──────────────────────────────────────────────────────────

func (h *WorkspacesHandler) handleCreate(ctx *fasthttp.RequestCtx) {
	tc, err := h.tenant(ctx)
	if err != nil {
		writeJSONError(ctx, fasthttp.StatusUnauthorized, "authentication_error", err.Error())
		return
	}

	var body struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description,omitempty"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &body); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if body.Name == "" || body.Slug == "" {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "invalid_request", "name and slug are required")
		return
	}

	ws, err := h.wsRepo.Create(context.Background(), tc.OrganizationID, body.Name, body.Slug, body.Description)
	if err != nil {
		if errors.Is(err, tenancy.ErrWorkspaceSlugConflict) {
			writeJSONError(ctx, fasthttp.StatusConflict, "conflict",
				fmt.Sprintf("workspace with slug %q already exists", body.Slug))
			return
		}
		h.logWarn("workspaces.Create: %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}

	bctx := newAuditBctxFromFasthttp(ctx, tc)
	_ = audit.Emit(context.Background(), bctx, audit.Entry{
		Action:              "workspace.create",
		ResourceType:        "workspace",
		ResourceID:          ws.ID,
		After:               h.summarize(ws),
		ActorIP:             ctx.RemoteIP().String(),
		WorkspaceIDOverride: ws.ID,
	})

	writeJSON(ctx, fasthttp.StatusCreated, h.summarize(ws))
}

// ─── get ─────────────────────────────────────────────────────────────

func (h *WorkspacesHandler) handleGet(ctx *fasthttp.RequestCtx) {
	tc, err := h.tenant(ctx)
	if err != nil {
		writeJSONError(ctx, fasthttp.StatusUnauthorized, "authentication_error", err.Error())
		return
	}
	id, _ := ctx.UserValue("id").(string)
	if id == "" {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "invalid_request", "missing workspace id")
		return
	}
	ws, err := h.wsRepo.Get(context.Background(), tc.OrganizationID, id)
	if err != nil {
		if errors.Is(err, tenancy.ErrWorkspaceNotFound) {
			writeJSONError(ctx, fasthttp.StatusNotFound, "not_found", "workspace not found")
			return
		}
		h.logWarn("workspaces.Get: %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, h.summarize(ws))
}

// ─── patch ───────────────────────────────────────────────────────────

func (h *WorkspacesHandler) handlePatch(ctx *fasthttp.RequestCtx) {
	tc, err := h.tenant(ctx)
	if err != nil {
		writeJSONError(ctx, fasthttp.StatusUnauthorized, "authentication_error", err.Error())
		return
	}
	id, _ := ctx.UserValue("id").(string)
	if id == "" {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "invalid_request", "missing workspace id")
		return
	}

	var body struct {
		Name                     *string `json:"name,omitempty"`
		Description              *string `json:"description,omitempty"`
		LogRetentionDays         *int    `json:"log_retention_days,omitempty"`
		MetricRetentionDays      *int    `json:"metric_retention_days,omitempty"`
		PayloadEncryptionEnabled *bool   `json:"payload_encryption_enabled,omitempty"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &body); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	changes := map[string]any{}
	if body.Name != nil {
		changes["name"] = *body.Name
	}
	if body.Description != nil {
		changes["description"] = *body.Description
	}
	if body.LogRetentionDays != nil {
		changes["log_retention_days"] = *body.LogRetentionDays
	}
	if body.MetricRetentionDays != nil {
		changes["metric_retention_days"] = *body.MetricRetentionDays
	}
	if body.PayloadEncryptionEnabled != nil {
		changes["payload_encryption_enabled"] = *body.PayloadEncryptionEnabled
	}

	before, err := h.wsRepo.Get(context.Background(), tc.OrganizationID, id)
	if err != nil {
		if errors.Is(err, tenancy.ErrWorkspaceNotFound) {
			writeJSONError(ctx, fasthttp.StatusNotFound, "not_found", "workspace not found")
			return
		}
		h.logWarn("workspaces.Get (before): %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}

	updated, err := h.wsRepo.Patch(context.Background(), tc.OrganizationID, id, changes)
	if err != nil {
		h.logWarn("workspaces.Patch: %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}

	bctx := newAuditBctxFromFasthttp(ctx, tc)
	_ = audit.Emit(context.Background(), bctx, audit.Entry{
		Action:              "workspace.update",
		ResourceType:        "workspace",
		ResourceID:          updated.ID,
		Before:              h.summarize(before),
		After:               h.summarize(updated),
		ActorIP:             ctx.RemoteIP().String(),
		WorkspaceIDOverride: updated.ID,
	})

	writeJSON(ctx, fasthttp.StatusOK, h.summarize(updated))
}

// ─── delete ──────────────────────────────────────────────────────────

func (h *WorkspacesHandler) handleDelete(ctx *fasthttp.RequestCtx) {
	tc, err := h.tenant(ctx)
	if err != nil {
		writeJSONError(ctx, fasthttp.StatusUnauthorized, "authentication_error", err.Error())
		return
	}
	id, _ := ctx.UserValue("id").(string)
	if id == "" {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "invalid_request", "missing workspace id")
		return
	}

	// Capture before-state for audit.
	before, err := h.wsRepo.Get(context.Background(), tc.OrganizationID, id)
	if err != nil {
		if errors.Is(err, tenancy.ErrWorkspaceNotFound) {
			writeJSONError(ctx, fasthttp.StatusNotFound, "not_found", "workspace not found")
			return
		}
		h.logWarn("workspaces.Get (before delete): %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}

	if err := h.wsRepo.SoftDelete(context.Background(), tc.OrganizationID, id); err != nil {
		if errors.Is(err, tenancy.ErrWorkspaceNotFound) {
			writeJSONError(ctx, fasthttp.StatusNotFound, "not_found", "workspace not found")
			return
		}
		h.logWarn("workspaces.SoftDelete: %v", err)
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "server_error", "internal error")
		return
	}

	// TODO(Train A T043 downstream): revoke VKs associated with this
	// workspace and enqueue a physical-delete job for 30 days from now
	// (US1 edge case). Handled by a separate sweep to keep this request
	// cheap.

	bctx := newAuditBctxFromFasthttp(ctx, tc)
	_ = audit.Emit(context.Background(), bctx, audit.Entry{
		Action:              "workspace.delete",
		ResourceType:        "workspace",
		ResourceID:          id,
		Before:              h.summarize(before),
		Outcome:             "allowed",
		Reason:              "soft-delete (30-day grace)",
		ActorIP:             ctx.RemoteIP().String(),
		WorkspaceIDOverride: id,
	})

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// ─── helpers ─────────────────────────────────────────────────────────

func (h *WorkspacesHandler) tenant(ctx *fasthttp.RequestCtx) (tenancy.TenantContext, error) {
	tc, ok := ctx.UserValue(string(tenancy.BifrostContextKeyTenantContext)).(tenancy.TenantContext)
	if !ok {
		return tenancy.TenantContext{}, errors.New("tenant context missing — tenant-resolve middleware did not run")
	}
	return tc, nil
}

func (h *WorkspacesHandler) summarize(w *tables_enterprise.TableWorkspace) map[string]any {
	if w == nil {
		return nil
	}
	out := map[string]any{
		"id":                         w.ID,
		"organization_id":            w.OrganizationID,
		"name":                       w.Name,
		"slug":                       w.Slug,
		"description":                w.Description,
		"payload_encryption_enabled": w.PayloadEncryptionEnabled,
		"created_at":                 w.CreatedAt,
		"updated_at":                 w.UpdatedAt,
	}
	if w.LogRetentionDays != nil {
		out["log_retention_days"] = *w.LogRetentionDays
	}
	if w.MetricRetentionDays != nil {
		out["metric_retention_days"] = *w.MetricRetentionDays
	}
	return out
}

func (h *WorkspacesHandler) logWarn(format string, args ...any) {
	if h.logger != nil {
		h.logger.Warn(formatMsg(format, args...))
	}
}
