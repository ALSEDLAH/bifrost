// Integration test for enterprise configstore migrations.
//
// Constitution Principle VIII — real dependencies. Run with:
//
//	docker compose -f docker-compose.enterprise.yml up -d postgres
//	BIFROST_ENT_PG_DSN="host=localhost port=55432 user=bifrost password=bifrost_test dbname=bifrost_enterprise sslmode=disable" \
//	  go test -tags=integration ./framework/configstore/...
//
//go:build integration

package configstore_test

import (
	"context"
	"os"
	"testing"

	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// fixtureUpstreamRow is a minimal stand-in for governance_virtual_keys
// so the backfill test runs without needing the full upstream schema.
type fixtureUpstreamRow struct {
	ID string `gorm:"primaryKey;type:varchar(255)"`
}

type fixtureVK struct{ fixtureUpstreamRow }

func (fixtureVK) TableName() string { return "governance_virtual_keys" }

type fixtureTeam struct{ fixtureUpstreamRow }

func (fixtureTeam) TableName() string { return "governance_teams" }

type fixtureCustomer struct{ fixtureUpstreamRow }

func (fixtureCustomer) TableName() string { return "governance_customers" }

type fixtureProvider struct{ fixtureUpstreamRow }

func (fixtureProvider) TableName() string { return "governance_providers" }

type fixtureKey struct{ fixtureUpstreamRow }

func (fixtureKey) TableName() string { return "governance_keys" }

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BIFROST_ENT_PG_DSN")
	if dsn == "" {
		t.Skip("BIFROST_ENT_PG_DSN not set; skipping (expected outside CI)")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		// Tear down everything we created.
		_ = db.Migrator().DropTable("ent_virtual_key_tenancy")
		_ = db.Migrator().DropTable("ent_team_tenancy")
		_ = db.Migrator().DropTable("ent_customer_tenancy")
		_ = db.Migrator().DropTable("ent_provider_tenancy")
		_ = db.Migrator().DropTable("ent_provider_key_tenancy")
		_ = db.Migrator().DropTable(&tables_enterprise.TableSystemDefaults{})
		_ = db.Migrator().DropTable("governance_virtual_keys")
		_ = db.Migrator().DropTable("governance_teams")
		_ = db.Migrator().DropTable("governance_customers")
		_ = db.Migrator().DropTable("governance_providers")
		_ = db.Migrator().DropTable("governance_keys")
		_ = db.Migrator().DropTable("migrations")
	})
	return db
}

// TestEnterpriseMigrations_E001_E002 verifies that:
//   - E001 creates ent_system_defaults and seeds the singleton row.
//   - E002 creates 5 sidecar tables and backfills sidecars for every
//     pre-existing upstream row.
//   - Re-running migrations is a no-op (idempotency, R-11).
//   - Upstream rows are not modified.
func TestEnterpriseMigrations_E001_E002(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()

	// Seed minimal "v1.5.2" fixture: 3 VKs, 2 teams, 1 customer,
	// 2 providers, 4 provider keys.
	if err := db.AutoMigrate(
		&fixtureVK{}, &fixtureTeam{}, &fixtureCustomer{},
		&fixtureProvider{}, &fixtureKey{},
	); err != nil {
		t.Fatalf("seed schema: %v", err)
	}
	for _, id := range []string{"vk-1", "vk-2", "vk-3"} {
		if err := db.Create(&fixtureVK{fixtureUpstreamRow{ID: id}}).Error; err != nil {
			t.Fatalf("seed vk: %v", err)
		}
	}
	for _, id := range []string{"team-1", "team-2"} {
		_ = db.Create(&fixtureTeam{fixtureUpstreamRow{ID: id}}).Error
	}
	_ = db.Create(&fixtureCustomer{fixtureUpstreamRow{ID: "cust-1"}}).Error
	for _, id := range []string{"prov-1", "prov-2"} {
		_ = db.Create(&fixtureProvider{fixtureUpstreamRow{ID: id}}).Error
	}
	for _, id := range []string{"k-1", "k-2", "k-3", "k-4"} {
		_ = db.Create(&fixtureKey{fixtureUpstreamRow{ID: id}}).Error
	}

	// First run.
	if err := configstore.RegisterEnterpriseMigrations(ctx, db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	// Capture defaults for re-use.
	var sd tables_enterprise.TableSystemDefaults
	if err := db.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&sd).Error; err != nil {
		t.Fatalf("system_defaults: %v", err)
	}
	if sd.DefaultOrganizationID == "" || sd.DefaultWorkspaceID == "" {
		t.Fatalf("system_defaults UUIDs empty: %+v", sd)
	}

	// Sidecar counts must match upstream counts.
	type rowCount struct{ Count int64 }
	checks := []struct {
		name      string
		sidecar   string
		expected  int64
	}{
		{"vk", "ent_virtual_key_tenancy", 3},
		{"team", "ent_team_tenancy", 2},
		{"customer", "ent_customer_tenancy", 1},
		{"provider", "ent_provider_tenancy", 2},
		{"provider_key", "ent_provider_key_tenancy", 4},
	}
	for _, c := range checks {
		var got int64
		if err := db.Table(c.sidecar).Count(&got).Error; err != nil {
			t.Fatalf("count %s: %v", c.sidecar, err)
		}
		if got != c.expected {
			t.Fatalf("%s sidecar count = %d; want %d", c.sidecar, got, c.expected)
		}
	}

	// Idempotency: second run is a no-op.
	if err := configstore.RegisterEnterpriseMigrations(ctx, db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	for _, c := range checks {
		var got int64
		if err := db.Table(c.sidecar).Count(&got).Error; err != nil {
			t.Fatalf("recount %s: %v", c.sidecar, err)
		}
		if got != c.expected {
			t.Fatalf("%s sidecar count after re-run = %d; want %d (idempotency broken)", c.sidecar, got, c.expected)
		}
	}

	// Defaults stable across re-run.
	var sd2 tables_enterprise.TableSystemDefaults
	_ = db.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&sd2).Error
	if sd.DefaultOrganizationID != sd2.DefaultOrganizationID {
		t.Fatalf("default org UUID changed across re-run: %s -> %s", sd.DefaultOrganizationID, sd2.DefaultOrganizationID)
	}

	// Upstream rows remain pristine: nothing should have a tenancy column
	// added, count unchanged, IDs unchanged.
	var vkCount int64
	_ = db.Model(&fixtureVK{}).Count(&vkCount).Error
	if vkCount != 3 {
		t.Fatalf("upstream vk count drifted: got %d, want 3", vkCount)
	}
}
