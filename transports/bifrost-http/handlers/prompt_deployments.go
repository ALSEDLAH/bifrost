// Prompt deployment handlers — /api/prompts/:prompt_id/deployments (spec 011).

package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type PromptDeploymentsHandler struct {
	store  configstore.ConfigStore
	cache  *configstore.ProductionDeploymentCache
	logger schemas.Logger
}

func NewPromptDeploymentsHandler(store configstore.ConfigStore, cache *configstore.ProductionDeploymentCache, logger schemas.Logger) *PromptDeploymentsHandler {
	return &PromptDeploymentsHandler{store: store, cache: cache, logger: logger}
}

func (h *PromptDeploymentsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/prompts/{prompt_id}/deployments", lib.ChainMiddlewares(h.list, middlewares...))
	r.PUT("/api/prompts/{prompt_id}/deployments/{label}", lib.ChainMiddlewares(h.upsert, middlewares...))
	r.DELETE("/api/prompts/{prompt_id}/deployments/{label}", lib.ChainMiddlewares(h.delete, middlewares...))
}

var validPromptDeploymentLabels = map[string]bool{"production": true, "staging": true}

type upsertRequest struct {
	VersionID  uint   `json:"version_id"`
	PromotedBy string `json:"promoted_by,omitempty"`
}

func (h *PromptDeploymentsHandler) list(ctx *fasthttp.RequestCtx) {
	promptID := ctx.UserValue("prompt_id").(string)
	rows, err := h.store.ListPromptDeployments(ctx, promptID)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []tables_enterprise.TablePromptDeployment{}
	}
	SendJSON(ctx, map[string]any{"deployments": rows})
}

func (h *PromptDeploymentsHandler) upsert(ctx *fasthttp.RequestCtx) {
	promptID := ctx.UserValue("prompt_id").(string)
	label := ctx.UserValue("label").(string)
	if !validPromptDeploymentLabels[label] {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("unsupported label %q (expected production|staging)", label))
		return
	}
	var req upsertRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.VersionID == 0 {
		SendError(ctx, fasthttp.StatusBadRequest, "version_id is required")
		return
	}
	row := &tables_enterprise.TablePromptDeployment{
		PromptID:   promptID,
		Label:      label,
		VersionID:  req.VersionID,
		PromotedBy: req.PromotedBy,
	}
	if err := h.store.UpsertPromptDeployment(ctx, row); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	// Spec 014 FR-005: drop the runtime resolver cache so the new
	// production label takes effect in <1s rather than waiting for
	// the 30s TTL.
	if label == "production" {
		h.cache.Invalidate()
	}
	SendJSON(ctx, row)
}

func (h *PromptDeploymentsHandler) delete(ctx *fasthttp.RequestCtx) {
	promptID := ctx.UserValue("prompt_id").(string)
	label := ctx.UserValue("label").(string)
	if err := h.store.DeletePromptDeployment(ctx, promptID, label); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if label == "production" {
		h.cache.Invalidate()
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
