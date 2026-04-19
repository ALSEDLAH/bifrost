package tables_enterprise

import "time"

// TableRole is a named collection of scopes. Four built-in roles
// (Owner, Admin, Member, Manager) are seeded by the E004 migration.
// Customers can create unlimited custom roles with arbitrary scope
// lists (US2).
//
// Per data-model §1.
type TableRole struct {
	ID             string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	OrganizationID string `gorm:"type:varchar(255);not null;index:idx_roles_org_name,unique" json:"organization_id"`

	Name string `gorm:"type:varchar(255);not null;index:idx_roles_org_name,unique" json:"name"`

	// ScopeBitmask is the compact representation — bit N = resource N +
	// verb (resource_index*3 + verb_index). Populated by T049 when the
	// scope registry is declared; for now the authoritative list lives
	// in ScopeJSON.
	ScopeBitmask int64 `gorm:"not null;default:0" json:"scope_bitmask"`

	// ScopeJSON is the forward-compatible canonical scope list:
	// {"resource": ["verb1", "verb2"]}. Example built-in Admin role:
	// {"metrics": ["read","write","delete"], "prompts": ["read","write","delete"], ...}.
	ScopeJSON string `gorm:"type:text;not null" json:"scope_json"`

	// IsBuiltin is true for Owner/Admin/Member/Manager. UI marks these
	// non-editable; deletion is forbidden.
	IsBuiltin bool `gorm:"not null;default:false" json:"is_builtin"`

	CreatedAt time.Time `gorm:"index;not null" json:"created_at"`
}

func (TableRole) TableName() string { return "ent_roles" }
