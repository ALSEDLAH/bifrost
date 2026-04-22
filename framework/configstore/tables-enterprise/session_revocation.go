// Per-user session revocation row (spec 026). One row per user_id;
// repeated revoke events update revoked_at in place.

package tables_enterprise

import "time"

type TableSessionRevocation struct {
	UserID     string    `gorm:"primaryKey;type:varchar(255)" json:"user_id"`
	RevokedAt  time.Time `gorm:"not null" json:"revoked_at"`
}

func (TableSessionRevocation) TableName() string { return "ent_session_revocations" }
