// Enterprise tables for logstore.
//
// Post-audit revision: TableLogTenancy sidecar REMOVED — upstream logs
// don't need tenancy sidecars (scoping via VK → Team → Customer chain).

package logstore

import "time"

// TableAuditEntry is the immutable audit trail (US4 / FR-010..FR-012).
// Foundational — every enterprise plugin emits audit entries from day one
// (Constitution Principle VI).
type TableAuditEntry struct {
	ID             string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	OrganizationID string `gorm:"type:varchar(255);not null;index:idx_audit_org_created" json:"organization_id"`
	WorkspaceID    string `gorm:"type:varchar(255);index" json:"workspace_id,omitempty"`

	ActorType    string `gorm:"type:varchar(64);not null;index" json:"actor_type"` // user|admin_api_key|service_account|system
	ActorID      string `gorm:"type:varchar(255);index" json:"actor_id,omitempty"`
	ActorDisplay string `gorm:"type:varchar(255)" json:"actor_display"`
	ActorIP      string `gorm:"type:varchar(64)" json:"actor_ip,omitempty"`

	Action       string `gorm:"type:varchar(255);not null;index" json:"action"`
	ResourceType string `gorm:"type:varchar(128);not null;index:idx_audit_resource" json:"resource_type"`
	ResourceID   string `gorm:"type:varchar(255);index:idx_audit_resource" json:"resource_id,omitempty"`

	Outcome string `gorm:"type:varchar(32);not null;index" json:"outcome"` // allowed|denied|error
	Reason  string `gorm:"type:text" json:"reason,omitempty"`

	BeforeJSON string `gorm:"type:text" json:"before_json,omitempty"`
	AfterJSON  string `gorm:"type:text" json:"after_json,omitempty"`

	RequestID string    `gorm:"type:varchar(255);index" json:"request_id,omitempty"`
	CreatedAt time.Time `gorm:"index:idx_audit_org_created;not null" json:"created_at"`
}

func (TableAuditEntry) TableName() string { return "ent_audit_entries" }
