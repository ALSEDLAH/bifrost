// SCIM config handlers — /api/scim/config (spec 009).
//
// Phase 1: stores the bearer-token hash + enabled flag. The /scim/v2/*
// HTTP endpoints are a phase-2 follow-up.

package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type SCIMConfigHandler struct {
	store  configstore.ConfigStore
	logger schemas.Logger
}

func NewSCIMConfigHandler(store configstore.ConfigStore, logger schemas.Logger) *SCIMConfigHandler {
	return &SCIMConfigHandler{store: store, logger: logger}
}

func (h *SCIMConfigHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/scim/config", lib.ChainMiddlewares(h.get, middlewares...))
	r.PATCH("/api/scim/config", lib.ChainMiddlewares(h.patch, middlewares...))
	r.POST("/api/scim/config/rotate", lib.ChainMiddlewares(h.rotate, middlewares...))
}

type scimConfigResponse struct {
	Enabled        bool       `json:"enabled"`
	EndpointURL    string     `json:"endpoint_url"`
	TokenPrefix    string     `json:"token_prefix,omitempty"`
	TokenCreatedAt *time.Time `json:"token_created_at,omitempty"`
}

func (h *SCIMConfigHandler) get(ctx *fasthttp.RequestCtx) {
	row, err := h.store.GetSCIMConfig(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	resp := scimConfigResponse{EndpointURL: scimEndpointURL(ctx)}
	if row != nil {
		resp.Enabled = row.Enabled
		resp.TokenPrefix = row.TokenPrefix
		resp.TokenCreatedAt = row.TokenCreatedAt
	}
	SendJSON(ctx, resp)
}

func (h *SCIMConfigHandler) patch(ctx *fasthttp.RequestCtx) {
	var req struct {
		Enabled *bool `json:"enabled,omitempty"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	row, err := h.store.GetSCIMConfig(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if row == nil {
		row = &tables_enterprise.TableSCIMConfig{}
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if err := h.store.UpsertSCIMConfig(ctx, row); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	SendJSON(ctx, scimConfigResponse{
		Enabled:        row.Enabled,
		EndpointURL:    scimEndpointURL(ctx),
		TokenPrefix:    row.TokenPrefix,
		TokenCreatedAt: row.TokenCreatedAt,
	})
}

func (h *SCIMConfigHandler) rotate(ctx *fasthttp.RequestCtx) {
	plaintext, err := generateSCIMToken()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	hash := sha256.Sum256([]byte(plaintext))
	prefix := plaintext[:8]
	now := time.Now().UTC()

	row, err := h.store.GetSCIMConfig(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if row == nil {
		row = &tables_enterprise.TableSCIMConfig{Enabled: true}
	}
	row.BearerTokenHash = hex.EncodeToString(hash[:])
	row.TokenPrefix = prefix
	row.TokenCreatedAt = &now
	if err := h.store.UpsertSCIMConfig(ctx, row); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	SendJSON(ctx, map[string]any{
		"token":            plaintext,
		"token_prefix":     prefix,
		"token_created_at": now.Format(time.RFC3339),
		"enabled":          row.Enabled,
	})
}

func generateSCIMToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return "scim_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func scimEndpointURL(ctx *fasthttp.RequestCtx) string {
	host := string(ctx.Request.Host())
	scheme := "https"
	if string(ctx.URI().Scheme()) == "http" {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/scim/v2", scheme, host)
}
