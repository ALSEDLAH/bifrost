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

	// HMAC chain for tamper evidence (spec 015). Empty when the
	// BIFROST_AUDIT_HMAC_KEY env var is unset (backwards compatible).
	// HMAC = HMAC-SHA256(key, PrevHMAC || canonical_bytes(row)) in hex.
	HMAC     string `gorm:"type:varchar(128)" json:"hmac,omitempty"`
	PrevHMAC string `gorm:"type:varchar(128)" json:"prev_hmac,omitempty"`
}

func (TableAuditEntry) TableName() string { return "ent_audit_entries" }

// CanonicalBytes serialises the row into a deterministic byte slice
// that feeds HMAC computation (spec 015). HMAC / PrevHMAC themselves
// are deliberately excluded — they're the output of this function.
func (e TableAuditEntry) CanonicalBytes() []byte {
	// Use a pipe-joined layout with byte-safe separators. Values
	// that can contain pipes (Reason, BeforeJSON, AfterJSON) are
	// safe because we don't parse this format — we only hash it.
	// Adding a new field requires a new canonical layout + key
	// rotation; doc this in the spec's Out-of-Scope if hit.
	fields := []string{
		e.ID,
		e.OrganizationID,
		e.WorkspaceID,
		e.ActorType,
		e.ActorID,
		e.ActorDisplay,
		e.ActorIP,
		e.Action,
		e.ResourceType,
		e.ResourceID,
		e.Outcome,
		e.Reason,
		e.BeforeJSON,
		e.AfterJSON,
		e.RequestID,
		e.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	// Crude join — pipes appearing inside values are tolerated
	// because the hash is over exact bytes.
	total := 0
	for _, f := range fields {
		total += len(f) + 1
	}
	out := make([]byte, 0, total)
	for i, f := range fields {
		if i > 0 {
			out = append(out, '|')
		}
		out = append(out, f...)
	}
	return out
}
