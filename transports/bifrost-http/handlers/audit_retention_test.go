// Spec 027 tests: retention config CRUD, prune semantics including
// the HMAC chain protection, and background-tick spacing.

package handlers

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

// newRetentionTestStore: same SQLite store the SCIM/SSO tests use,
// plus the enterprise logstore migrations so ent_audit_entries exists.
func newRetentionTestStore(t *testing.T) (configstore.ConfigStore, *gorm.DB) {
	t.Helper()
	dir := t.TempDir()
	store, err := configstore.NewConfigStore(context.Background(), &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: filepath.Join(dir, "cs.db")},
	}, &mockLogger{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	require.NoError(t, configstore.RegisterEnterpriseMigrations(context.Background(), store.DB()))
	// Audit entries table lives in the logstore migrations.
	require.NoError(t, logstore.RegisterEnterpriseMigrations(context.Background(), store.DB()))
	return store, store.DB()
}

// seedAuditFull creates an audit row with the given created_at and HMAC chain bits.
func seedAuditFull(t *testing.T, db *gorm.DB, id string, createdAt time.Time, hmac, prevHMAC string) {
	t.Helper()
	row := &logstore.TableAuditEntry{
		ID: id, OrganizationID: "org-x",
		ActorType: "user", ActorID: "u-1",
		Action: "test.action", ResourceType: "test", ResourceID: "r-1",
		Outcome: "allowed",
		CreatedAt: createdAt,
		HMAC: hmac, PrevHMAC: prevHMAC,
	}
	require.NoError(t, db.Create(row).Error)
}

func newRetentionHandler(t *testing.T) (*AuditRetentionHandler, *gorm.DB) {
	t.Helper()
	store, db := newRetentionTestStore(t)
	h := NewAuditRetentionHandler(db, db, &mockLogger{})
	t.Cleanup(h.Close)
	_ = store
	return h, db
}

func TestRetention_GetConfig_DefaultsWhenMissing(t *testing.T) {
	h, _ := newRetentionHandler(t)
	ctx := &fasthttp.RequestCtx{}
	h.getConfig(ctx)
	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("status %d", ctx.Response.StatusCode())
	}
	var out tables_enterprise.TableAuditRetentionConfig
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &out))
	if out.Enabled {
		t.Errorf("default enabled should be false; got true")
	}
	if out.RetentionDays != 90 {
		t.Errorf("default retention_days should be 90; got %d", out.RetentionDays)
	}
}

func TestRetention_PutConfig_PersistsAndRoundTrips(t *testing.T) {
	h, _ := newRetentionHandler(t)
	body, _ := json.Marshal(retentionPutBody{Enabled: true, RetentionDays: 30})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	h.putConfig(ctx)
	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("PUT status %d body %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}

	getCtx := &fasthttp.RequestCtx{}
	h.getConfig(getCtx)
	var out tables_enterprise.TableAuditRetentionConfig
	require.NoError(t, json.Unmarshal(getCtx.Response.Body(), &out))
	if !out.Enabled || out.RetentionDays != 30 {
		t.Errorf("config didn't roundtrip: %+v", out)
	}
}

func TestRetention_PutConfig_ZeroDays_400(t *testing.T) {
	h, _ := newRetentionHandler(t)
	body, _ := json.Marshal(retentionPutBody{Enabled: true, RetentionDays: 0})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	h.putConfig(ctx)
	if ctx.Response.StatusCode() != 400 {
		t.Errorf("expected 400 for zero retention_days; got %d", ctx.Response.StatusCode())
	}
}

func TestRetention_Prune_DisabledReturns409(t *testing.T) {
	h, _ := newRetentionHandler(t)
	// don't set config — defaults to enabled=false
	ctx := &fasthttp.RequestCtx{}
	h.runPrune(ctx)
	if ctx.Response.StatusCode() != 409 {
		t.Errorf("disabled prune should 409; got %d", ctx.Response.StatusCode())
	}
}

func TestRetention_Prune_DeletesOldRows(t *testing.T) {
	h, db := newRetentionHandler(t)
	// Enable with 30-day retention.
	body, _ := json.Marshal(retentionPutBody{Enabled: true, RetentionDays: 30})
	put := &fasthttp.RequestCtx{}
	put.Request.SetBody(body)
	h.putConfig(put)
	require.Equal(t, 200, put.Response.StatusCode())

	// Seed: 3 old rows (60+ days), 2 fresh (yesterday) — no chain.
	old := time.Now().Add(-65 * 24 * time.Hour)
	fresh := time.Now().Add(-24 * time.Hour)
	for i, ts := range []time.Time{old, old, old, fresh, fresh} {
		seedAuditFull(t, db, "id-"+string(rune('a'+i)), ts, "", "")
	}

	prune := &fasthttp.RequestCtx{}
	h.runPrune(prune)
	if prune.Response.StatusCode() != 200 {
		t.Fatalf("prune status %d body %s", prune.Response.StatusCode(), prune.Response.Body())
	}
	var res pruneResult
	require.NoError(t, json.Unmarshal(prune.Response.Body(), &res))
	if res.Deleted != 3 {
		t.Errorf("expected 3 deleted; got %d", res.Deleted)
	}
	if res.Refused != "" {
		t.Errorf("unexpected refused: %q", res.Refused)
	}
	var remaining int64
	db.Model(&logstore.TableAuditEntry{}).Count(&remaining)
	if remaining != 2 {
		t.Errorf("expected 2 rows left; got %d", remaining)
	}
}

func TestRetention_Prune_BlocksOnHMACChain(t *testing.T) {
	h, db := newRetentionHandler(t)
	body, _ := json.Marshal(retentionPutBody{Enabled: true, RetentionDays: 30})
	put := &fasthttp.RequestCtx{}
	put.Request.SetBody(body)
	h.putConfig(put)

	// Old row carries hmac=A; fresh row was chained from it (prev=A).
	// Pruning the old row would orphan the chain.
	old := time.Now().Add(-65 * 24 * time.Hour)
	fresh := time.Now().Add(-24 * time.Hour)
	seedAuditFull(t, db, "old-1", old, "A", "")
	seedAuditFull(t, db, "new-1", fresh, "B", "A")

	prune := &fasthttp.RequestCtx{}
	h.runPrune(prune)
	var res pruneResult
	require.NoError(t, json.Unmarshal(prune.Response.Body(), &res))
	if res.Deleted != 0 {
		t.Errorf("should not delete when chain would break; got deleted=%d", res.Deleted)
	}
	if res.Refused == "" {
		t.Errorf("expected refused message; got %+v", res)
	}
}

func TestRetention_Prune_ForceOverridesChain(t *testing.T) {
	h, db := newRetentionHandler(t)
	body, _ := json.Marshal(retentionPutBody{Enabled: true, RetentionDays: 30})
	put := &fasthttp.RequestCtx{}
	put.Request.SetBody(body)
	h.putConfig(put)

	old := time.Now().Add(-65 * 24 * time.Hour)
	fresh := time.Now().Add(-24 * time.Hour)
	seedAuditFull(t, db, "old-1", old, "A", "")
	seedAuditFull(t, db, "new-1", fresh, "B", "A")

	pbody, _ := json.Marshal(prunePostBody{Force: true})
	prune := &fasthttp.RequestCtx{}
	prune.Request.SetBody(pbody)
	h.runPrune(prune)
	var res pruneResult
	require.NoError(t, json.Unmarshal(prune.Response.Body(), &res))
	if res.Deleted != 1 {
		t.Errorf("force prune should delete 1; got %d (refused=%q)", res.Deleted, res.Refused)
	}
}

func TestRetention_Prune_StampsLastPrunedAt(t *testing.T) {
	h, db := newRetentionHandler(t)
	body, _ := json.Marshal(retentionPutBody{Enabled: true, RetentionDays: 30})
	put := &fasthttp.RequestCtx{}
	put.Request.SetBody(body)
	h.putConfig(put)
	seedAuditFull(t, db, "x", time.Now().Add(-65*24*time.Hour), "", "")

	before := time.Now().UTC()
	prune := &fasthttp.RequestCtx{}
	h.runPrune(prune)

	cfg, err := h.loadConfig()
	require.NoError(t, err)
	if cfg.LastPrunedAt == nil {
		t.Fatalf("last_pruned_at should be stamped")
	}
	if cfg.LastPrunedAt.Before(before.Add(-time.Second)) {
		t.Errorf("last_pruned_at suspiciously old: %v vs before %v", cfg.LastPrunedAt, before)
	}
}

func TestRetention_NoOldRows_ZeroDeleted(t *testing.T) {
	h, db := newRetentionHandler(t)
	body, _ := json.Marshal(retentionPutBody{Enabled: true, RetentionDays: 30})
	put := &fasthttp.RequestCtx{}
	put.Request.SetBody(body)
	h.putConfig(put)
	seedAuditFull(t, db, "fresh", time.Now().Add(-time.Hour), "", "")

	prune := &fasthttp.RequestCtx{}
	h.runPrune(prune)
	var res pruneResult
	require.NoError(t, json.Unmarshal(prune.Response.Body(), &res))
	if res.Deleted != 0 || res.Refused != "" {
		t.Errorf("nothing-to-prune should return deleted=0 refused=''; got %+v", res)
	}
}

func TestRetention_BackgroundTick_RespectsSpacing(t *testing.T) {
	// Don't actually wait 6h. Instead invoke maybePrune manually with
	// last_pruned_at set to 'just now' and verify no rows are touched.
	h, db := newRetentionHandler(t)
	body, _ := json.Marshal(retentionPutBody{Enabled: true, RetentionDays: 30})
	put := &fasthttp.RequestCtx{}
	put.Request.SetBody(body)
	h.putConfig(put)
	seedAuditFull(t, db, "old-1", time.Now().Add(-65*24*time.Hour), "", "")

	now := time.Now().UTC()
	require.NoError(t, h.configDB.Model(&tables_enterprise.TableAuditRetentionConfig{}).
		Where("id = ?", tables_enterprise.AuditRetentionSingletonID).
		Update("last_pruned_at", &now).Error)

	h.maybePrune()
	var cnt int64
	db.Model(&logstore.TableAuditEntry{}).Count(&cnt)
	if cnt != 1 {
		t.Errorf("background prune should be skipped within 24h spacing; got %d rows left", cnt)
	}
}

func TestRetention_Close_Idempotent(t *testing.T) {
	h, _ := newRetentionHandler(t)
	h.Close()
	h.Close() // second call must not panic / double-close
}
