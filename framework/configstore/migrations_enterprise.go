// Enterprise migrations for the configstore.
//
// Sibling file to upstream's migrations.go. Enterprise migration IDs
// use "E###_<name>" prefix to avoid collision with upstream IDs.
//
// Post-audit revision: E002 (sidecar tables) REMOVED — upstream tables
// already have CustomerID/TeamID FKs. E004 simplified — Organization
// and Workspace tables dropped (governance_customers/teams are canonical).

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
		// E002 (sidecar tables) REMOVED — upstream CustomerID/TeamID FKs suffice.
		// E003 is in framework/logstore/migrations_enterprise.go (logstore-side).
		migrationE004UsersAndRoles(ctx),
		migrationE005AlertChannels(ctx),
		migrationE006MCPToolGroups(ctx),
	}

	m := migrator.New(db, migrator.DefaultOptions, migrations)
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("enterprise configstore migrations failed: %w", err)
	}
	return nil
}

// builtInRoles uses the frontend's 24 RbacResource × 6 RbacOperation model.
// Resource names match ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx.
var builtInRoles = []struct {
	Name      string
	IsBuiltin bool
	ScopeJSON string
}{
	// Owner: full access to everything.
	{Name: "Owner", IsBuiltin: true, ScopeJSON: `{"*": ["Read","View","Create","Update","Delete","Download"]}`},
	// Admin: broad access minus some sensitive operations.
	{Name: "Admin", IsBuiltin: true, ScopeJSON: `{
		"GuardrailsConfig": ["Read","View","Create","Update","Delete"],
		"GuardrailsProviders": ["Read","View","Create","Update","Delete"],
		"GuardrailRules": ["Read","View","Create","Update","Delete"],
		"UserProvisioning": ["Read","View","Create","Update"],
		"Settings": ["Read","View","Update"],
		"Users": ["Read","View","Create","Update","Delete"],
		"Logs": ["Read","View","Download"],
		"Observability": ["Read","View","Update"],
		"VirtualKeys": ["Read","View","Create","Update","Delete"],
		"ModelProvider": ["Read","View","Create","Update","Delete"],
		"Plugins": ["Read","View","Create","Update","Delete"],
		"MCPGateway": ["Read","View","Create","Update","Delete"],
		"AuditLogs": ["Read","View","Download"],
		"Customers": ["Read","View","Create","Update","Delete"],
		"Teams": ["Read","View","Create","Update","Delete"],
		"RBAC": ["Read","View","Create","Update"],
		"Governance": ["Read","View","Create","Update","Delete"],
		"RoutingRules": ["Read","View","Create","Update","Delete"],
		"PromptRepository": ["Read","View","Create","Update","Delete"],
		"PromptDeploymentStrategy": ["Read","View","Create","Update"],
		"AccessProfiles": ["Read","View","Create","Update","Delete"],
		"AlertChannels": ["Read","View","Create","Update","Delete"],
		"MCPToolGroups": ["Read","View","Create","Update","Delete"]
	}`},
	// Member: read/view most things, write on prompts/configs.
	{Name: "Member", IsBuiltin: true, ScopeJSON: `{
		"Settings": ["Read","View"],
		"Logs": ["Read","View"],
		"Observability": ["Read","View"],
		"VirtualKeys": ["Read","View"],
		"ModelProvider": ["Read","View"],
		"Plugins": ["Read","View"],
		"MCPGateway": ["Read","View"],
		"AuditLogs": ["Read","View"],
		"Customers": ["Read","View"],
		"Teams": ["Read","View"],
		"Governance": ["Read","View"],
		"RoutingRules": ["Read","View"],
		"PromptRepository": ["Read","View","Create","Update"],
		"GuardrailsConfig": ["Read","View"]
	}`},
	// Manager: workspace-level admin.
	{Name: "Manager", IsBuiltin: true, ScopeJSON: `{
		"Settings": ["Read","View","Update"],
		"Users": ["Read","View","Create","Update"],
		"Logs": ["Read","View","Download"],
		"Observability": ["Read","View","Update"],
		"VirtualKeys": ["Read","View","Create","Update","Delete"],
		"ModelProvider": ["Read","View","Create","Update"],
		"MCPGateway": ["Read","View","Create","Update"],
		"Customers": ["Read","View"],
		"Teams": ["Read","View","Create","Update"],
		"Governance": ["Read","View","Create","Update"],
		"RoutingRules": ["Read","View","Create","Update","Delete"],
		"PromptRepository": ["Read","View","Create","Update","Delete"],
		"PromptDeploymentStrategy": ["Read","View","Create","Update"],
		"GuardrailsConfig": ["Read","View","Create","Update"]
	}`},
}

// migrationE004UsersAndRoles creates the enterprise users + roles tables
// and seeds the four built-in roles.
func migrationE004UsersAndRoles(ctx context.Context) *migrator.Migration {
	return &migrator.Migration{
		ID: "E004_orgs_workspaces_users_roles",
		Migrate: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)

			// Create users + roles tables only (orgs/workspaces use governance tables).
			if err := tx.AutoMigrate(
				&tables_enterprise.TableUser{},
				&tables_enterprise.TableRole{},
				&tables_enterprise.TableUserRoleAssignment{},
			); err != nil {
				return fmt.Errorf("auto-migrate users/roles: %w", err)
			}

			// Resolve the synthetic default org UUID from E001.
			var sd tables_enterprise.TableSystemDefaults
			if err := tx.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&sd).Error; err != nil {
				return fmt.Errorf("read system_defaults (E001 must run first): %w", err)
			}

			now := time.Now().UTC()

			// Seed the four built-in roles.
			for _, r := range builtInRoles {
				var existing tables_enterprise.TableRole
				err := tx.Where("organization_id = ? AND name = ?", sd.DefaultOrganizationID, r.Name).First(&existing).Error
				if err == nil {
					continue
				}
				role := tables_enterprise.TableRole{
					ID:             uuid.NewString(),
					OrganizationID: sd.DefaultOrganizationID,
					Name:           r.Name,
					ScopeJSON:      r.ScopeJSON,
					IsBuiltin:      r.IsBuiltin,
					CreatedAt:      now,
				}
				if err := tx.Create(&role).Error; err != nil {
					return fmt.Errorf("seed built-in role %s: %w", r.Name, err)
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			for _, t := range []any{
				&tables_enterprise.TableUserRoleAssignment{},
				&tables_enterprise.TableRole{},
				&tables_enterprise.TableUser{},
			} {
				if err := tx.Migrator().DropTable(t); err != nil {
					return fmt.Errorf("drop table: %w", err)
				}
			}
			return nil
		},
	}
}

// migrationE001SeedDefaultOrg creates ent_system_defaults and seeds
// the singleton row with synthetic org+workspace UUIDs.
func migrationE001SeedDefaultOrg(ctx context.Context) *migrator.Migration {
	return &migrator.Migration{
		ID: "E001_seed_default_org",
		Migrate: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)

			if err := tx.AutoMigrate(&tables_enterprise.TableSystemDefaults{}); err != nil {
				return fmt.Errorf("auto-migrate ent_system_defaults: %w", err)
			}

			var existing tables_enterprise.TableSystemDefaults
			if err := tx.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&existing).Error; err == nil {
				return nil
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
			return tx.Migrator().DropTable(&tables_enterprise.TableSystemDefaults{})
		},
	}
}

// migrationE005AlertChannels creates the ent_alert_channels table used by
// the governance plugin's threshold-crossing dispatcher (spec 004).
func migrationE005AlertChannels(ctx context.Context) *migrator.Migration {
	return &migrator.Migration{
		ID: "E005_alert_channels",
		Migrate: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			if err := tx.AutoMigrate(&tables_enterprise.TableAlertChannel{}); err != nil {
				return fmt.Errorf("auto-migrate alert_channels: %w", err)
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			return tx.Migrator().DropTable(&tables_enterprise.TableAlertChannel{})
		},
	}
}

// migrationE006MCPToolGroups creates the ent_mcp_tool_groups table used
// by the MCP tool-groups admin UI (spec 005).
func migrationE006MCPToolGroups(ctx context.Context) *migrator.Migration {
	return &migrator.Migration{
		ID: "E006_mcp_tool_groups",
		Migrate: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			if err := tx.AutoMigrate(&tables_enterprise.TableMCPToolGroup{}); err != nil {
				return fmt.Errorf("auto-migrate mcp_tool_groups: %w", err)
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			return tx.Migrator().DropTable(&tables_enterprise.TableMCPToolGroup{})
		},
	}
}
