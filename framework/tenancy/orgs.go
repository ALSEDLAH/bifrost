// Organization repository — CRUD helpers for ent_organizations.
//
// All read paths enforce that the caller can only see their own
// organization. Cross-org reads are blocked at the framework level,
// not just the handler — defence in depth (Principle V).

package tenancy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"gorm.io/gorm"
)

// ErrOrgNotFound is returned when a lookup misses.
var ErrOrgNotFound = errors.New("tenancy: organization not found")

// OrgRepo is the repository handle for organization operations.
type OrgRepo struct {
	db *gorm.DB
}

// NewOrgRepo constructs a repository bound to a configstore *gorm.DB.
func NewOrgRepo(db *gorm.DB) *OrgRepo { return &OrgRepo{db: db} }

// GetDefault returns the single is_default=true organization row.
// Used by single-org-mode code paths that resolve the "current" org
// without needing a request context.
func (r *OrgRepo) GetDefault(ctx context.Context) (*tables_enterprise.TableOrganization, error) {
	var row tables_enterprise.TableOrganization
	err := r.db.WithContext(ctx).Where("is_default = ?", true).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("orgs.GetDefault: %w", err)
	}
	return &row, nil
}

// GetByID fetches an organization by its UUID. Used by the
// /admin/organizations/current endpoint after the caller's
// TenantContext has already authorized them for that org.
func (r *OrgRepo) GetByID(ctx context.Context, id string) (*tables_enterprise.TableOrganization, error) {
	var row tables_enterprise.TableOrganization
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("orgs.GetByID: %w", err)
	}
	return &row, nil
}

// Update applies partial changes to an organization. Accepts a map so
// the caller can supply only the fields they want to mutate (avoids
// overwriting unrelated columns from a stale struct).
func (r *OrgRepo) Update(ctx context.Context, id string, changes map[string]any) (*tables_enterprise.TableOrganization, error) {
	if len(changes) == 0 {
		return r.GetByID(ctx, id)
	}
	changes["updated_at"] = time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&tables_enterprise.TableOrganization{}).
		Where("id = ?", id).
		Updates(changes).Error; err != nil {
		return nil, fmt.Errorf("orgs.Update: %w", err)
	}
	return r.GetByID(ctx, id)
}

// CreateMultiOrg creates a new organization. Only used in cloud mode
// (multi_org_enabled=true). In single-org mode callers must use the
// default row seeded by E004.
func (r *OrgRepo) CreateMultiOrg(ctx context.Context, name string) (*tables_enterprise.TableOrganization, error) {
	now := time.Now().UTC()
	org := tables_enterprise.TableOrganization{
		ID:                   uuid.NewString(),
		Name:                 name,
		IsDefault:            false,
		DefaultRetentionDays: 90,
		DataResidencyRegion:  "us-east-1",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := r.db.WithContext(ctx).Create(&org).Error; err != nil {
		return nil, fmt.Errorf("orgs.Create: %w", err)
	}
	return &org, nil
}
