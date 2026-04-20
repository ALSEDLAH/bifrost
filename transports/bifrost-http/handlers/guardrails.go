// Guardrails admin handlers — /api/guardrails/{providers,rules} (spec 010).

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

type GuardrailsHandler struct {
	store       configstore.ConfigStore
	invalidator func() // optional — called after successful CRUD mutations (spec 016 FR-007)
	logger      schemas.Logger
}

func NewGuardrailsHandler(store configstore.ConfigStore, logger schemas.Logger) *GuardrailsHandler {
	return &GuardrailsHandler{store: store, logger: logger}
}

// SetInvalidator installs a callback invoked after every successful
// rule / provider CRUD mutation. The guardrails runtime plugin wires
// its Invalidate() here at server startup so edits propagate without
// waiting for a periodic reload.
func (h *GuardrailsHandler) SetInvalidator(fn func()) {
	if h == nil {
		return
	}
	h.invalidator = fn
}

func (h *GuardrailsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/guardrails/providers", lib.ChainMiddlewares(h.listProviders, middlewares...))
	r.POST("/api/guardrails/providers", lib.ChainMiddlewares(h.createProvider, middlewares...))
	r.PATCH("/api/guardrails/providers/{id}", lib.ChainMiddlewares(h.updateProvider, middlewares...))
	r.DELETE("/api/guardrails/providers/{id}", lib.ChainMiddlewares(h.deleteProvider, middlewares...))

	r.GET("/api/guardrails/rules", lib.ChainMiddlewares(h.listRules, middlewares...))
	r.POST("/api/guardrails/rules", lib.ChainMiddlewares(h.createRule, middlewares...))
	r.PATCH("/api/guardrails/rules/{id}", lib.ChainMiddlewares(h.updateRule, middlewares...))
	r.DELETE("/api/guardrails/rules/{id}", lib.ChainMiddlewares(h.deleteRule, middlewares...))
}

// ---- Provider handlers -----------------------------------------------

type guardrailProviderRequest struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Config  json.RawMessage `json:"config"`
	Enabled *bool           `json:"enabled,omitempty"`
}

var validProviderTypes = map[string]bool{"openai-moderation": true, "regex": true, "custom-webhook": true}

func (h *GuardrailsHandler) listProviders(ctx *fasthttp.RequestCtx) {
	rows, err := h.store.ListGuardrailProviders(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []tables_enterprise.TableGuardrailProvider{}
	}
	SendJSON(ctx, map[string]any{"providers": rows})
}

func (h *GuardrailsHandler) createProvider(ctx *fasthttp.RequestCtx) {
	var req guardrailProviderRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.Name == "" || req.Type == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name and type are required")
		return
	}
	if !validProviderTypes[req.Type] {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("unsupported provider type %q", req.Type))
		return
	}
	configStr := "{}"
	if len(req.Config) > 0 {
		configStr = string(req.Config)
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p := &tables_enterprise.TableGuardrailProvider{
		Name:    req.Name,
		Type:    req.Type,
		Config:  configStr,
		Enabled: enabled,
	}
	if err := h.store.CreateGuardrailProvider(ctx, p); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if h.invalidator != nil {
		h.invalidator()
	}
	SendJSONWithStatus(ctx, p, fasthttp.StatusCreated)
}

func (h *GuardrailsHandler) updateProvider(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	existing, err := h.store.GetGuardrailProviderByID(ctx, id)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		SendError(ctx, fasthttp.StatusNotFound, "guardrail provider not found")
		return
	}
	var req guardrailProviderRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if len(req.Config) > 0 {
		existing.Config = string(req.Config)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := h.store.UpdateGuardrailProvider(ctx, existing); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if h.invalidator != nil {
		h.invalidator()
	}
	SendJSON(ctx, existing)
}

func (h *GuardrailsHandler) deleteProvider(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	count, err := h.store.CountGuardrailRulesByProvider(ctx, id)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if count > 0 {
		SendError(ctx, fasthttp.StatusConflict, fmt.Sprintf("%d rule(s) still reference this provider", count))
		return
	}
	if err := h.store.DeleteGuardrailProvider(ctx, id); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if h.invalidator != nil {
		h.invalidator()
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// ---- Rule handlers ---------------------------------------------------

type guardrailRuleRequest struct {
	Name       string `json:"name"`
	ProviderID string `json:"provider_id"`
	Trigger    string `json:"trigger"`
	Action     string `json:"action"`
	Pattern    string `json:"pattern"`
	Enabled    *bool  `json:"enabled,omitempty"`
}

var validTriggers = map[string]bool{"input": true, "output": true, "both": true}
var validActions = map[string]bool{"block": true, "flag": true, "log": true}

func (h *GuardrailsHandler) listRules(ctx *fasthttp.RequestCtx) {
	rows, err := h.store.ListGuardrailRules(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []tables_enterprise.TableGuardrailRule{}
	}
	SendJSON(ctx, map[string]any{"rules": rows})
}

func (h *GuardrailsHandler) createRule(ctx *fasthttp.RequestCtx) {
	var req guardrailRuleRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.Name == "" || req.Trigger == "" || req.Action == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name, trigger, action are required")
		return
	}
	if !validTriggers[req.Trigger] {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid trigger %q (expected input|output|both)", req.Trigger))
		return
	}
	if !validActions[req.Action] {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid action %q (expected block|flag|log)", req.Action))
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	r := &tables_enterprise.TableGuardrailRule{
		Name:       req.Name,
		ProviderID: req.ProviderID,
		Trigger:    req.Trigger,
		Action:     req.Action,
		Pattern:    req.Pattern,
		Enabled:    enabled,
	}
	if err := h.store.CreateGuardrailRule(ctx, r); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if h.invalidator != nil {
		h.invalidator()
	}
	SendJSONWithStatus(ctx, r, fasthttp.StatusCreated)
}

func (h *GuardrailsHandler) updateRule(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	existing, err := h.store.GetGuardrailRuleByID(ctx, id)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		SendError(ctx, fasthttp.StatusNotFound, "guardrail rule not found")
		return
	}
	var req guardrailRuleRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.ProviderID != "" {
		existing.ProviderID = req.ProviderID
	}
	if req.Trigger != "" {
		if !validTriggers[req.Trigger] {
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid trigger %q", req.Trigger))
			return
		}
		existing.Trigger = req.Trigger
	}
	if req.Action != "" {
		if !validActions[req.Action] {
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid action %q", req.Action))
			return
		}
		existing.Action = req.Action
	}
	if req.Pattern != "" {
		existing.Pattern = req.Pattern
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := h.store.UpdateGuardrailRule(ctx, existing); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if h.invalidator != nil {
		h.invalidator()
	}
	SendJSON(ctx, existing)
}

func (h *GuardrailsHandler) deleteRule(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	if err := h.store.DeleteGuardrailRule(ctx, id); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if h.invalidator != nil {
		h.invalidator()
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
