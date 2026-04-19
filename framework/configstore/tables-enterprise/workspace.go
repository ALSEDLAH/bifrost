package tables_enterprise

import "time"

// TableWorkspace is the sub-organization isolation boundary. Every
// tenant-scoped resource (virtual keys, prompts, configs, guardrails,
// logs) scopes by (organization_id, workspace_id).
//
// Per data-model §1.
type TableWorkspace struct {
	ID             string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	OrganizationID string `gorm:"type:varchar(255);not null;index:idx_workspaces_org_slug,unique" json:"organization_id"`

	Name        string `gorm:"type:varchar(255);not null" json:"name"`
	Slug        string `gorm:"type:varchar(255);not null;index:idx_workspaces_org_slug,unique" json:"slug"`
	Description string `gorm:"type:text" json:"description,omitempty"`

	// Per-workspace retention overrides (nil = use org default from
	// TableOrganization.DefaultRetentionDays).
	LogRetentionDays    *int `json:"log_retention_days,omitempty"`
	MetricRetentionDays *int `json:"metric_retention_days,omitempty"`

	// PayloadEncryptionEnabled opts this workspace into logstore
	// payload encryption via BYOK (FR-035a). Default false keeps the
	// hot-path performance profile for customers who don't need it.
	// Toggling requires a BYOK kms_config to be active.
	PayloadEncryptionEnabled bool `gorm:"not null;default:false" json:"payload_encryption_enabled"`

	CreatedAt time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt time.Time  `gorm:"index;not null" json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (TableWorkspace) TableName() string { return "ent_workspaces" }
