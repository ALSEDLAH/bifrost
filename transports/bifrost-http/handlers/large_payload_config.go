// Large payload config handlers — /api/config/large-payload (spec 006).

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

type LargePayloadConfigHandler struct {
	store  configstore.ConfigStore
	config *lib.Config
	logger schemas.Logger
}

func NewLargePayloadConfigHandler(store configstore.ConfigStore, config *lib.Config, logger schemas.Logger) *LargePayloadConfigHandler {
	return &LargePayloadConfigHandler{store: store, config: config, logger: logger}
}

func (h *LargePayloadConfigHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/config/large-payload", lib.ChainMiddlewares(h.get, middlewares...))
	r.PUT("/api/config/large-payload", lib.ChainMiddlewares(h.update, middlewares...))
}

type largePayloadConfigDTO struct {
	Enabled                bool  `json:"enabled"`
	RequestThresholdBytes  int64 `json:"request_threshold_bytes"`
	ResponseThresholdBytes int64 `json:"response_threshold_bytes"`
	PrefetchSizeBytes      int64 `json:"prefetch_size_bytes"`
	MaxPayloadBytes        int64 `json:"max_payload_bytes"`
	TruncatedLogBytes      int64 `json:"truncated_log_bytes"`
}

// DefaultLargePayloadConfigDTO mirrors the UI's DefaultLargePayloadConfig.
var DefaultLargePayloadConfigDTO = largePayloadConfigDTO{
	Enabled:                false,
	RequestThresholdBytes:  10 * 1024 * 1024,  // 10 MB
	ResponseThresholdBytes: 10 * 1024 * 1024,  // 10 MB
	PrefetchSizeBytes:      64 * 1024,         // 64 KB
	MaxPayloadBytes:        500 * 1024 * 1024, // 500 MB
	TruncatedLogBytes:      1024 * 1024,       // 1 MB
}

func (h *LargePayloadConfigHandler) get(ctx *fasthttp.RequestCtx) {
	row, err := h.store.GetLargePayloadConfig(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if row == nil {
		SendJSON(ctx, DefaultLargePayloadConfigDTO)
		return
	}
	SendJSON(ctx, largePayloadConfigDTO{
		Enabled:                row.Enabled,
		RequestThresholdBytes:  row.RequestThresholdBytes,
		ResponseThresholdBytes: row.ResponseThresholdBytes,
		PrefetchSizeBytes:      row.PrefetchSizeBytes,
		MaxPayloadBytes:        row.MaxPayloadBytes,
		TruncatedLogBytes:      row.TruncatedLogBytes,
	})
}

func (h *LargePayloadConfigHandler) update(ctx *fasthttp.RequestCtx) {
	var req largePayloadConfigDTO
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	row := &tables_enterprise.TableLargePayloadConfig{
		Enabled:                req.Enabled,
		RequestThresholdBytes:  req.RequestThresholdBytes,
		ResponseThresholdBytes: req.ResponseThresholdBytes,
		PrefetchSizeBytes:      req.PrefetchSizeBytes,
		MaxPayloadBytes:        req.MaxPayloadBytes,
		TruncatedLogBytes:      req.TruncatedLogBytes,
	}
	if err := h.store.UpsertLargePayloadConfig(ctx, row); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	// Update the in-process config so the next request picks up the new
	// StreamingDecompressThreshold without restart (FR-003).
	if h.config != nil && req.Enabled {
		h.config.StreamingDecompressThreshold = req.RequestThresholdBytes
	}
	SendJSON(ctx, req)
}
