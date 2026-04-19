// Unit tests for the audit.Emit serialization layer.
//
// Uses an in-memory SQLite to avoid the docker-compose stack — these
// are unit tests, not integration tests. Validates: tenant attribution,
// before/after JSON serialization, default-outcome substitution,
// missing-tenant rejection.

package audit_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	if err := db.AutoMigrate(&logstore.TableAuditEntry{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func newCtx(tc tenancy.TenantContext) *schemas.BifrostContext {
	bctx := schemas.NewBifrostContext(context.Background())
	bctx.SetValue(tenancy.BifrostContextKeyTenantContext, tc)
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, tc.OrganizationID)
	bctx.SetValue(tenancy.BifrostContextKeyWorkspaceID, tc.WorkspaceID)
	return bctx
}

func TestEmit_HappyPath(t *testing.T) {
	db := newSQLite(t)
	p, err := audit.Init(context.Background(), db, nil, audit.Config{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer p.Cleanup()

	bctx := newCtx(tenancy.TenantContext{
		OrganizationID: "org-1",
		WorkspaceID:    "ws-1",
		UserID:         "user-42",
		ResolvedVia:    tenancy.ResolverSession,
	})

	type wsBefore struct {
		Name string `json:"name"`
	}
	if err := audit.Emit(context.Background(), bctx, audit.Entry{
		Action:       "workspace.update",
		ResourceType: "workspace",
		ResourceID:   "ws-1",
		Before:       wsBefore{Name: "old"},
		After:        wsBefore{Name: "new"},
	}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	var rows []logstore.TableAuditEntry
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row; got %d", len(rows))
	}
	r := rows[0]
	if r.OrganizationID != "org-1" || r.WorkspaceID != "ws-1" {
		t.Fatalf("tenant misattributed: %+v", r)
	}
	if r.ActorID != "user-42" {
		t.Fatalf("actor id missing: %+v", r)
	}
	if r.Outcome != "allowed" {
		t.Fatalf("default outcome should be allowed; got %s", r.Outcome)
	}
	var b wsBefore
	if err := json.Unmarshal([]byte(r.BeforeJSON), &b); err != nil || b.Name != "old" {
		t.Fatalf("before snapshot serialization broken: %s err=%v", r.BeforeJSON, err)
	}
}

func TestEmit_RejectsMissingTenant(t *testing.T) {
	db := newSQLite(t)
	p, _ := audit.Init(context.Background(), db, nil, audit.Config{})
	defer p.Cleanup()

	emptyCtx := schemas.NewBifrostContext(context.Background())
	err := audit.Emit(context.Background(), emptyCtx, audit.Entry{
		Action: "workspace.update", ResourceType: "workspace", ResourceID: "ws-1",
	})
	if err == nil {
		t.Fatal("expected error when TenantContext missing; got nil")
	}
}

func TestEmit_NoSinkReturnsErrNoSink(t *testing.T) {
	// No Init call — defaultSink is nil.
	bctx := newCtx(tenancy.TenantContext{OrganizationID: "org-1", WorkspaceID: "ws-1"})
	err := audit.Emit(context.Background(), bctx, audit.Entry{
		Action: "test", ResourceType: "test", ResourceID: "x",
	})
	if err != audit.ErrNoSink {
		t.Fatalf("want ErrNoSink; got %v", err)
	}
}

func TestEmitDenied_SetsOutcome(t *testing.T) {
	db := newSQLite(t)
	p, _ := audit.Init(context.Background(), db, nil, audit.Config{})
	defer p.Cleanup()
	bctx := newCtx(tenancy.TenantContext{OrganizationID: "org-1", WorkspaceID: "ws-1"})

	if err := audit.EmitDenied(context.Background(), bctx,
		"workspace.delete", "workspace", "ws-1", "missing scope workspaces:delete",
	); err != nil {
		t.Fatalf("emit: %v", err)
	}

	var row logstore.TableAuditEntry
	_ = db.Order("created_at desc").First(&row).Error
	if row.Outcome != "denied" || row.Reason == "" {
		t.Fatalf("denied entry malformed: %+v", row)
	}
}
