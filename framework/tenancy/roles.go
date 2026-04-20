// Role repository (US2, T050).
//
// CRUD for custom roles + user-role assignments, built on the GORM
// tables seeded by E004.

package tenancy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"gorm.io/gorm"
)

var (
	ErrRoleNotFound   = errors.New("tenancy: role not found")
	ErrRoleIsBuiltin  = errors.New("tenancy: cannot modify built-in role")
	ErrDuplicateRole  = errors.New("tenancy: role name already exists in this organization")
	ErrAssignmentExists = errors.New("tenancy: user already has a role in this scope")
)

// RoleRepo wraps role + user-role-assignment persistence.
type RoleRepo struct {
	db *gorm.DB
}

// NewRoleRepo constructs the repo.
func NewRoleRepo(db *gorm.DB) *RoleRepo {
	return &RoleRepo{db: db}
}

// ListRoles returns all roles for the given organization.
func (r *RoleRepo) ListRoles(ctx context.Context, orgID string) ([]tables_enterprise.TableRole, error) {
	var roles []tables_enterprise.TableRole
	if err := r.db.WithContext(ctx).Where("organization_id = ?", orgID).
		Order("is_builtin DESC, name ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// GetRole returns a single role by ID.
func (r *RoleRepo) GetRole(ctx context.Context, roleID string) (*tables_enterprise.TableRole, error) {
	var role tables_enterprise.TableRole
	if err := r.db.WithContext(ctx).Where("id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return &role, nil
}

// CreateRoleInput is the input for creating a custom role.
type CreateRoleInput struct {
	OrganizationID string
	Name           string
	Scopes         map[string][]string // resource → verbs
}

// CreateRole creates a custom (non-builtin) role.
func (r *RoleRepo) CreateRole(ctx context.Context, in CreateRoleInput) (*tables_enterprise.TableRole, error) {
	// Check for duplicate name.
	var existing tables_enterprise.TableRole
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND name = ?", in.OrganizationID, in.Name).
		First(&existing).Error
	if err == nil {
		return nil, ErrDuplicateRole
	}

	scopeJSON, err := json.Marshal(in.Scopes)
	if err != nil {
		return nil, fmt.Errorf("marshal scopes: %w", err)
	}

	// Flatten scopes for bitmask.
	var flat []string
	for res, verbs := range in.Scopes {
		for _, v := range verbs {
			flat = append(flat, res+":"+v)
		}
	}

	role := tables_enterprise.TableRole{
		ID:             uuid.NewString(),
		OrganizationID: in.OrganizationID,
		Name:           in.Name,
		ScopeJSON:      string(scopeJSON),
		ScopeBitmask:   int64(0),
		IsBuiltin:      false,
		CreatedAt:      time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// UpdateRoleInput is the input for updating a custom role.
type UpdateRoleInput struct {
	Name   *string
	Scopes *map[string][]string
}

// UpdateRole updates a custom role. Built-in roles cannot be modified.
func (r *RoleRepo) UpdateRole(ctx context.Context, roleID string, in UpdateRoleInput) (*tables_enterprise.TableRole, error) {
	role, err := r.GetRole(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if role.IsBuiltin {
		return nil, ErrRoleIsBuiltin
	}

	updates := map[string]any{}
	if in.Name != nil {
		updates["name"] = *in.Name
	}
	if in.Scopes != nil {
		scopeJSON, err := json.Marshal(*in.Scopes)
		if err != nil {
			return nil, fmt.Errorf("marshal scopes: %w", err)
		}
		updates["scope_json"] = string(scopeJSON)

		var flat []string
		for res, verbs := range *in.Scopes {
			for _, v := range verbs {
				flat = append(flat, res+":"+v)
			}
		}
		updates["scope_bitmask"] = int64(0)
	}

	if len(updates) > 0 {
		if err := r.db.WithContext(ctx).Model(role).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return r.GetRole(ctx, roleID)
}

// DeleteRole deletes a custom role. Built-in roles cannot be deleted.
func (r *RoleRepo) DeleteRole(ctx context.Context, roleID string) error {
	role, err := r.GetRole(ctx, roleID)
	if err != nil {
		return err
	}
	if role.IsBuiltin {
		return ErrRoleIsBuiltin
	}
	// Remove all assignments first.
	if err := r.db.WithContext(ctx).
		Where("role_id = ?", roleID).
		Delete(&tables_enterprise.TableUserRoleAssignment{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Delete(role).Error
}

// AssignRoleInput is the input for assigning a role to a user.
type AssignRoleInput struct {
	UserID      string
	RoleID      string
	WorkspaceID string // empty = org-wide
	AssignedBy  string
}

// AssignRole assigns a role to a user.
func (r *RoleRepo) AssignRole(ctx context.Context, in AssignRoleInput) (*tables_enterprise.TableUserRoleAssignment, error) {
	// Check role exists.
	if _, err := r.GetRole(ctx, in.RoleID); err != nil {
		return nil, err
	}

	assignment := tables_enterprise.TableUserRoleAssignment{
		ID:          uuid.NewString(),
		UserID:      in.UserID,
		RoleID:      in.RoleID,
		WorkspaceID: in.WorkspaceID,
		AssignedAt:  time.Now().UTC(),
		AssignedBy:  in.AssignedBy,
	}
	if err := r.db.WithContext(ctx).Create(&assignment).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

// UnassignRole removes a role assignment.
func (r *RoleRepo) UnassignRole(ctx context.Context, assignmentID string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", assignmentID).
		Delete(&tables_enterprise.TableUserRoleAssignment{}).Error
}

// ListAssignments returns all role assignments for a user.
func (r *RoleRepo) ListAssignments(ctx context.Context, userID string) ([]tables_enterprise.TableUserRoleAssignment, error) {
	var assignments []tables_enterprise.TableUserRoleAssignment
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

// ParseScopeJSON parses a ScopeJSON string into a flat scope list
// (e.g. ["metrics:read", "prompts:write"]).
func ParseScopeJSON(scopeJSON string) ([]string, error) {
	if scopeJSON == "" {
		return nil, nil
	}
	var m map[string][]string
	if err := json.Unmarshal([]byte(scopeJSON), &m); err != nil {
		return nil, err
	}
	var scopes []string
	for res, verbs := range m {
		if res == "*" {
			return []string{"*"}, nil
		}
		for _, v := range verbs {
			scopes = append(scopes, res+":"+v)
		}
	}
	return scopes, nil
}
