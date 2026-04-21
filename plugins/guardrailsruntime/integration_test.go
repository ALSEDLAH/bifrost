// Integration test — exercises the full plugin path end-to-end with
// an in-memory SQLite configstore + real audit sink, driving it via
// the actual LLMPlugin interface (spec 016 T014).
//
// This catches regressions that unit tests miss: table index build
// over real providers + rules, PreLLMHook error-propagation, and
// audit writes via audit.Init → audit.Emit through the HMAC chain
// (spec 015 integration).

package guardrailsruntime

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
)

// tenantedCtx returns a BifrostContext pre-populated with a
// synthetic tenant so audit.Emit can resolve the actor without
// real middleware running.
func tenantedCtx() *schemas.BifrostContext {
	bctx := &schemas.BifrostContext{}
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, "test-org")
	bctx.SetValue(tenancy.BifrostContextKeyUserID, "test-user")
	bctx.SetValue(tenancy.BifrostContextKeyResolvedVia, tenancy.Resolver("test"))
	return bctx
}

// newSQLiteStoresForIntegration boots fresh SQLite configstore +
// logstore schemas keyed by a per-test temp file. Returns the
// configstore plus the underlying *gorm.DB needed for audit.Init.
func newSQLiteStoresForIntegration(t *testing.T) configstore.ConfigStore {
	t.Helper()
	dir := t.TempDir()
	cfg := &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: filepath.Join(dir, "cs.db")},
	}
	store, err := configstore.NewConfigStore(context.Background(), cfg, testLogger{})
	if err != nil {
		t.Fatalf("configstore init: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(context.Background()) })

	// Migrate the enterprise schema (includes guardrail tables and
	// audit_entries).
	if err := configstore.RegisterEnterpriseMigrations(context.Background(), store.DB()); err != nil {
		t.Fatalf("enterprise migrations: %v", err)
	}
	if err := logstore.RegisterEnterpriseMigrations(context.Background(), store.DB()); err != nil {
		t.Fatalf("logstore enterprise migrations: %v", err)
	}
	return store
}

func TestIntegration_RegexBlock_WritesAuditRow(t *testing.T) {
	store := newSQLiteStoresForIntegration(t)
	ctx := context.Background()

	// Seed one block rule.
	rule := &tables_enterprise.TableGuardrailRule{
		Name: "ccn-int", Trigger: "input", Action: "block",
		Pattern: ccnPattern, Enabled: true,
	}
	if err := store.CreateGuardrailRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// Stand up a live audit plugin so Emit() writes to the same DB.
	auditPlugin, err := audit.Init(ctx, store.DB(), testLogger{}, audit.Config{})
	if err != nil {
		t.Fatalf("audit init: %v", err)
	}
	t.Cleanup(func() { _ = auditPlugin.Cleanup() })

	p, err := Init(ctx, store, store.DB(), testLogger{})
	if err != nil {
		t.Fatalf("plugin init: %v", err)
	}
	if p.RuleCount() != 1 {
		t.Fatalf("expected 1 rule loaded; got %d", p.RuleCount())
	}

	// Drive a blocked request with a tenanted context so audit.Emit
	// can resolve the actor.
	_, sc, err := p.PreLLMHook(tenantedCtx(), chatReq("card 4111111111111111 please"))
	if err != nil {
		t.Fatalf("hook err: %v", err)
	}
	if sc == nil || sc.Error == nil {
		t.Fatalf("expected short-circuit block; got %+v", sc)
	}

	// Confirm an audit row landed and flows through the HMAC chain
	// setup in spec 015 (HMAC stays empty when no key env var is set
	// — just asserting the row itself arrived).
	var rows []logstore.TableAuditEntry
	if err := store.DB().Model(&logstore.TableAuditEntry{}).
		Where("action = ?", "guardrail.block").
		Find(&rows).Error; err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 guardrail.block audit row; got %d", len(rows))
	}
	if rows[0].ResourceType != "guardrail_rule" || rows[0].Outcome != "denied" {
		t.Errorf("audit row metadata wrong: %+v", rows[0])
	}
}

func TestIntegration_InvalidateReloadsRules(t *testing.T) {
	store := newSQLiteStoresForIntegration(t)
	ctx := context.Background()

	// Must init audit sink before the plugin — Emit() calls it on
	// evaluation; missing sink logs at Debug and proceeds, so this
	// test still works without it, but the real server order is this.
	auditPlugin, _ := audit.Init(ctx, store.DB(), testLogger{}, audit.Config{})
	if auditPlugin != nil {
		t.Cleanup(func() { _ = auditPlugin.Cleanup() })
	}

	p, err := Init(ctx, store, store.DB(), testLogger{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if p.RuleCount() != 0 {
		t.Fatalf("empty DB should load 0 rules; got %d", p.RuleCount())
	}

	// Add a rule out-of-band, then invalidate.
	if err := store.CreateGuardrailRule(ctx, &tables_enterprise.TableGuardrailRule{
		Name: "late", Trigger: "input", Action: "log",
		Pattern: "secret", Enabled: true,
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	p.Invalidate()
	if p.RuleCount() != 1 {
		t.Errorf("Invalidate() should pick up the new rule; got %d", p.RuleCount())
	}
}
