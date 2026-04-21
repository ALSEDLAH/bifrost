// SSO OIDC configuration (spec 023). Singleton row pinned by the
// constant SSOConfigSingletonID. The client secret is stored
// encrypted-at-rest using the same envelope strategy as other
// secret-bearing tables (encryption performed by the handler layer).

package tables_enterprise

import "time"

const SSOConfigSingletonID = "ent_sso_config_singleton"

type TableSSOConfig struct {
	ID                     string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Enabled                bool   `gorm:"not null;default:false" json:"enabled"`
	Issuer                 string `gorm:"type:varchar(512)" json:"issuer"`
	ClientID               string `gorm:"type:varchar(512)" json:"client_id"`
	ClientSecretEncrypted  string `gorm:"type:text" json:"-"`
	RedirectURI            string `gorm:"type:varchar(512)" json:"redirect_uri"`
	AllowedEmailDomainsCSV string `gorm:"type:text" json:"-"`
	JITProvisioning        bool   `gorm:"not null;default:false" json:"jit_provisioning"`
	UpdatedAt              time.Time `gorm:"not null" json:"updated_at"`
}

func (TableSSOConfig) TableName() string { return "ent_sso_config" }
