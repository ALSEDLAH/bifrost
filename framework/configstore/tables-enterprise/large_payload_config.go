package tables_enterprise

import "time"

// LargePayloadConfigRowID is the singleton primary key.
const LargePayloadConfigRowID = "default"

// TableLargePayloadConfig persists transport-level large payload
// thresholds. Singleton — always read/written with ID="default".
//
// Per specs/006-large-payload-settings/plan.md.
type TableLargePayloadConfig struct {
	ID                     string `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Enabled                bool   `gorm:"not null;default:false" json:"enabled"`
	RequestThresholdBytes  int64  `gorm:"not null;default:0" json:"request_threshold_bytes"`
	ResponseThresholdBytes int64  `gorm:"not null;default:0" json:"response_threshold_bytes"`
	PrefetchSizeBytes      int64  `gorm:"not null;default:0" json:"prefetch_size_bytes"`
	MaxPayloadBytes        int64  `gorm:"not null;default:0" json:"max_payload_bytes"`
	TruncatedLogBytes      int64  `gorm:"not null;default:0" json:"truncated_log_bytes"`

	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (TableLargePayloadConfig) TableName() string { return "ent_large_payload_config" }
