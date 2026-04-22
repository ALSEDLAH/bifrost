// Audit log retention config (spec 027). Singleton row pinned by
// AuditRetentionSingletonID. Prune jobs read this to decide whether
// to run and what cutoff to apply.

package tables_enterprise

import "time"

const AuditRetentionSingletonID = "ent_audit_retention_singleton"

type TableAuditRetentionConfig struct {
	ID             string     `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Enabled        bool       `gorm:"not null;default:false" json:"enabled"`
	RetentionDays  int        `gorm:"not null;default:90" json:"retention_days"`
	LastPrunedAt   *time.Time `json:"last_pruned_at,omitempty"`
	UpdatedAt      time.Time  `gorm:"not null" json:"updated_at"`
}

func (TableAuditRetentionConfig) TableName() string { return "ent_audit_retention_config" }
