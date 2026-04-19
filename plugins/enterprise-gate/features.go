// Enterprise feature manifest — single source of truth consulted by:
//   1. The observability-completeness CI check (scripts/check-obs-completeness.sh)
//      to enumerate which plugins must emit OTEL + Prometheus + audit.
//   2. The enterprise-gate plugin itself, to register middleware
//      providers in the correct order.
//   3. The license plugin's IsEntitled() surface — feature names here
//      match the entitlement claims in the JWT license file.
//
// Additions to enterprise feature surface MUST be reflected here.
// Constitution Principle VI + research R-09.

package enterprisegate

// Feature is a single enterprise feature's manifest entry.
type Feature struct {
	// Name is the canonical feature identifier. Used by the license
	// plugin's IsEntitled() check and by audit log entries.
	Name string

	// PluginDir is the path under plugins/ that implements the feature.
	// The obs-completeness CI scans this directory for OTEL/Prom/audit
	// emit calls.
	PluginDir string

	// UserStory is the spec.md user-story tag (e.g. "US1", "US24").
	UserStory string

	// Train is one of "A", "B", "C", "D", "E" per plan.md.
	Train string
}

// Features is the authoritative list. Order is the load order; earlier
// entries are loaded by the plugin manager first. Foundational plugins
// (license, enterprise-gate, audit) come first; feature plugins follow
// in train order; cloud-only Train E plugins come last.
var Features = []Feature{
	// Foundational (loaded in every enterprise build).
	{Name: "license", PluginDir: "plugins/license", UserStory: "US24", Train: "E"},
	{Name: "audit", PluginDir: "plugins/audit", UserStory: "US4", Train: "A"},
	{Name: "enterprise-gate", PluginDir: "plugins/enterprise-gate", UserStory: "US1", Train: "A"},

	// Train A — Tenancy + Identity (P1)
	// Note: orgs/workspaces/RBAC are served by enterprise-gate +
	// transports/handlers, no dedicated plugin.
	{Name: "sso", PluginDir: "plugins/sso", UserStory: "US3", Train: "A"},

	// Train B — Governance Depth (P2)
	{Name: "guardrails-central", PluginDir: "plugins/guardrails-central", UserStory: "US6", Train: "B"},
	{Name: "guardrails-partners", PluginDir: "plugins/guardrails-partners", UserStory: "US6", Train: "B"},
	{Name: "guardrails-webhook", PluginDir: "plugins/guardrails-webhook", UserStory: "US9", Train: "B"},
	{Name: "pii-redactor", PluginDir: "plugins/pii-redactor", UserStory: "US7", Train: "B"},

	// Train C — Observability + DX (P3 + P4)
	{Name: "alerts", PluginDir: "plugins/alerts", UserStory: "US10", Train: "C"},
	{Name: "logexport", PluginDir: "plugins/logexport", UserStory: "US11", Train: "C"},
	{Name: "canary", PluginDir: "plugins/canary", UserStory: "US16", Train: "C"},

	// Train D — Security + Ecosystem (P5 + P6)
	{Name: "byok", PluginDir: "plugins/byok", UserStory: "US18", Train: "D"},

	// Train E — Cloud Commercial (cloud mode only)
	{Name: "metering", PluginDir: "plugins/metering", UserStory: "US26", Train: "E"},
	{Name: "billing", PluginDir: "plugins/billing", UserStory: "US27", Train: "E"},
}

// FeatureNames returns just the canonical names — convenient for
// IsEntitled-style lookups.
func FeatureNames() []string {
	out := make([]string, 0, len(Features))
	for _, f := range Features {
		out = append(out, f.Name)
	}
	return out
}

// FeaturesByTrain partitions Features by train letter.
func FeaturesByTrain(train string) []Feature {
	out := make([]Feature, 0, 4)
	for _, f := range Features {
		if f.Train == train {
			out = append(out, f)
		}
	}
	return out
}
