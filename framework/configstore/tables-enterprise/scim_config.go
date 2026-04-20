package tables_enterprise

import "time"

// SCIMConfigRowID is the singleton primary key.
const SCIMConfigRowID = "default"

// TableSCIMConfig persists SCIM provisioning state. Singleton.
//
// Only the sha256 hash of the bearer token is stored; the plaintext
// is returned once from the rotate endpoint. `TokenPrefix` is the
// first 8 chars of the plaintext, kept for UX (e.g., "scim_abcd…") so
// admins can distinguish tokens.
//
// Per specs/009-scim-provisioning/spec.md.
type TableSCIMConfig struct {
	ID               string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Enabled          bool       `gorm:"not null;default:false" json:"enabled"`
	BearerTokenHash  string     `gorm:"type:varchar(128)" json:"-"`
	TokenPrefix      string     `gorm:"type:varchar(32)" json:"token_prefix"`
	TokenCreatedAt   *time.Time `json:"token_created_at,omitempty"`
	CreatedAt        time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"not null" json:"updated_at"`
}

func (TableSCIMConfig) TableName() string { return "ent_scim_config" }
