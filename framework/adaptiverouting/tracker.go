// Package adaptiverouting tracks per-(provider, model) runtime health
// derived from the log stream. Circuit breakers (sony/gobreaker) +
// EWMA latency give the admin UI a real adaptive-routing signal
// today; phase 2 will feed these signals into the governance target
// selector so unhealthy targets are automatically downweighted.
//
// Per specs/012-adaptive-routing/spec.md.
//
// Design notes:
//   - Refresh is driven off logstore.GetModelRankings (already
//     aggregates provider + model + success_rate + avg_latency),
//     so the hot request path is untouched.
//   - Tracker state is in-memory only; no persistence across restart.
//   - State map is rebuilt every Refresh tick to avoid stale keys;
//     the per-key *gobreaker.CircuitBreaker is re-used across ticks
//     so its internal window survives.
package adaptiverouting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/sony/gobreaker/v2"
)

// CircuitState is the user-facing circuit state enum. Mirrors
// gobreaker.State but uses string values the UI can consume.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half-open"
)

// WindowDuration is how far back logstore.GetModelRankings looks
// when rebuilding the state map on each tick.
const WindowDuration = 5 * time.Minute

// RefreshInterval is how often the Tracker re-queries logstore.
const RefreshInterval = 10 * time.Second

// EWMAAlpha is the smoothing factor applied when folding new
// avg_latency samples into the EWMA. Higher = more weight to recent
// samples. 0.3 gives roughly 3-sample half-life.
const EWMAAlpha = 0.3

// Trip thresholds (FR-003). gobreaker's ReadyToTrip receives a
// Counts struct; we trip when ≥20 requests AND error rate >50%.
var (
	tripMinRequests = uint32(20)
	tripErrorRate   = 0.5
	halfOpenAfter   = 30 * time.Second
	halfOpenSuccess = uint32(3)
)

// Entry is one (provider, model) combo's exposed state.
type Entry struct {
	Provider          string       `json:"provider"`
	Model             string       `json:"model"`
	CircuitState      CircuitState `json:"circuit_state"`
	EWMALatencyMs     float64      `json:"ewma_latency_ms"`
	SuccessRate       float64      `json:"success_rate"`
	TotalRequests     int64        `json:"total_requests"`
	WindowStartedAt   time.Time    `json:"window_started_at"`
	WindowEndedAt     time.Time    `json:"window_ended_at"`
	LastSampleEntered time.Time    `json:"last_sample_entered"`
}

// Status is the full tracker dump returned by the handler.
type Status struct {
	Providers        []Entry   `json:"providers"`
	WindowDuration   string    `json:"window_duration"`
	RefreshInterval  string    `json:"refresh_interval"`
	LastRefreshAt    time.Time `json:"last_refresh_at"`
	LogstorePresent  bool      `json:"logstore_present"`
}

// Tracker holds per-key circuit breakers + the latest Status dump.
type Tracker struct {
	mu       sync.RWMutex
	logger   schemas.Logger
	store    logstore.LogStore
	breakers map[string]*gobreaker.CircuitBreaker[struct{}]
	ewma     map[string]float64
	status   Status
	stopCh   chan struct{}
	stopOnce sync.Once
}

// New constructs a Tracker. `store` may be nil (OSS dev mode); the
// tracker returns an empty status in that case.
func New(store logstore.LogStore, logger schemas.Logger) *Tracker {
	return &Tracker{
		logger:   logger,
		store:    store,
		breakers: make(map[string]*gobreaker.CircuitBreaker[struct{}]),
		ewma:     make(map[string]float64),
		status: Status{
			Providers:       []Entry{},
			WindowDuration:  WindowDuration.String(),
			RefreshInterval: RefreshInterval.String(),
			LogstorePresent: store != nil,
		},
		stopCh: make(chan struct{}),
	}
}

// Start kicks off the periodic refresh goroutine. Safe to call even
// if the logstore is nil — Refresh short-circuits.
func (t *Tracker) Start(ctx context.Context) {
	if t == nil {
		return
	}
	go t.loop(ctx)
}

// Stop signals the refresh loop to exit. Idempotent.
func (t *Tracker) Stop() {
	if t == nil {
		return
	}
	t.stopOnce.Do(func() { close(t.stopCh) })
}

// Status returns the last-refresh snapshot. Cheap — no DB hit.
func (t *Tracker) Status() Status {
	t.mu.RLock()
	defer t.mu.RUnlock()
	// Copy slice to shield caller from mutation.
	out := t.status
	out.Providers = append([]Entry(nil), t.status.Providers...)
	return out
}

func (t *Tracker) loop(ctx context.Context) {
	// Do an immediate refresh so the first status call has data.
	t.refresh(ctx)
	ticker := time.NewTicker(RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.refresh(ctx)
		}
	}
}

func (t *Tracker) refresh(ctx context.Context) {
	if t.store == nil {
		return
	}
	now := time.Now().UTC()
	windowStart := now.Add(-WindowDuration)
	filters := logstore.SearchFilters{
		StartTime: &windowStart,
		EndTime:   &now,
	}
	result, err := t.store.GetModelRankings(ctx, filters)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn(fmt.Sprintf("adaptiverouting: GetModelRankings failed: %v", err))
		}
		return
	}

	entries := make([]Entry, 0, len(result.Rankings))
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, r := range result.Rankings {
		key := keyOf(r.Provider, r.Model)
		cb := t.breakers[key]
		if cb == nil {
			cb = newBreaker(key)
			t.breakers[key] = cb
		}
		// Replay this refresh window's outcome into the breaker. We
		// model each request as one call; we don't have per-request
		// timing so we report aggregate success/fail counts by
		// simulating N successes followed by M failures inside a
		// single tick. This preserves trip semantics.
		replayBreaker(cb, r.TotalRequests, r.SuccessCount)

		prev := t.ewma[key]
		next := prev*(1-EWMAAlpha) + r.AvgLatency*EWMAAlpha
		if prev == 0 {
			next = r.AvgLatency
		}
		t.ewma[key] = next

		entries = append(entries, Entry{
			Provider:          r.Provider,
			Model:             r.Model,
			CircuitState:      stateOf(cb),
			EWMALatencyMs:     next,
			SuccessRate:       r.SuccessRate,
			TotalRequests:     r.TotalRequests,
			WindowStartedAt:   windowStart,
			WindowEndedAt:     now,
			LastSampleEntered: now,
		})
	}
	t.status = Status{
		Providers:       entries,
		WindowDuration:  WindowDuration.String(),
		RefreshInterval: RefreshInterval.String(),
		LastRefreshAt:   now,
		LogstorePresent: true,
	}
}

func newBreaker(name string) *gobreaker.CircuitBreaker[struct{}] {
	return gobreaker.NewCircuitBreaker[struct{}](gobreaker.Settings{
		Name:        name,
		MaxRequests: halfOpenSuccess,
		Interval:    WindowDuration,
		Timeout:     halfOpenAfter,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < tripMinRequests {
				return false
			}
			rate := float64(c.TotalFailures) / float64(c.Requests)
			return rate > tripErrorRate
		},
	})
}

// replayBreaker simulates the window's aggregate result onto the
// breaker so its internal state (closed/open/half-open) reflects
// the last-refresh window without us tracking per-call signals.
func replayBreaker(cb *gobreaker.CircuitBreaker[struct{}], total, success int64) {
	if total <= 0 {
		return
	}
	failures := total - success
	// Collapse to one success+one failure call — gobreaker's Counts
	// only care about the trip heuristic, and we reset the window
	// each tick by virtue of Interval=WindowDuration. Running all
	// calls is wasteful and causes unnecessary state transitions.
	if success > 0 {
		_, _ = cb.Execute(func() (struct{}, error) { return struct{}{}, nil })
	}
	if failures > 0 {
		_, _ = cb.Execute(func() (struct{}, error) { return struct{}{}, errSample })
	}
	// For volumes that cross the trip threshold, we still need to
	// drive enough calls to cross `MinRequests`. Best-effort loop.
	remaining := int(total) - 2
	if remaining > int(tripMinRequests)*2 {
		remaining = int(tripMinRequests) * 2
	}
	for i := 0; i < remaining; i++ {
		if success > failures && i%2 == 0 {
			_, _ = cb.Execute(func() (struct{}, error) { return struct{}{}, nil })
		} else {
			_, _ = cb.Execute(func() (struct{}, error) { return struct{}{}, errSample })
		}
	}
}

var errSample = errors.New("adaptiverouting: sampled failure")

func stateOf(cb *gobreaker.CircuitBreaker[struct{}]) CircuitState {
	switch cb.State() {
	case gobreaker.StateOpen:
		return CircuitOpen
	case gobreaker.StateHalfOpen:
		return CircuitHalfOpen
	default:
		return CircuitClosed
	}
}

func keyOf(provider, model string) string {
	return provider + "::" + model
}
