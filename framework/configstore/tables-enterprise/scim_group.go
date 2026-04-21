// SCIM Group tables (spec 022). Group identity-grouping pushed by
// IdPs (Azure AD / Okta / JumpCloud). These are NOT the upstream
// governance_teams — those are billing entities.

package tables_enterprise

import "time"

// TableSCIMGroup stores IdP-pushed groups. Members are separated out
// into TableSCIMGroupMember to let membership PATCHes add/remove
// without rewriting the whole group row.
type TableSCIMGroup struct {
	ID             string  `gorm:"primaryKey;type:varchar(255)" json:"id"`
	OrganizationID string  `gorm:"type:varchar(255);not null;index:idx_scim_groups_org_external,unique,where:external_id IS NOT NULL" json:"organization_id"`
	DisplayName    string  `gorm:"type:varchar(255);not null;index" json:"display_name"`
	ExternalID     *string `gorm:"type:varchar(255);index:idx_scim_groups_org_external,unique,where:external_id IS NOT NULL" json:"external_id,omitempty"`

	CreatedAt time.Time `gorm:"index;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (TableSCIMGroup) TableName() string { return "ent_scim_groups" }

// TableSCIMGroupMember is the junction table. Composite primary key
// (group_id, user_id) prevents duplicate member entries.
type TableSCIMGroupMember struct {
	GroupID   string    `gorm:"primaryKey;type:varchar(255)" json:"group_id"`
	UserID    string    `gorm:"primaryKey;type:varchar(255);index" json:"user_id"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}

func (TableSCIMGroupMember) TableName() string { return "ent_scim_group_members" }
