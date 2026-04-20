// Budget threshold alerts (US8, T055).
//
// Sibling-file extension of UsageTracker per Constitution Principle XI
// rule 1 — no edits to upstream tracker.go other than a single hook
// call. Detects 50 / 75 / 90 % crossings on any budget affected by a
// virtual-key usage update and emits:
//
//   - logger.Warn           — ops visibility
//   - EventBroadcaster(…)   — WebSocket push to connected UI clients
//                             (opt-in; no-op if not set)
//
// Audit-log emission is intentionally out of scope for v1 because
// audit.Emit requires a resolved tenant context and the governance
// tracker's UpdateUsage path doesn't carry one. The structured log
// line covers the compliance trail until a durable record is wired.
//
// Dedup: one emit per (budget_id, reset_cycle_start) per threshold
// level. The cycle_start is budget.LastReset.UnixNano() so a reset
// automatically invalidates the cached dedup key and the new cycle
// starts fresh.

package governance

import (
	"context"
	"fmt"
	"sync"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/alertchannels"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	alertchannelsTables "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

// budgetThresholdLevels is the ordered list of thresholds to alert on.
// Crossings are detected from oldUsage < threshold to newUsage >= threshold.
var budgetThresholdLevels = []int{50, 75, 90}

// budgetThresholdKey uniquely identifies a (budget, reset cycle, level)
// for dedup. budget reset → new LastReset → new key → fresh emits.
type budgetThresholdKey struct {
	budgetID   string
	cycleStart int64 // budget.LastReset.UnixNano()
	level      int
}

// thresholdState holds the runtime state for threshold emission. Kept
// in one place so the tracker zero-value remains valid.
type thresholdState struct {
	emitted     sync.Map // map[budgetThresholdKey]struct{}
	mu          sync.RWMutex
	broadcaster schemas.EventBroadcaster
	// Alert-channel dispatch — wired by server startup (spec 004).
	// dispatcher sends events to configured webhook/slack channels.
	// channels is called every emission so updates take effect without
	// a restart; returning nil / empty slice is a no-op.
	dispatcher *alertchannels.Dispatcher
	channels   func() []alertchannelsTables.TableAlertChannel
}

// thresholdStateStore is a process-level holder indexed by tracker
// pointer. Uses a map so the UsageTracker struct layout is untouched
// (sibling-file rule: no field additions to upstream structs).
var (
	thresholdStatesMu sync.RWMutex
	thresholdStates   = make(map[*UsageTracker]*thresholdState)
)

func (t *UsageTracker) thresholdState() *thresholdState {
	thresholdStatesMu.RLock()
	s, ok := thresholdStates[t]
	thresholdStatesMu.RUnlock()
	if ok {
		return s
	}
	thresholdStatesMu.Lock()
	defer thresholdStatesMu.Unlock()
	if s, ok = thresholdStates[t]; ok {
		return s
	}
	s = &thresholdState{}
	thresholdStates[t] = s
	return s
}

// SetThresholdBroadcaster installs the WebSocket broadcaster used to
// push budget.threshold.crossed events to connected UI clients. Safe
// to call multiple times; last writer wins. Passing nil disables
// broadcasting without stopping log emission.
func (t *UsageTracker) SetThresholdBroadcaster(fn schemas.EventBroadcaster) {
	if t == nil {
		return
	}
	s := t.thresholdState()
	s.mu.Lock()
	s.broadcaster = fn
	s.mu.Unlock()
}

// SetAlertDispatcher installs the alert-channel dispatcher used to
// fan out threshold crossings to user-configured webhook/Slack
// destinations. `channelsFn` is called on every emission; return nil
// or an empty slice to no-op (spec 004).
func (t *UsageTracker) SetAlertDispatcher(d *alertchannels.Dispatcher, channelsFn func() []alertchannelsTables.TableAlertChannel) {
	if t == nil {
		return
	}
	s := t.thresholdState()
	s.mu.Lock()
	s.dispatcher = d
	s.channels = channelsFn
	s.mu.Unlock()
}

// afterBudgetUpdate is called by UpdateUsage after a successful
// in-memory VK budget update. It walks every budget affected by the
// hierarchy (provider-config → VK → team → customer), computes the
// pre/post percentage assuming the given cost was just added to
// CurrentUsage, and emits for each newly-crossed threshold.
//
// The caller is `governance.UsageTracker.UpdateUsage` — see the
// one-line hook in tracker.go. Safe to call with nil vk (no-op).
func (t *UsageTracker) afterBudgetUpdate(ctx context.Context, vk *configstoreTables.TableVirtualKey, provider schemas.ModelProvider, cost float64) {
	if t == nil || vk == nil || cost <= 0 {
		return
	}
	local, ok := t.store.(*LocalGovernanceStore)
	if !ok {
		// Not the local in-memory store; thresholds only fire on
		// in-process budget mutations (the only writer path).
		return
	}
	budgets, _ := local.collectBudgetsFromHierarchy(vk, provider)
	if len(budgets) == 0 {
		return
	}
	state := t.thresholdState()
	for _, b := range budgets {
		t.checkAndEmit(ctx, state, b, vk, provider, cost)
	}
}

// checkAndEmit computes threshold crossings for one budget and emits
// once per (budget, cycle, level).
func (t *UsageTracker) checkAndEmit(ctx context.Context, state *thresholdState, budget *configstoreTables.TableBudget, vk *configstoreTables.TableVirtualKey, provider schemas.ModelProvider, cost float64) {
	if budget == nil || budget.MaxLimit <= 0 {
		return
	}
	newUsage := budget.CurrentUsage
	oldUsage := newUsage - cost
	if oldUsage < 0 {
		oldUsage = 0
	}
	newPct := (newUsage / budget.MaxLimit) * 100
	oldPct := (oldUsage / budget.MaxLimit) * 100
	cycleStart := budget.LastReset.UnixNano()

	for _, level := range budgetThresholdLevels {
		if !(oldPct < float64(level) && newPct >= float64(level)) {
			continue
		}
		key := budgetThresholdKey{budgetID: budget.ID, cycleStart: cycleStart, level: level}
		if _, loaded := state.emitted.LoadOrStore(key, struct{}{}); loaded {
			continue
		}
		t.emitThreshold(ctx, state, budget, vk, provider, level, newUsage)
	}
}

// emitThreshold performs the actual log + broadcast emission for a
// single crossing.
func (t *UsageTracker) emitThreshold(_ context.Context, state *thresholdState, budget *configstoreTables.TableBudget, vk *configstoreTables.TableVirtualKey, provider schemas.ModelProvider, level int, newUsage float64) {
	vkID := ""
	teamID := ""
	customerID := ""
	if vk != nil {
		vkID = vk.ID
		if vk.TeamID != nil {
			teamID = *vk.TeamID
		}
		if vk.CustomerID != nil {
			customerID = *vk.CustomerID
		}
	}
	t.logger.Warn(fmt.Sprintf(
		"budget.threshold.crossed level=%d budget_id=%s vk_id=%s team_id=%s customer_id=%s provider=%s usage=%.4f max=%.4f",
		level, budget.ID, vkID, teamID, customerID, string(provider), newUsage, budget.MaxLimit,
	))

	payload := map[string]any{
		"level":          level,
		"budget_id":      budget.ID,
		"virtual_key":    vkID,
		"team_id":        teamID,
		"customer_id":    customerID,
		"provider":       string(provider),
		"current_usage":  newUsage,
		"max_limit":      budget.MaxLimit,
		"reset_duration": budget.ResetDuration,
	}

	state.mu.RLock()
	bc := state.broadcaster
	dispatcher := state.dispatcher
	channelsFn := state.channels
	state.mu.RUnlock()

	if bc != nil {
		bc("budget.threshold.crossed", payload)
	}
	if dispatcher != nil && channelsFn != nil {
		dispatcher.Send(channelsFn(), alertchannels.Event{
			Type: "budget.threshold.crossed",
			Data: payload,
		})
	}
}
