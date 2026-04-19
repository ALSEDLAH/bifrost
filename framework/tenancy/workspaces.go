// Workspace repository — CRUD with soft-delete semantics.
//
// Every read and list is implicitly scoped by organization_id via the
// supplied TenantContext. Soft-delete with a 30-day grace period per
// spec US1 edge case: a deleted workspace is hidden from the UI
// immediately, its virtual keys are revoked, but rows are physically
// removed only after 30 days — so accidental deletions can be rolled
// back by clearing deleted_at within the grace window.

package tenancy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"gorm.io/gorm"
)

// ErrWorkspaceNotFound is returned when a lookup misses (including
// hits against soft-deleted rows, which are hidden from reads).
var ErrWorkspaceNotFound = errors.New("tenancy: workspace not found")

// ErrWorkspaceSlugConflict is returned when Create gets a slug that
// already exists within the same organization.
var ErrWorkspaceSlugConflict = errors.New("tenancy: workspace slug already exists in this organization")

// WorkspaceRepo is the repository handle for workspace operations.
type WorkspaceRepo struct {
	db *gorm.DB
}

// NewWorkspaceRepo constructs a repository bound to a configstore *gorm.DB.
func NewWorkspaceRepo(db *gorm.DB) *WorkspaceRepo { return &WorkspaceRepo{db: db} }

// List returns all non-deleted workspaces in the given organization.
// Soft-deleted workspaces (deleted_at != nil) are excluded.
func (r *WorkspaceRepo) List(ctx context.Context, orgID string) ([]tables_enterprise.TableWorkspace, error) {
	var rows []tables_enterprise.TableWorkspace
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND deleted_at IS NULL", orgID).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("workspaces.List: %w", err)
	}
	return rows, nil
}

// Get fetches a single workspace by ID, scoped to the caller's org.
// Returns ErrWorkspaceNotFound for both "does not exist" and "exists
// in a different org" — we never leak cross-org existence.
func (r *WorkspaceRepo) Get(ctx context.Context, orgID, id string) (*tables_enterprise.TableWorkspace, error) {
	var row tables_enterprise.TableWorkspace
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workspaces.Get: %w", err)
	}
	return &row, nil
}

// Create inserts a new workspace. Returns ErrWorkspaceSlugConflict
// when the (organization_id, slug) pair already exists.
func (r *WorkspaceRepo) Create(ctx context.Context, orgID, name, slug, description string) (*tables_enterprise.TableWorkspace, error) {
	slug = normalizeSlug(slug)
	if slug == "" {
		return nil, errors.New("workspaces.Create: slug is required")
	}

	// Probe for conflict first — GORM's error surface on a UNIQUE
	// violation is dialect-dependent (PostgreSQL returns 23505,
	// SQLite returns 2067), so an explicit existence check gives us
	// a clean typed error regardless of backend.
	var existing tables_enterprise.TableWorkspace
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND slug = ?", orgID, slug).
		First(&existing).Error
	if err == nil {
		return nil, ErrWorkspaceSlugConflict
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("workspaces.Create (conflict check): %w", err)
	}

	now := time.Now().UTC()
	ws := tables_enterprise.TableWorkspace{
		ID:             uuid.NewString(),
		OrganizationID: orgID,
		Name:           name,
		Slug:           slug,
		Description:    description,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := r.db.WithContext(ctx).Create(&ws).Error; err != nil {
		return nil, fmt.Errorf("workspaces.Create: %w", err)
	}
	return &ws, nil
}

// Patch applies partial updates. Accepts a map for selective mutation.
// The caller must pre-validate field names.
func (r *WorkspaceRepo) Patch(ctx context.Context, orgID, id string, changes map[string]any) (*tables_enterprise.TableWorkspace, error) {
	if len(changes) == 0 {
		return r.Get(ctx, orgID, id)
	}
	changes["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&tables_enterprise.TableWorkspace{}).
		Where("organization_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Updates(changes)
	if result.Error != nil {
		return nil, fmt.Errorf("workspaces.Patch: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrWorkspaceNotFound
	}
	return r.Get(ctx, orgID, id)
}

// SoftDelete marks a workspace deleted (sets deleted_at). The
// workspace's virtual keys and resources are revoked by a separate
// sweep invoked from the US1 handler; the repo only handles the
// deleted_at flip. Physical deletion happens via the retention job
// after 30 days (spec US1 edge case).
func (r *WorkspaceRepo) SoftDelete(ctx context.Context, orgID, id string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&tables_enterprise.TableWorkspace{}).
		Where("organization_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Updates(map[string]any{
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("workspaces.SoftDelete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWorkspaceNotFound
	}
	return nil
}

// Restore reverses a soft-delete within the 30-day grace window.
// Returns ErrWorkspaceNotFound if the workspace is hard-deleted,
// doesn't exist, or was never soft-deleted.
func (r *WorkspaceRepo) Restore(ctx context.Context, orgID, id string) error {
	result := r.db.WithContext(ctx).
		Model(&tables_enterprise.TableWorkspace{}).
		Where("organization_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
		Updates(map[string]any{
			"deleted_at": nil,
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("workspaces.Restore: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWorkspaceNotFound
	}
	return nil
}

// normalizeSlug lower-cases and trims. We don't do full RFC-3986
// URL-slug validation here; the handler rejects obviously-bad input
// before calling the repo.
func normalizeSlug(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
