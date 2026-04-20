// Package guardrailsruntime is the enforcement half of spec 010.
// Spec 010 shipped the config surface (providers + rules). This
// plugin reads those tables on boot and evaluates each enabled rule
// against inference input/output, applying block / flag / log
// semantics per spec 016.
package guardrailsruntime

import (
	"context"
	"fmt"
	"sync"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"gorm.io/gorm"
)

// PluginName is the stable name used by plugin loading.
const PluginName = "guardrails-runtime"

// BifrostContextKeyGuardrailFlags is the context value set on flag
// actions. Consumers (audit, logging) can pull it off the request
// context to record which rules fired without blocking the request.
const BifrostContextKeyGuardrailFlags schemas.BifrostContextKey = "bifrost-guardrail-flags"

// Plugin evaluates guardrail rules on every inference request.
type Plugin struct {
	store  configstore.ConfigStore
	logger schemas.Logger
	db     *gorm.DB

	mu    sync.RWMutex
	rules []ruleEntry
	emit  auditEmitter // nil → use default audit.Emit
}

// Init constructs the plugin and loads the initial rule set.
// db is accepted for symmetry with other plugins but unused today —
// the store interface is enough.
func Init(ctx context.Context, store configstore.ConfigStore, db *gorm.DB, logger schemas.Logger) (*Plugin, error) {
	if store == nil {
		return nil, fmt.Errorf("guardrails-runtime: nil configstore")
	}
	p := &Plugin{store: store, logger: logger, db: db}
	if err := p.Reload(ctx); err != nil && logger != nil {
		// Non-fatal: an empty rule set is a valid "deny nothing" mode.
		logger.Warn(fmt.Sprintf("guardrails-runtime: initial load failed: %v", err))
	}
	return p, nil
}

// GetName satisfies BasePlugin.
func (p *Plugin) GetName() string { return PluginName }

// Cleanup releases plugin resources. Nothing dynamic to tear down.
func (p *Plugin) Cleanup() error { return nil }

// Reload rebuilds the rule cache from storage. Called on boot and by
// the admin handlers after a successful CRUD mutation (spec 016 FR-007).
func (p *Plugin) Reload(ctx context.Context) error {
	providers, err := p.store.ListGuardrailProviders(ctx)
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}
	rules, err := p.store.ListGuardrailRules(ctx)
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}
	loaded := buildRuleIndex(providers, rules, p.logger)
	p.mu.Lock()
	p.rules = loaded
	p.mu.Unlock()
	return nil
}

// RuleCount is exposed for test / status checks.
func (p *Plugin) RuleCount() int {
	if p == nil {
		return 0
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.rules)
}

// Invalidate re-runs Reload with a background context. Intended for
// the guardrails admin handlers: every successful rule / provider
// CRUD mutation calls this so edits take effect immediately (spec 016
// FR-007). Swallows reload errors (logged) — a transient DB blip
// shouldn't fail the admin API response; the next periodic reload
// (if wired) will recover.
func (p *Plugin) Invalidate() {
	if p == nil {
		return
	}
	if err := p.Reload(context.Background()); err != nil && p.logger != nil {
		p.logger.Warn(fmt.Sprintf("guardrails-runtime: invalidate reload: %v", err))
	}
}
