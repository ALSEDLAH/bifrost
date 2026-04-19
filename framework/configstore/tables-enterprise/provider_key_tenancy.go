package tables_enterprise

import "time"

// TableProviderKeyTenancy is the 1:1 sidecar attaching tenancy to
// upstream's governance_keys (TableKey). Provider keys inherit their
// org scope from the parent provider config but are tenanted explicitly
// here so cross-tenant lookups remain a single JOIN.
type TableProviderKeyTenancy struct {
	KeyID          string    `gorm:"primaryKey;type:varchar(255)" json:"key_id"`
	OrganizationID string    `gorm:"type:varchar(255);not null;index" json:"organization_id"`
	CreatedAt      time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"not null" json:"updated_at"`
}

func (TableProviderKeyTenancy) TableName() string { return "ent_provider_key_tenancy" }
