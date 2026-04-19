// Enterprise sidecar + new tables for logstore.
//
// Constitution Principle XI rule 1 — sibling-file extension. Upstream's
// framework/logstore/tables.go is not modified. Enterprise tables live
// in this single file (logstore convention is one file for tables, vs
// configstore's per-table directory).
//
// Each enterprise log-store table uses the prefix "ent_" to make it
// visually distinct in any DB tooling.

package logstore

import "time"

// TableLogTenancy is the 1:1 sidecar attaching enterprise tenancy to
// upstream's logs table (Log struct). Reads JOIN logs ON log_id; rows
// without a sidecar fall into the synthetic default org (sidecar row
// written at first-boot E003 migration time).
type TableLogTenancy struct {
	// LogID is the FK to logs.id and the PK of this 1:1 sidecar.
	// logs uses string IDs so we match the type.
	LogID string `gorm:"primaryKey;type:varchar(255)" json:"log_id"`

	OrganizationID string `gorm:"type:varchar(255);not null;index:idx_log_tenancy_org_ws" json:"organization_id"`
	WorkspaceID    string `gorm:"type:varchar(255);not null;index:idx_log_tenancy_org_ws" json:"workspace_id"`

	// VirtualKeyID is denormalized from the upstream log row for
	// per-key analytics queries that don't want to JOIN governance_*.
	VirtualKeyID string `gorm:"type:varchar(255);index" json:"virtual_key_id,omitempty"`

	CreatedAt time.Time `gorm:"index;not null" json:"created_at"`
}

func (TableLogTenancy) TableName() string { return "ent_log_tenancy" }

// TableAuditEntry is the immutable audit trail (US4 / FR-010..FR-012).
// Created here in Phase 2 (not US4) because audit is foundational —
// every other enterprise plugin emits audit entries from day one
// (Constitution Principle VI). The US4 task in tasks.md narrows to
// "build the UI + handlers"; the table itself lands here.
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

	// Before/After snapshots are JSON-serialized at emit time. May be
	// encrypted by framework/crypto when they contain sensitive fields.
	BeforeJSON string `gorm:"type:text" json:"before_json,omitempty"`
	AfterJSON  string `gorm:"type:text" json:"after_json,omitempty"`

	RequestID string    `gorm:"type:varchar(255);index" json:"request_id,omitempty"`
	CreatedAt time.Time `gorm:"index:idx_audit_org_created;not null" json:"created_at"`
}

func (TableAuditEntry) TableName() string { return "ent_audit_entries" }
