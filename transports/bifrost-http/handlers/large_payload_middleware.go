// Large-payload context injector (spec 006 phase 2).
//
// Reads the singleton ent_large_payload_config row (with a TTL-cached
// in-process copy) and sets the BifrostContextKeyLargeResponseThreshold
// + BifrostContextKeyLargePayloadPrefetchSize user-values on each
// request. Provider clients already consume these keys — the only
// missing link was a writer. This middleware is that writer.

package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/valyala/fasthttp"
)

// largePayloadCache holds the current config plus an expiry. The
// handler/update path calls Invalidate() on save so admins don't have
// to wait the full TTL for new values to take effect.
type largePayloadCache struct {
	mu       sync.RWMutex
	expires  time.Time
	enabled  bool
	respThr  int64
	prefetch int64
}

var lpCache = &largePayloadCache{}

// InvalidateLargePayloadCache drops the cached config. Called by the
// admin PUT /api/config/large-payload handler after a successful save.
func InvalidateLargePayloadCache() {
	lpCache.mu.Lock()
	lpCache.expires = time.Time{}
	lpCache.mu.Unlock()
}

const largePayloadCacheTTL = 30 * time.Second

// LargePayloadMiddleware returns a middleware that stamps each
// inference request with the admin-configured response threshold and
// prefetch size. Safe to register even if the enterprise config table
// is empty — the middleware short-circuits to a no-op.
func LargePayloadMiddleware(store configstore.ConfigStore) schemas.BifrostHTTPMiddleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			enabled, resp, prefetch := lpCache.read(store)
			if enabled {
				if resp > 0 {
					ctx.SetUserValue(schemas.BifrostContextKeyLargeResponseThreshold, resp)
				}
				if prefetch > 0 {
					ctx.SetUserValue(schemas.BifrostContextKeyLargePayloadPrefetchSize, int(prefetch))
				}
			}
			next(ctx)
		}
	}
}

func (c *largePayloadCache) read(store configstore.ConfigStore) (enabled bool, respThr, prefetch int64) {
	c.mu.RLock()
	fresh := time.Now().Before(c.expires)
	if fresh {
		enabled, respThr, prefetch = c.enabled, c.respThr, c.prefetch
	}
	c.mu.RUnlock()
	if fresh {
		return
	}
	// Slow path: reload.
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.expires) {
		return c.enabled, c.respThr, c.prefetch
	}
	row, err := store.GetLargePayloadConfig(context.Background())
	if err != nil || row == nil {
		c.enabled = false
		c.respThr = 0
		c.prefetch = 0
	} else {
		c.enabled = row.Enabled
		c.respThr = row.ResponseThresholdBytes
		c.prefetch = row.PrefetchSizeBytes
	}
	c.expires = time.Now().Add(largePayloadCacheTTL)
	return c.enabled, c.respThr, c.prefetch
}
