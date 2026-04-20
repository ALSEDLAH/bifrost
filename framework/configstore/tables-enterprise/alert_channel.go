package tables_enterprise

import "time"

// TableAlertChannel registers a destination (webhook / Slack) that
// receives governance events such as budget-threshold crossings.
//
// `Config` holds a type-specific JSON blob:
//   - type "webhook": {"url": string, "headers"?: map[string]string, "method"?: "POST"|"PUT"}
//   - type "slack":   {"webhook_url": string}
//
// Per specs/004-alert-channels/data-model.md.
type TableAlertChannel struct {
	ID      string `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name    string `gorm:"type:varchar(255);not null" json:"name"`
	Type    string `gorm:"type:varchar(50);not null" json:"type"`
	Config  string `gorm:"type:text;not null;column:config" json:"config"`
	Enabled bool   `gorm:"not null;default:true" json:"enabled"`

	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (TableAlertChannel) TableName() string { return "ent_alert_channels" }
