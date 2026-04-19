// Enterprise migrations for the logstore.
//
// Constitution Principle XI rule 2 — sibling-file additive registration.
// Upstream's framework/logstore/migrations.go is not modified. Enterprise
// migrations register here under IDs prefixed "E###_<name>" so they sort
// disjoint from upstream's descriptive IDs in the migration tracking
// table.
//
// Wiring: the enterprise-gate plugin (plugins/enterprise-gate) calls
// RegisterEnterpriseMigrations(ctx, db) in its Init() once per store.

package logstore

import (
	"context"
	"fmt"
	"time"

	"github.com/maximhq/bifrost/framework/migrator"
	"gorm.io/gorm"
)

// RegisterEnterpriseMigrations applies all enterprise logstore migrations.
// Safe to call multiple times; gormigrate skips already-applied IDs.
func RegisterEnterpriseMigrations(ctx context.Context, db *gorm.DB) error {
	migrations := []*migrator.Migration{
		migrationE003CreateLogTenancyAndAudit(ctx),
		// E006/E009/E012/E014/E015/E023/E025 land in subsequent tasks.
	}

	m := migrator.New(db, migrator.DefaultOptions, migrations)
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("enterprise logstore migrations failed: %w", err)
	}
	return nil
}

// migrationE003CreateLogTenancyAndAudit creates the log-tenancy sidecar
// AND the audit_entries table. Audit is co-created here in Phase 2
// because every other enterprise plugin emits audit entries; deferring
// it to US4 would force every Phase-2-onward plugin to no-op its emit
// calls.
//
// The log-tenancy sidecar is backfilled with default-org rows for any
// pre-existing log rows so cross-tenant queries against historical data
// resolve to the synthetic default tenant rather than dropping the rows.
//
// Default org/workspace UUIDs are looked up from the configstore's
// ent_system_defaults singleton via raw SQL — this avoids a Go-level
// import cycle (logstore must not import configstore).
func migrationE003CreateLogTenancyAndAudit(ctx context.Context) *migrator.Migration {
	return &migrator.Migration{
		ID: "E003_create_log_tenancy_and_audit",
		Migrate: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)

			if err := tx.AutoMigrate(
				&TableLogTenancy{},
				&TableAuditEntry{},
			); err != nil {
				return fmt.Errorf("auto-migrate ent log tables: %w", err)
			}

			// Resolve the synthetic defaults from configstore via raw SQL
			// (the same database is shared between configstore and logstore
			// in Bifrost's standard layout; deployments that split them
			// must skip the backfill — handled by the HasTable check).
			if !tx.Migrator().HasTable("ent_system_defaults") {
				// configstore is in a different DB; backfill skipped.
				// The enterprise-gate plugin will populate sidecars
				// lazily as new logs are written.
				return nil
			}

			type defaultsRow struct {
				DefaultOrganizationID string
				DefaultWorkspaceID    string
			}
			var sd defaultsRow
			if err := tx.Raw(`
				SELECT default_organization_id, default_workspace_id
				FROM ent_system_defaults
				WHERE id = 'system'
			`).Scan(&sd).Error; err != nil {
				return fmt.Errorf("read system_defaults: %w", err)
			}
			if sd.DefaultOrganizationID == "" {
				return fmt.Errorf("system_defaults row missing or empty (E001 must run first)")
			}

			now := time.Now().UTC()

			// Backfill log_tenancy for existing log rows.
			if tx.Migrator().HasTable("logs") {
				if err := tx.Exec(`
					INSERT INTO ent_log_tenancy (log_id, organization_id, workspace_id, virtual_key_id, created_at)
					SELECT l.id, ?, ?, COALESCE(l.virtual_key_id, ''), ?
					FROM logs l
					WHERE NOT EXISTS (
						SELECT 1 FROM ent_log_tenancy s WHERE s.log_id = l.id
					)
				`, sd.DefaultOrganizationID, sd.DefaultWorkspaceID, now).Error; err != nil {
					// virtual_key_id may not exist on the logs table in
					// older schemas; retry without it.
					if err2 := tx.Exec(`
						INSERT INTO ent_log_tenancy (log_id, organization_id, workspace_id, virtual_key_id, created_at)
						SELECT l.id, ?, ?, '', ?
						FROM logs l
						WHERE NOT EXISTS (
							SELECT 1 FROM ent_log_tenancy s WHERE s.log_id = l.id
						)
					`, sd.DefaultOrganizationID, sd.DefaultWorkspaceID, now).Error; err2 != nil {
						return fmt.Errorf("backfill log_tenancy: %w", err2)
					}
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			for _, t := range []any{
				&TableAuditEntry{},
				&TableLogTenancy{},
			} {
				if err := tx.Migrator().DropTable(t); err != nil {
					return fmt.Errorf("drop ent log table: %w", err)
				}
			}
			return nil
		},
	}
}
