package tables_enterprise

import "time"

// TableCustomerTenancy is the 1:1 sidecar attaching tenancy to upstream's
// governance_customers (TableCustomer). Customers are typically managed
// at the workspace level so workspace_id is required.
type TableCustomerTenancy struct {
	CustomerID     string    `gorm:"primaryKey;type:varchar(255)" json:"customer_id"`
	OrganizationID string    `gorm:"type:varchar(255);not null;index:idx_customer_tenancy_org_ws" json:"organization_id"`
	WorkspaceID    string    `gorm:"type:varchar(255);not null;index:idx_customer_tenancy_org_ws" json:"workspace_id"`
	CreatedAt      time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"not null" json:"updated_at"`
}

func (TableCustomerTenancy) TableName() string { return "ent_customer_tenancy" }
