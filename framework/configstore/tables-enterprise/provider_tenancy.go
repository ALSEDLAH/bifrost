package tables_enterprise

import "time"

// TableProviderTenancy is the 1:1 sidecar attaching tenancy to upstream's
// governance_providers (TableProvider). Provider configurations are
// typically org-scoped (a customer's OpenAI key is shared across that
// org's workspaces) so workspace_id is optional. When workspace_id is
// empty, the provider is shared org-wide; when set, it's restricted to
// that workspace.
type TableProviderTenancy struct {
	ProviderID     string    `gorm:"primaryKey;type:varchar(255)" json:"provider_id"`
	OrganizationID string    `gorm:"type:varchar(255);not null;index:idx_provider_tenancy_org_ws" json:"organization_id"`
	WorkspaceID    string    `gorm:"type:varchar(255);index:idx_provider_tenancy_org_ws" json:"workspace_id,omitempty"`
	CreatedAt      time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"not null" json:"updated_at"`
}

func (TableProviderTenancy) TableName() string { return "ent_provider_tenancy" }
