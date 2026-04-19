package tables_enterprise

import "time"

// TableOrganization is the top-level tenant boundary. In v1 single-org
// deployments exactly one row exists with is_default=true (matching the
// UUID persisted in TableSystemDefaults.DefaultOrganizationID). In cloud
// mode (`deployment.mode: cloud`) many rows exist, one per customer org.
//
// Per data-model §1.
type TableOrganization struct {
	ID   string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name string `gorm:"type:varchar(255);not null" json:"name"`

	// IsDefault is true for the synthetic single-org row in selfhosted/
	// airgapped deployments. Exactly one row has IsDefault=true when the
	// enterprise build is in single-org mode.
	IsDefault bool `gorm:"not null;default:false" json:"is_default"`

	// SSORequired forces every login through SSO. Break-glass local
	// accounts (see BreakGlassEnabled) remain available when true.
	SSORequired bool `gorm:"not null;default:false" json:"sso_required"`

	// BreakGlassEnabled allows a whitelisted set of emails to log in
	// with local credentials even when SSORequired=true. Every break-
	// glass login fires a high-severity audit entry (R-12).
	BreakGlassEnabled bool `gorm:"not null;default:false" json:"break_glass_enabled"`

	// DefaultRetentionDays is the org-level default for log retention.
	// Workspaces may override via TableWorkspace.LogRetentionDays.
	DefaultRetentionDays int `gorm:"not null;default:90" json:"default_retention_days"`

	// DataResidencyRegion is v1 metadata only. Defaults to us-east-1;
	// EU-residency in v2 requires a separate regional deployment
	// (FR-050b).
	DataResidencyRegion string `gorm:"type:varchar(64);default:'us-east-1'" json:"data_residency_region"`

	CreatedAt time.Time `gorm:"index;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"index;not null" json:"updated_at"`
}

func (TableOrganization) TableName() string { return "ent_organizations" }
