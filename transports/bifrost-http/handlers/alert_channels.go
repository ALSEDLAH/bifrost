// Alert channel CRUD handlers — /api/alert-channels (spec 004).
//
// Sibling file to the rbac / audit_logs handlers. Uses the configstore
// interface so other backends (memory, mocked) can plug in for tests.

package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/alertchannels"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// AlertChannelsHandler serves /api/alert-channels/*.
type AlertChannelsHandler struct {
	store      configstore.ConfigStore
	dispatcher *alertchannels.Dispatcher
	logger     schemas.Logger
}

// NewAlertChannelsHandler constructs the handler.
func NewAlertChannelsHandler(store configstore.ConfigStore, dispatcher *alertchannels.Dispatcher, logger schemas.Logger) *AlertChannelsHandler {
	return &AlertChannelsHandler{store: store, dispatcher: dispatcher, logger: logger}
}

// RegisterRoutes wires the alert-channel endpoints.
func (h *AlertChannelsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/alert-channels", lib.ChainMiddlewares(h.list, middlewares...))
	r.POST("/api/alert-channels", lib.ChainMiddlewares(h.create, middlewares...))
	r.PATCH("/api/alert-channels/{id}", lib.ChainMiddlewares(h.update, middlewares...))
	r.DELETE("/api/alert-channels/{id}", lib.ChainMiddlewares(h.delete, middlewares...))
	r.POST("/api/alert-channels/{id}/test", lib.ChainMiddlewares(h.test, middlewares...))
}

type alertChannelRequest struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Config  json.RawMessage `json:"config"`
	Enabled *bool           `json:"enabled,omitempty"`
}

func (h *AlertChannelsHandler) list(ctx *fasthttp.RequestCtx) {
	channels, err := h.store.ListAlertChannels(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if channels == nil {
		channels = []tables_enterprise.TableAlertChannel{}
	}
	SendJSON(ctx, map[string]any{"channels": channels})
}

func (h *AlertChannelsHandler) create(ctx *fasthttp.RequestCtx) {
	var req alertChannelRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.Name == "" || req.Type == "" || len(req.Config) == 0 {
		SendError(ctx, fasthttp.StatusBadRequest, "name, type, and config are required")
		return
	}
	if req.Type != string(alertchannels.ChannelTypeWebhook) && req.Type != string(alertchannels.ChannelTypeSlack) {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("unsupported type %q (expected webhook|slack)", req.Type))
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	ch := &tables_enterprise.TableAlertChannel{
		Name:    req.Name,
		Type:    req.Type,
		Config:  string(req.Config),
		Enabled: enabled,
	}
	if err := h.store.CreateAlertChannel(ctx, ch); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	SendJSONWithStatus(ctx, ch, fasthttp.StatusCreated)
}

func (h *AlertChannelsHandler) update(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	existing, err := h.store.GetAlertChannelByID(ctx, id)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		SendError(ctx, fasthttp.StatusNotFound, "alert channel not found")
		return
	}
	var req alertChannelRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Type != "" {
		if req.Type != string(alertchannels.ChannelTypeWebhook) && req.Type != string(alertchannels.ChannelTypeSlack) {
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("unsupported type %q", req.Type))
			return
		}
		existing.Type = req.Type
	}
	if len(req.Config) > 0 {
		existing.Config = string(req.Config)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := h.store.UpdateAlertChannel(ctx, existing); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	SendJSON(ctx, existing)
}

func (h *AlertChannelsHandler) delete(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	if err := h.store.DeleteAlertChannel(ctx, id); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *AlertChannelsHandler) test(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	existing, err := h.store.GetAlertChannelByID(ctx, id)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		SendError(ctx, fasthttp.StatusNotFound, "alert channel not found")
		return
	}
	if h.dispatcher == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "alert dispatcher not configured")
		return
	}
	h.dispatcher.Send(
		[]tables_enterprise.TableAlertChannel{*existing},
		alertchannels.Event{
			Type:      "alert.test",
			Timestamp: time.Now().UTC(),
			Data:      map[string]any{"channel_id": id, "message": "This is a test alert from Bifrost."},
		},
	)
	SendJSON(ctx, map[string]any{"dispatched": true})
}
