package tables_enterprise

import "time"

// TableGuardrailProvider registers a content-safety provider the admin
// can reference from rules (spec 010).
//
// `Config` JSON (type-dependent):
//   - "openai-moderation": {"api_key": "...", "base_url"?: "..."}
//   - "regex":             {} (no provider-side config)
//   - "custom-webhook":    {"url": "...", "headers"?: {...}}
type TableGuardrailProvider struct {
	ID      string `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name    string `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	Type    string `gorm:"type:varchar(50);not null" json:"type"`
	Config  string `gorm:"type:text;not null;column:config" json:"config"`
	Enabled bool   `gorm:"not null;default:true" json:"enabled"`

	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (TableGuardrailProvider) TableName() string { return "ent_guardrail_providers" }

// TableGuardrailRule is evaluated against inference input / output.
//
// ProviderID may be empty for regex-only rules; non-empty rules delegate
// to the referenced provider. `Pattern` is type-dependent — a regex
// literal for regex rules, or an optional category filter for the
// provider types.
type TableGuardrailRule struct {
	ID         string `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name       string `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	ProviderID string `gorm:"type:varchar(36);index" json:"provider_id"`
	Trigger    string `gorm:"type:varchar(20);not null" json:"trigger"` // "input" | "output" | "both"
	Action     string `gorm:"type:varchar(20);not null" json:"action"`  // "block" | "flag" | "log"
	Pattern    string `gorm:"type:text" json:"pattern"`
	Enabled    bool   `gorm:"not null;default:true" json:"enabled"`

	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

func (TableGuardrailRule) TableName() string { return "ent_guardrail_rules" }
