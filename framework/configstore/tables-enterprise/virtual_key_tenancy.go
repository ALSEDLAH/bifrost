package tables_enterprise

import "time"

// TableVirtualKeyTenancy is the 1:1 sidecar that attaches enterprise
// tenancy (organization_id + workspace_id) to upstream's
// governance_virtual_keys (TableVirtualKey) without modifying upstream.
//
// Constitution Principle XI rule 1 — sibling-file extension only;
// upstream's framework/configstore/tables/key.go is not edited.
//
// Reads JOIN upstream rows ON virtual_key_id; rows without a sidecar
// fall into the synthetic default organization (which always has a
// sidecar row written at first-boot E001 / E002 migration time).
//
// If upstream later accepts a PR adding these columns to TableVirtualKey,
// the sidecar collapses into the main struct via E0XX_collapse_*
// migration in a v1.7+ release (Principle XI rule 7).
type TableVirtualKeyTenancy struct {
	// VirtualKeyID is the FK to governance_virtual_keys.id and the PK
	// of this 1:1 sidecar.
	VirtualKeyID string `gorm:"primaryKey;type:varchar(255)" json:"virtual_key_id"`

	// OrganizationID is the resolved organization for this VK. Required.
	OrganizationID string `gorm:"type:varchar(255);not null;index:idx_vk_tenancy_org_ws" json:"organization_id"`

	// WorkspaceID is the resolved workspace. Required for VKs (every VK
	// belongs to exactly one workspace).
	WorkspaceID string `gorm:"type:varchar(255);not null;index:idx_vk_tenancy_org_ws" json:"workspace_id"`

	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (TableVirtualKeyTenancy) TableName() string { return "ent_virtual_key_tenancy" }
