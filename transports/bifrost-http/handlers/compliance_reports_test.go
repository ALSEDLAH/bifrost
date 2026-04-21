// Unit tests for the compliance reports handler (spec 019).

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func newReportsTestStore(t *testing.T) configstore.ConfigStore {
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
	require.NoError(t, logstore.RegisterEnterpriseMigrations(context.Background(), store.DB()))
	return store
}

func seedAudit(t *testing.T, store configstore.ConfigStore, action, outcome string, offsetDays int) {
	t.Helper()
	row := logstore.TableAuditEntry{
		ID:           uuid.NewString(),
		Action:       action,
		ResourceType: "test",
		Outcome:      outcome,
		CreatedAt:    time.Now().UTC().Add(-time.Duration(offsetDays) * 24 * time.Hour),
	}
	require.NoError(t, store.DB().Create(&row).Error)
}

// invokeReport calls the handler directly (no router / middleware
// chain) so the test doesn't depend on upstream routing init order.
// The `endpoint` arg picks which handler method to run.
func invokeReport(t *testing.T, store configstore.ConfigStore, endpoint string, query string) map[string]any {
	t.Helper()
	h := NewComplianceReportsHandler(store.DB(), &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/api/reports/" + endpoint + "?" + query)
	switch endpoint {
	case "admin-activity":
		h.adminActivity(ctx)
	case "access-control":
		h.accessControl(ctx)
	default:
		t.Fatalf("unknown endpoint %q", endpoint)
	}
	if got := ctx.Response.StatusCode(); got != fasthttp.StatusOK {
		t.Fatalf("status %d, body=%s", got, ctx.Response.Body())
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &out))
	return out
}

func TestAdminActivity_GroupsAndCounts(t *testing.T) {
	store := newReportsTestStore(t)
	// 3 role.create allowed, 1 role.create denied, 2 user.create allowed.
	seedAudit(t, store, "role.create", "allowed", 1)
	seedAudit(t, store, "role.create", "allowed", 2)
	seedAudit(t, store, "role.create", "allowed", 3)
	seedAudit(t, store, "role.create", "denied", 1)
	seedAudit(t, store, "user.create", "allowed", 1)
	seedAudit(t, store, "user.create", "allowed", 1)
	// Outside the 7-day window: should be excluded.
	seedAudit(t, store, "role.create", "allowed", 90)

	body := invokeReport(t, store, "admin-activity", "days=7")
	if body["window_days"] != float64(7) {
		t.Errorf("window_days: %v", body["window_days"])
	}
	buckets, _ := body["buckets"].([]any)
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets (role.create/allowed, role.create/denied, user.create/allowed); got %d: %+v", len(buckets), buckets)
	}
	// Buckets are ordered by count DESC — top should be role.create/allowed = 3.
	first, _ := buckets[0].(map[string]any)
	if first["action"] != "role.create" || first["outcome"] != "allowed" || first["count"] != float64(3) {
		t.Errorf("first bucket wrong: %+v", first)
	}
}

func TestAdminActivity_EmptyDB_ReturnsZeroBuckets(t *testing.T) {
	store := newReportsTestStore(t)
	body := invokeReport(t, store, "admin-activity", "days=30")
	buckets, _ := body["buckets"].([]any)
	if len(buckets) != 0 {
		t.Errorf("empty DB should return zero buckets; got %d", len(buckets))
	}
}

func TestAdminActivity_ClampsDaysRange(t *testing.T) {
	store := newReportsTestStore(t)
	// days=0 → default 30. days=9999 → 365.
	body := invokeReport(t, store, "admin-activity", "days=0")
	if body["window_days"] != float64(30) {
		t.Errorf("days=0 should clamp to 30; got %v", body["window_days"])
	}
	body = invokeReport(t, store, "admin-activity", "days=9999")
	if body["window_days"] != float64(365) {
		t.Errorf("days=9999 should clamp to 365; got %v", body["window_days"])
	}
}

func TestAccessControl_CountsByActionPrefix(t *testing.T) {
	store := newReportsTestStore(t)
	seedAudit(t, store, "role.create", "allowed", 1)
	seedAudit(t, store, "role.update", "allowed", 2)
	seedAudit(t, store, "role.delete", "allowed", 2)
	seedAudit(t, store, "assignment.create", "allowed", 3)
	seedAudit(t, store, "user.create", "allowed", 4)
	seedAudit(t, store, "user.delete", "allowed", 5)
	seedAudit(t, store, "apikey.rotate", "allowed", 6)
	// Outside window: excluded from default 30-day query.
	seedAudit(t, store, "role.create", "allowed", 100)

	body := invokeReport(t, store, "access-control", "days=30")
	type testCase struct {
		key  string
		want float64
	}
	cases := []testCase{
		{"role_changes", 3},      // role.create + role.update + role.delete
		{"role_assignments", 1},  // assignment.create
		{"user_creates", 1},
		{"user_deletes", 1},
		{"key_rotations", 1},
	}
	for _, c := range cases {
		if got := body[c.key]; got != c.want {
			t.Errorf("%s: want %v, got %v (%s)", c.key, c.want, got, fmt.Sprintf("%T", got))
		}
	}
}
