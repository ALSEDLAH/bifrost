package tables_enterprise

import "time"

// TableUserRoleAssignment links a user to a role, optionally scoped to
// a specific workspace. When WorkspaceID is empty, the assignment is
// org-level (applies across all workspaces in the user's org).
//
// Per data-model §1.
type TableUserRoleAssignment struct {
	ID     string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	UserID string `gorm:"type:varchar(255);not null;index:idx_ura_user_ws,unique" json:"user_id"`
	RoleID string `gorm:"type:varchar(255);not null;index" json:"role_id"`

	// WorkspaceID may be empty to mean "org-wide". The unique index
	// ensures at most one role per (user, workspace) pair.
	WorkspaceID string `gorm:"type:varchar(255);index:idx_ura_user_ws,unique" json:"workspace_id,omitempty"`

	AssignedAt time.Time `gorm:"not null" json:"assigned_at"`
	AssignedBy string    `gorm:"type:varchar(255);index" json:"assigned_by,omitempty"`
}

func (TableUserRoleAssignment) TableName() string { return "ent_user_role_assignments" }
