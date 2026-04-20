// Enterprise migrations for the logstore.
//
// Post-audit revision: TableLogTenancy sidecar REMOVED — log scoping
// works via VK → Team → Customer chain in governance plugin.
// Only ent_audit_entries table remains.

package logstore

import (
	"context"
	"fmt"

	"github.com/maximhq/bifrost/framework/migrator"
	"gorm.io/gorm"
)

// RegisterEnterpriseMigrations applies all enterprise logstore migrations.
func RegisterEnterpriseMigrations(ctx context.Context, db *gorm.DB) error {
	migrations := []*migrator.Migration{
		migrationE003CreateAuditEntries(ctx),
	}

	m := migrator.New(db, migrator.DefaultOptions, migrations)
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("enterprise logstore migrations failed: %w", err)
	}
	return nil
}

// migrationE003CreateAuditEntries creates the audit entries table.
// ID kept as "E003_create_log_tenancy_and_audit" for backward compat
// with databases that already ran the original migration.
func migrationE003CreateAuditEntries(ctx context.Context) *migrator.Migration {
	return &migrator.Migration{
		ID: "E003_create_log_tenancy_and_audit",
		Migrate: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			if err := tx.AutoMigrate(&TableAuditEntry{}); err != nil {
				return fmt.Errorf("auto-migrate ent_audit_entries: %w", err)
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			return tx.Migrator().DropTable(&TableAuditEntry{})
		},
	}
}
