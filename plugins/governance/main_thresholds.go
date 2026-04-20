// Budget threshold alerts — GovernancePlugin surface (US8, T055; spec 004).
//
// Sibling-file extension per Constitution Principle XI rule 1. Exposes
// two setters so the HTTP transport can wire runtime broadcasters and
// the alert-channel dispatcher at boot without touching upstream main.go.

package governance

import (
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/alertchannels"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

// SetBudgetThresholdBroadcaster installs the WebSocket event
// broadcaster used to push budget.threshold.crossed events to the UI.
// Safe to call on nil plugin; safe to pass nil broadcaster.
func (p *GovernancePlugin) SetBudgetThresholdBroadcaster(fn schemas.EventBroadcaster) {
	if p == nil || p.tracker == nil {
		return
	}
	p.tracker.SetThresholdBroadcaster(fn)
}

// SetBudgetThresholdAlertDispatcher installs the persistent alert-channel
// dispatcher so budget-threshold crossings also fan out to user-configured
// webhook/Slack destinations (spec 004). Safe to call on nil plugin; safe
// to pass nil dispatcher/fetcher.
func (p *GovernancePlugin) SetBudgetThresholdAlertDispatcher(d *alertchannels.Dispatcher, channelsFn func() []tables_enterprise.TableAlertChannel) {
	if p == nil || p.tracker == nil {
		return
	}
	p.tracker.SetAlertDispatcher(d, channelsFn)
}

// SetAdaptiveHealth installs the adaptive-routing health lookup onto
// the internal routing engine so selectAdaptiveTarget can down-weight
// unhealthy targets (spec 013). Safe to call on nil plugin or with
// a nil fn (disables adaptive weighting).
func (p *GovernancePlugin) SetAdaptiveHealth(fn HealthFn) {
	if p == nil || p.engine == nil {
		return
	}
	p.engine.SetAdaptiveHealth(fn)
}
