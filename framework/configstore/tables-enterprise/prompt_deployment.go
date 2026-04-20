package tables_enterprise

import "time"

// TablePromptDeployment labels a specific prompt version as the
// production / staging / etc. release for a given prompt. Composite
// primary key on (prompt_id, label).
//
// Per specs/011-prompt-deployments/spec.md. v1 stores labels; runtime
// resolution (promptName → labeled version) is phase 2.
type TablePromptDeployment struct {
	PromptID  string `gorm:"primaryKey;type:varchar(36)" json:"prompt_id"`
	Label     string `gorm:"primaryKey;type:varchar(50)" json:"label"`
	VersionID uint   `gorm:"not null" json:"version_id"`

	PromotedBy string    `gorm:"type:varchar(255)" json:"promoted_by"`
	PromotedAt time.Time `gorm:"not null" json:"promoted_at"`
}

func (TablePromptDeployment) TableName() string { return "ent_prompt_deployments" }
