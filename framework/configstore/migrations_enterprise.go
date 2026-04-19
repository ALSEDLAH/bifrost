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
		migrationE002CreateTenancySidecars(ctx),
		// E003 is in framework/logstore/migrations_enterprise.go (logstore-side).
		// E004..E024 land in subsequent tasks.
	}

	m := migrator.New(db, migrator.DefaultOptions, migrations)
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("enterprise configstore migrations failed: %w", err)
	}
	return nil
}

// migrationE002CreateTenancySidecars creates the 5 sidecar tables that
// attach tenancy to upstream's governance_virtual_keys / governance_teams
// / governance_customers / governance_providers / governance_keys, then
// backfills one sidecar row per existing upstream row pointing at the
// synthetic default org+workspace (R-03).
//
// The backfill is idempotent: rows that already have a sidecar are
// skipped via INSERT...ON CONFLICT DO NOTHING semantics, expressed
// portably as "select existing + skip" so SQLite + PostgreSQL behave
// identically.
//
// Rollback drops the sidecar tables. Upstream tables are untouched.
func migrationE002CreateTenancySidecars(ctx context.Context) *migrator.Migration {
	return &migrator.Migration{
		ID: "E002_create_tenancy_sidecars",
		Migrate: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)

			// 1. Create the 5 sidecar tables.
			if err := tx.AutoMigrate(
				&tables_enterprise.TableVirtualKeyTenancy{},
				&tables_enterprise.TableTeamTenancy{},
				&tables_enterprise.TableCustomerTenancy{},
				&tables_enterprise.TableProviderTenancy{},
				&tables_enterprise.TableProviderKeyTenancy{},
			); err != nil {
				return fmt.Errorf("auto-migrate sidecar tables: %w", err)
			}

			// 2. Resolve the synthetic default org/workspace (seeded by E001).
			var sd tables_enterprise.TableSystemDefaults
			if err := tx.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&sd).Error; err != nil {
				return fmt.Errorf("read system_defaults (E001 must run first): %w", err)
			}

			now := time.Now().UTC()

			// 3. Backfill one sidecar row per pre-existing upstream row.
			//    Use raw SQL with INSERT ... SELECT WHERE NOT EXISTS so we
			//    don't depend on dialect-specific ON CONFLICT semantics.
			backfills := []struct {
				name      string
				upstream  string // upstream table name
				sidecar   string // sidecar table name
				fkColumn  string // sidecar FK column (== sidecar PK)
				wsScoped  bool   // whether to set workspace_id
			}{
				{"virtual_keys", "governance_virtual_keys", "ent_virtual_key_tenancy", "virtual_key_id", true},
				{"teams", "governance_teams", "ent_team_tenancy", "team_id", true},
				{"customers", "governance_customers", "ent_customer_tenancy", "customer_id", true},
				{"providers", "governance_providers", "ent_provider_tenancy", "provider_id", false},
				{"provider_keys", "governance_keys", "ent_provider_key_tenancy", "key_id", false},
			}

			for _, b := range backfills {
				// Skip when upstream table doesn't exist (allows an enterprise
				// build to boot on a virgin DB before upstream's migrations
				// have created its own tables — defensive ordering).
				if !tx.Migrator().HasTable(b.upstream) {
					continue
				}

				var sql string
				if b.wsScoped {
					sql = fmt.Sprintf(`
						INSERT INTO %s (%s, organization_id, workspace_id, created_at, updated_at)
						SELECT u.id, ?, ?, ?, ?
						FROM %s u
						WHERE NOT EXISTS (
							SELECT 1 FROM %s s WHERE s.%s = u.id
						)
					`, b.sidecar, b.fkColumn, b.upstream, b.sidecar, b.fkColumn)
					if err := tx.Exec(sql, sd.DefaultOrganizationID, sd.DefaultWorkspaceID, now, now).Error; err != nil {
						return fmt.Errorf("backfill %s: %w", b.name, err)
					}
				} else {
					sql = fmt.Sprintf(`
						INSERT INTO %s (%s, organization_id, created_at, updated_at)
						SELECT u.id, ?, ?, ?
						FROM %s u
						WHERE NOT EXISTS (
							SELECT 1 FROM %s s WHERE s.%s = u.id
						)
					`, b.sidecar, b.fkColumn, b.upstream, b.sidecar, b.fkColumn)
					if err := tx.Exec(sql, sd.DefaultOrganizationID, now, now).Error; err != nil {
						return fmt.Errorf("backfill %s: %w", b.name, err)
					}
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			for _, t := range []any{
				&tables_enterprise.TableProviderKeyTenancy{},
				&tables_enterprise.TableProviderTenancy{},
				&tables_enterprise.TableCustomerTenancy{},
				&tables_enterprise.TableTeamTenancy{},
				&tables_enterprise.TableVirtualKeyTenancy{},
			} {
				if err := tx.Migrator().DropTable(t); err != nil {
					return fmt.Errorf("drop sidecar: %w", err)
				}
			}
			return nil
		},
	}
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
