// Package tables_enterprise contains GORM struct definitions for all
// enterprise-only configstore tables.
//
// Constitution Principle XI rule 1 — sibling-file extension. This package
// is parallel to framework/configstore/tables; upstream's tables/ is never
// edited. Tenancy on existing upstream tables is attached via 1:1 sidecar
// tables in this package (see virtual_key_tenancy.go etc.).
//
// All tables in this package use the prefix "ent_" to make them visually
// distinct from upstream tables in any database tooling.
package tables_enterprise

import "time"

// TableSystemDefaults is a singleton row holding the synthetic
// SYSTEM_DEFAULT_ORG_UUID and SYSTEM_DEFAULT_WORKSPACE_UUID. The
// enterprise-gate plugin reads this row at startup; the values seed
// every tenant-scoped table for v1 single-org deployments.
//
// The row's primary key is hard-coded to "system" so that re-running the
// seed migration is an idempotent no-op (research R-11).
type TableSystemDefaults struct {
	// ID is always the literal string "system" — there is exactly one row.
	ID string `gorm:"primaryKey;type:varchar(32)" json:"id"`

	// DefaultOrganizationID is the UUID v7 of the synthetic default
	// organization for single-org-mode deployments. Persisted at first
	// boot; never regenerated.
	DefaultOrganizationID string `gorm:"type:varchar(255);not null" json:"default_organization_id"`

	// DefaultWorkspaceID is the UUID v7 of the synthetic default
	// workspace under the default organization.
	DefaultWorkspaceID string `gorm:"type:varchar(255);not null" json:"default_workspace_id"`

	// SeededAt records when the synthetic defaults were generated.
	SeededAt time.Time `gorm:"not null" json:"seeded_at"`
}

// TableName returns the database table name. Prefix "ent_" marks this as
// an enterprise-overlay table.
func (TableSystemDefaults) TableName() string { return "ent_system_defaults" }

// SystemDefaultsRowID is the literal PK value for the singleton row.
const SystemDefaultsRowID = "system"
