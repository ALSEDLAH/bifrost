// Cluster status handler — /api/cluster/status (spec 007).
//
// Single-node v1: returns self-reported hostname, version, uptime,
// pid, and runtime memory stats. Multi-node coordination is a
// future spec — node_role is always "standalone" today.

package handlers

import (
	"os"
	"runtime"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

var clusterStartedAt = time.Now().UTC()

type ClusterStatusHandler struct {
	version string
}

func NewClusterStatusHandler(version string) *ClusterStatusHandler {
	if version == "" {
		version = "unknown"
	}
	return &ClusterStatusHandler{version: version}
}

func (h *ClusterStatusHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/cluster/status", lib.ChainMiddlewares(h.get, middlewares...))
}

type clusterStatusResponse struct {
	NodeRole           string `json:"node_role"`
	Hostname           string `json:"hostname"`
	Version            string `json:"version"`
	StartedAt          string `json:"started_at"`
	UptimeSeconds      int64  `json:"uptime_seconds"`
	PID                int    `json:"pid"`
	Goroutines         int    `json:"goroutines"`
	ProcessMemoryBytes uint64 `json:"process_memory_bytes"`
}

func (h *ClusterStatusHandler) get(ctx *fasthttp.RequestCtx) {
	hostname, _ := os.Hostname()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	SendJSON(ctx, clusterStatusResponse{
		NodeRole:           "standalone",
		Hostname:           hostname,
		Version:            h.version,
		StartedAt:          clusterStartedAt.Format(time.RFC3339),
		UptimeSeconds:      int64(time.Since(clusterStartedAt).Seconds()),
		PID:                os.Getpid(),
		Goroutines:         runtime.NumGoroutine(),
		ProcessMemoryBytes: m.Alloc,
	})
}
