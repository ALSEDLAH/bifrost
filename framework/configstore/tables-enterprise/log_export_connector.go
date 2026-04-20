package tables_enterprise

import "time"

// TableLogExportConnector persists credentials for downstream log
// export destinations. The actual forwarding pipeline is a separate
// phase-2 plugin that reads these rows on boot.
//
// `Config` holds a type-specific JSON blob:
//   - type "datadog":  {"api_key": string, "site": string, "tags"?: map}
//   - type "bigquery": {"project_id": string, "dataset": string,
//                       "table": string, "credentials_json": string}
//
// Per specs/008-log-export-connectors/spec.md.
type TableLogExportConnector struct {
	ID      string `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Type    string `gorm:"type:varchar(50);not null;index" json:"type"`
	Name    string `gorm:"type:varchar(255);not null" json:"name"`
	Config  string `gorm:"type:text;not null;column:config" json:"config"`
	Enabled bool   `gorm:"not null;default:true" json:"enabled"`

	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (TableLogExportConnector) TableName() string { return "ent_log_export_connectors" }
