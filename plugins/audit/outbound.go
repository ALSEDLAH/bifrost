// Audit outbound — fans every successful audit insert to the
// configured alert channels so operators get real-time SIEM
// mirroring (spec 018). Reuses the spec-004 dispatcher; no new
// tables or endpoints.

package audit

import (
	"sync"

	"github.com/maximhq/bifrost/framework/alertchannels"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/logstore"
)

// outboundBridge holds the dispatcher + channel fetcher installed
// by the HTTP transport at startup. Protected by a dedicated mutex
// so reads on the Emit hot path don't contend with config reloads.
type outboundBridge struct {
	mu         sync.RWMutex
	dispatcher *alertchannels.Dispatcher
	channelsFn func() []tables_enterprise.TableAlertChannel
}

var defaultOutbound outboundBridge

// SetAlertDispatcher installs a dispatcher + channel-fetcher pair
// on the process-default audit plugin. Both arguments may be nil
// (no-op). Typically called once at server startup, right after
// audit.Init.
func SetAlertDispatcher(d *alertchannels.Dispatcher, channelsFn func() []tables_enterprise.TableAlertChannel) {
	defaultOutbound.mu.Lock()
	defaultOutbound.dispatcher = d
	defaultOutbound.channelsFn = channelsFn
	defaultOutbound.mu.Unlock()
}

// dispatchOutbound sends the row to the alert-channel dispatcher
// (if configured). Safe to call with an unwired bridge — returns
// immediately. The dispatcher itself runs its sends in its own
// goroutines so this call does not block.
func dispatchOutbound(row logstore.TableAuditEntry) {
	defaultOutbound.mu.RLock()
	d := defaultOutbound.dispatcher
	fn := defaultOutbound.channelsFn
	defaultOutbound.mu.RUnlock()
	if d == nil || fn == nil {
		return
	}
	channels := fn()
	if len(channels) == 0 {
		return
	}
	d.Send(channels, alertchannels.Event{
		Type: "audit.entry",
		Data: map[string]any{
			"id":            row.ID,
			"action":        row.Action,
			"resource_type": row.ResourceType,
			"resource_id":   row.ResourceID,
			"outcome":       row.Outcome,
			"actor_type":    row.ActorType,
			"actor_id":      row.ActorID,
			"reason":        row.Reason,
			"request_id":    row.RequestID,
			"created_at":    row.CreatedAt,
		},
	})
}
