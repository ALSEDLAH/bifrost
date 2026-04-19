// Tenancy integration test.
//
// Constitution Principle VIII — real dependencies. Run against a live
// PostgreSQL instance brought up by docker-compose.enterprise.yml:
//
//	docker compose -f docker-compose.enterprise.yml up -d postgres
//	BIFROST_ENT_PG_DSN="host=localhost port=55432 user=bifrost password=bifrost_test dbname=bifrost_enterprise sslmode=disable" \
//	  go test -tags=integration ./framework/tenancy/...
//
// Without the integration tag this file is skipped — `go test ./...`
// remains fast for routine development.
//
//go:build integration

package tenancy_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/tenancy"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type tenancyRow struct {
	ID             string `gorm:"primaryKey;type:varchar(255)"`
	OrganizationID string `gorm:"type:varchar(255);not null;index"`
	WorkspaceID    string `gorm:"type:varchar(255);not null;index"`
	Payload        string `gorm:"type:text"`
}

func (tenancyRow) TableName() string { return "ent_tenancy_test_rows" }

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BIFROST_ENT_PG_DSN")
	if dsn == "" {
		t.Skip("BIFROST_ENT_PG_DSN not set; skipping (expected in non-CI environments)")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.AutoMigrate(&tenancyRow{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Migrator().DropTable(&tenancyRow{}) })
	return db
}

func newCtxWith(tc tenancy.TenantContext) *schemas.BifrostContext {
	bctx := schemas.NewBifrostContext(context.Background())
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, tc.OrganizationID)
	bctx.SetValue(tenancy.BifrostContextKeyWorkspaceID, tc.WorkspaceID)
	bctx.SetValue(tenancy.BifrostContextKeyRoleScopes, tc.RoleScopes)
	bctx.SetValue(tenancy.BifrostContextKeyTenantContext, tc)
	return bctx
}

// TestScopedDB_BlocksCrossTenantReads verifies that two workspaces in two
// different organizations cannot see each other's rows via the
// tenancy.ScopedDB helper. This is the SC-001 cross-tenant guarantee.
func TestScopedDB_BlocksCrossTenantReads(t *testing.T) {
	db := newTestDB(t)

	orgA, wsA := uuid.NewString(), uuid.NewString()
	orgB, wsB := uuid.NewString(), uuid.NewString()

	rows := []tenancyRow{
		{ID: uuid.NewString(), OrganizationID: orgA, WorkspaceID: wsA, Payload: "alpha"},
		{ID: uuid.NewString(), OrganizationID: orgA, WorkspaceID: wsA, Payload: "beta"},
		{ID: uuid.NewString(), OrganizationID: orgB, WorkspaceID: wsB, Payload: "gamma"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	ctxA := newCtxWith(tenancy.TenantContext{
		OrganizationID: orgA, WorkspaceID: wsA, ResolvedVia: tenancy.ResolverAdminAPIKey,
	})
	scopedA, err := tenancy.ScopedDB(ctxA, db, true)
	if err != nil {
		t.Fatalf("scoped: %v", err)
	}

	var seen []tenancyRow
	if err := scopedA.Find(&seen).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("orgA should see exactly 2 rows; got %d", len(seen))
	}
	for _, r := range seen {
		if r.OrganizationID != orgA {
			t.Fatalf("scoped query leaked row from org %s", r.OrganizationID)
		}
	}

	// Switch context to orgB; should see only the gamma row.
	ctxB := newCtxWith(tenancy.TenantContext{
		OrganizationID: orgB, WorkspaceID: wsB, ResolvedVia: tenancy.ResolverAdminAPIKey,
	})
	scopedB, err := tenancy.ScopedDB(ctxB, db, true)
	if err != nil {
		t.Fatalf("scoped B: %v", err)
	}
	var seenB []tenancyRow
	if err := scopedB.Find(&seenB).Error; err != nil {
		t.Fatalf("find B: %v", err)
	}
	if len(seenB) != 1 || seenB[0].Payload != "gamma" {
		t.Fatalf("orgB should see only gamma; got %v", seenB)
	}

	// Sanity: an unscoped query sees all 3 rows. This proves the
	// filtering is what blocks the leak — not the absence of data.
	var raw []tenancyRow
	if err := db.Find(&raw).Error; err != nil {
		t.Fatalf("raw find: %v", err)
	}
	if len(raw) != 3 {
		t.Fatalf("expected 3 rows total; got %d", len(raw))
	}

	// Sanity: zero TenantContext returns ErrNoTenantContext.
	if _, err := tenancy.ScopedDB(schemas.NewBifrostContext(context.Background()), db, true); err == nil {
		t.Fatalf("expected ErrNoTenantContext; got nil")
	}

	_ = time.Now() // keeps `time` import non-unused if test is shortened
}
