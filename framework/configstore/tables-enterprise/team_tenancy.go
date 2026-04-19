package tables_enterprise

import "time"

// TableTeamTenancy is the 1:1 sidecar attaching tenancy to upstream's
// governance_teams (TableTeam). See virtual_key_tenancy.go for the
// pattern rationale (Principle XI rule 1).
type TableTeamTenancy struct {
	TeamID         string    `gorm:"primaryKey;type:varchar(255)" json:"team_id"`
	OrganizationID string    `gorm:"type:varchar(255);not null;index:idx_team_tenancy_org_ws" json:"organization_id"`
	WorkspaceID    string    `gorm:"type:varchar(255);not null;index:idx_team_tenancy_org_ws" json:"workspace_id"`
	CreatedAt      time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"not null" json:"updated_at"`
}

func (TableTeamTenancy) TableName() string { return "ent_team_tenancy" }
