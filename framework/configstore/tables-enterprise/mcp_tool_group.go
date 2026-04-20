package tables_enterprise

import "time"

// TableMCPToolGroup is a named collection of MCP tool references. v1
// is labels-only (no VK/team gating — that's a follow-up spec). The
// `Tools` column is a JSON array of {mcp_client_id, tool_name} pairs.
//
// Per specs/005-mcp-tool-groups/plan.md.
type TableMCPToolGroup struct {
	ID          string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Tools       string    `gorm:"type:text;column:tools" json:"tools"` // JSON []MCPToolRef
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null" json:"updated_at"`
}

func (TableMCPToolGroup) TableName() string { return "ent_mcp_tool_groups" }
