// Adaptive routing status — /api/adaptive-routing/status (spec 012).
//
// Read-only: returns the current per-(provider, model) health snapshot
// from framework/adaptiverouting. No DB hit on request path.

package handlers

import (
	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/adaptiverouting"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type AdaptiveRoutingHandler struct {
	tracker *adaptiverouting.Tracker
}

func NewAdaptiveRoutingHandler(tracker *adaptiverouting.Tracker) *AdaptiveRoutingHandler {
	return &AdaptiveRoutingHandler{tracker: tracker}
}

func (h *AdaptiveRoutingHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/adaptive-routing/status", lib.ChainMiddlewares(h.status, middlewares...))
}

func (h *AdaptiveRoutingHandler) status(ctx *fasthttp.RequestCtx) {
	if h.tracker == nil {
		SendJSON(ctx, adaptiverouting.Status{Providers: []adaptiverouting.Entry{}})
		return
	}
	SendJSON(ctx, h.tracker.Status())
}
