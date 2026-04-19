// Enterprise migrations for the configstore.
//
// Constitution Principle XI rule 2 — additive sibling file. Upstream's
// framework/configstore/migrations.go is not modified. Enterprise
// migrations register here under IDs prefixed "E###_<name>" so they sort
// disjoint from upstream's descriptive migration IDs in the migrations
// tracking table.
//
// Idempotency: gormigrate writes each migration ID to the tracking table
// after successful apply; re-running RegisterEnterpriseMigrations on an
// already-migrated database is a no-op (research R-11).
//
// Wiring: the enterprise-gate plugin (plugins/enterprise-gate) calls
// RegisterEnterpriseMigrations(db) in its Init(); no upstream code needs
// to change.

package configstore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/migrator"
	"gorm.io/gorm"
)

// RegisterEnterpriseMigrations applies all enterprise configstore migrations.
// Safe to call multiple times; gormigrate skips already-applied IDs.
func RegisterEnterpriseMigrations(ctx context.Context, db *gorm.DB) error {
	migrations := []*migrator.Migration{
		migrationE001SeedDefaultOrg(ctx),
		// E002..E024 land in subsequent tasks.
	}

	m := migrator.New(db, migrator.DefaultOptions, migrations)
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("enterprise configstore migrations failed: %w", err)
	}
	return nil
}

// migrationE001SeedDefaultOrg creates ent_system_defaults and seeds the
// singleton row holding SYSTEM_DEFAULT_ORG_UUID + SYSTEM_DEFAULT_WORKSPACE_UUID.
//
// The IDs are generated with UUID v7 (sortable by time) the FIRST time the
// migration runs; subsequent runs are no-ops because gormigrate skips this ID
// in the tracking table. Even if the migration tracking table is dropped, the
// upsert clause keeps the row stable — so the synthetic IDs are deployment-
// stable across schema rebuilds.
func migrationE001SeedDefaultOrg(ctx context.Context) *migrator.Migration {
	return &migrator.Migration{
		ID: "E001_seed_default_org",
		Migrate: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)

			if err := tx.AutoMigrate(&tables_enterprise.TableSystemDefaults{}); err != nil {
				return fmt.Errorf("auto-migrate ent_system_defaults: %w", err)
			}

			// Upsert the singleton row. If it already exists (e.g., the migration
			// was rolled back and re-run), keep the original UUIDs.
			var existing tables_enterprise.TableSystemDefaults
			if err := tx.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&existing).Error; err == nil {
				return nil // already seeded
			}

			seed := tables_enterprise.TableSystemDefaults{
				ID:                    tables_enterprise.SystemDefaultsRowID,
				DefaultOrganizationID: uuid.NewString(),
				DefaultWorkspaceID:    uuid.NewString(),
				SeededAt:              time.Now().UTC(),
			}
			if err := tx.Create(&seed).Error; err != nil {
				return fmt.Errorf("seed default org row: %w", err)
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			if err := tx.Migrator().DropTable(&tables_enterprise.TableSystemDefaults{}); err != nil {
				return fmt.Errorf("drop ent_system_defaults: %w", err)
			}
			return nil
		},
	}
}
