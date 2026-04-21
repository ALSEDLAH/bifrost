// Log export connector handlers — /api/log-export/connectors (spec 008).

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

type LogExportConnectorsHandler struct {
	store       configstore.ConfigStore
	invalidator func() // optional — called after successful CRUD mutations (spec 017 FR-003)
	logger      schemas.Logger
}

func NewLogExportConnectorsHandler(store configstore.ConfigStore, logger schemas.Logger) *LogExportConnectorsHandler {
	return &LogExportConnectorsHandler{store: store, logger: logger}
}

// SetInvalidator installs a callback invoked after every successful
// connector CRUD mutation. The logexport runtime plugin wires its
// Invalidate() here at server startup so new/updated/deleted
// connectors take effect on the next Inject (<1s) rather than
// waiting the 30s TTL.
func (h *LogExportConnectorsHandler) SetInvalidator(fn func()) {
	if h == nil {
		return
	}
	h.invalidator = fn
}

func (h *LogExportConnectorsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/log-export/connectors", lib.ChainMiddlewares(h.list, middlewares...))
	r.POST("/api/log-export/connectors", lib.ChainMiddlewares(h.create, middlewares...))
	r.PATCH("/api/log-export/connectors/{id}", lib.ChainMiddlewares(h.update, middlewares...))
	r.DELETE("/api/log-export/connectors/{id}", lib.ChainMiddlewares(h.delete, middlewares...))
}

type logExportRequest struct {
	Type    string          `json:"type"`
	Name    string          `json:"name"`
	Config  json.RawMessage `json:"config"`
	Enabled *bool           `json:"enabled,omitempty"`
}

var validConnectorTypes = map[string]bool{"datadog": true, "bigquery": true}

func (h *LogExportConnectorsHandler) list(ctx *fasthttp.RequestCtx) {
	typeFilter := string(ctx.QueryArgs().Peek("type"))
	rows, err := h.store.ListLogExportConnectors(ctx, typeFilter)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []tables_enterprise.TableLogExportConnector{}
	}
	SendJSON(ctx, map[string]any{"connectors": rows})
}

func (h *LogExportConnectorsHandler) create(ctx *fasthttp.RequestCtx) {
	var req logExportRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.Type == "" || req.Name == "" || len(req.Config) == 0 {
		SendError(ctx, fasthttp.StatusBadRequest, "type, name, and config are required")
		return
	}
	if !validConnectorTypes[req.Type] {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("unsupported connector type %q (expected datadog|bigquery)", req.Type))
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row := &tables_enterprise.TableLogExportConnector{
		Type:    req.Type,
		Name:    req.Name,
		Config:  string(req.Config),
		Enabled: enabled,
	}
	if err := h.store.CreateLogExportConnector(ctx, row); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if h.invalidator != nil {
		h.invalidator()
	}
	SendJSONWithStatus(ctx, row, fasthttp.StatusCreated)
}

func (h *LogExportConnectorsHandler) update(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	existing, err := h.store.GetLogExportConnectorByID(ctx, id)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		SendError(ctx, fasthttp.StatusNotFound, "connector not found")
		return
	}
	var req logExportRequest
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
	if err := h.store.UpdateLogExportConnector(ctx, existing); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if h.invalidator != nil {
		h.invalidator()
	}
	SendJSON(ctx, existing)
}

func (h *LogExportConnectorsHandler) delete(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	if err := h.store.DeleteLogExportConnector(ctx, id); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if h.invalidator != nil {
		h.invalidator()
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
