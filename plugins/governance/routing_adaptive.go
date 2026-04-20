// Adaptive target selection (spec 013).
//
// Wraps the static selectWeightedTarget with a health-factor lookup so
// circuit-open / half-open providers are skipped or down-weighted
// automatically. Sibling-file extension per Constitution Principle XI
// rule 1 — no edits to selectWeightedTarget itself.

package governance

import (
	"fmt"

	"github.com/maximhq/bifrost/core/schemas"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
)

// selectAdaptiveTarget picks one target, consulting healthFn when
// available. Effective weight = static_weight × health_factor. Falls
// back to static selection when every candidate's effective weight is
// zero so requests are never silently dropped by the gateway (spec 013
// FR-004).
func selectAdaptiveTarget(
	targets []configstoreTables.TableRoutingTarget,
	healthFn HealthFn,
	ctx *schemas.BifrostContext,
	logger schemas.Logger,
	ruleName string,
) (configstoreTables.TableRoutingTarget, bool) {
	if healthFn == nil {
		return selectWeightedTarget(targets)
	}

	// Build a parallel slice of targets with weight = static × health.
	adjusted := make([]configstoreTables.TableRoutingTarget, 0, len(targets))
	anyHealthy := false
	for _, t := range targets {
		if t.Weight < 0 {
			continue
		}
		prov, model := "", ""
		if t.Provider != nil {
			prov = *t.Provider
		}
		if t.Model != nil {
			model = *t.Model
		}
		// If a target doesn't pin a (provider, model) — e.g. a
		// chained or name-only target — treat it as healthy. Phase 2
		// only acts when we have a concrete key to look up.
		h := 1.0
		if prov != "" && model != "" {
			h = healthFn(prov, model)
		}
		if h < 0 {
			h = 0
		}
		if h > 1 {
			h = 1
		}
		eff := t.Weight * h
		if eff > 0 {
			anyHealthy = true
		}
		if ctx != nil {
			ctx.AppendRoutingEngineLog(
				schemas.RoutingEngineRoutingRule,
				fmt.Sprintf("Rule '%s' target %s/%s adaptive_weight=%.3f (static=%.3f, health=%.2f)", ruleName, prov, model, eff, t.Weight, h),
			)
		}
		adj := t
		adj.Weight = eff
		adjusted = append(adjusted, adj)
	}

	// Fail-safe: if every candidate is circuit-open (eff == 0), fall
	// back to the original weighted selection rather than dropping
	// the request. The upstream error will surface through the
	// usual provider path — the gateway itself does not withhold.
	if !anyHealthy {
		if ctx != nil {
			ctx.AppendRoutingEngineLog(
				schemas.RoutingEngineRoutingRule,
				fmt.Sprintf("Rule '%s' all targets unhealthy — falling back to static weighted selection", ruleName),
			)
		}
		if logger != nil {
			logger.Warn(fmt.Sprintf("[RoutingEngine] Rule %s: all targets unhealthy, using static fallback", ruleName))
		}
		return selectWeightedTarget(targets)
	}

	return selectWeightedTarget(adjusted)
}
