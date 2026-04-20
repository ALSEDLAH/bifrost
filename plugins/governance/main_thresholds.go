// Budget threshold alerts — GovernancePlugin surface (US8, T055).
//
// Sibling-file extension per Constitution Principle XI rule 1. Adds one
// exported method so the HTTP transport can wire the WebSocket
// broadcaster at boot without touching upstream main.go.

package governance

import "github.com/maximhq/bifrost/core/schemas"

// SetBudgetThresholdBroadcaster installs the WebSocket event
// broadcaster used to push budget.threshold.crossed events to the UI.
// Safe to call on nil plugin; safe to pass nil broadcaster.
func (p *GovernancePlugin) SetBudgetThresholdBroadcaster(fn schemas.EventBroadcaster) {
	if p == nil || p.tracker == nil {
		return
	}
	p.tracker.SetThresholdBroadcaster(fn)
}
