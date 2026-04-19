package tables_enterprise

import "time"

// TableUser is a person authenticated into the system. Linked to an
// identity-provider subject (SSO) when the org has SSO configured;
// local credentials are break-glass only (Principle VII, R-12).
//
// Per data-model §1.
type TableUser struct {
	ID             string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	OrganizationID string `gorm:"type:varchar(255);not null;index:idx_users_org_email,unique" json:"organization_id"`

	Email       string `gorm:"type:varchar(255);not null;index:idx_users_org_email,unique" json:"email"`
	DisplayName string `gorm:"type:varchar(255)" json:"display_name,omitempty"`

	// IdpSubject is the SSO subject claim. Unique per org when set.
	IdpSubject string `gorm:"type:varchar(255);index:idx_users_org_subject,unique" json:"idp_subject,omitempty"`

	// LocalPasswordHash is the argon2id hash for break-glass logins.
	// Encrypted at rest via framework/crypto. Never logged or exported.
	LocalPasswordHash string `gorm:"type:text" json:"-"`

	// MFASecret is the TOTP seed for break-glass accounts; encrypted.
	MFASecret string `gorm:"type:text" json:"-"`

	// Status: active, suspended, pending.
	// "pending" means the user was invited but hasn't logged in yet;
	// first SSO login auto-promotes them to active (FR-006).
	Status string `gorm:"type:varchar(32);not null;default:'pending'" json:"status"`

	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"index;not null" json:"updated_at"`
}

func (TableUser) TableName() string { return "ent_users" }
