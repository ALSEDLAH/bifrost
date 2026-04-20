// RBAC handlers — /api/rbac/* endpoints (US2, T032/T033).
//
// Exposes the tenancy.RoleRepo + user/assignment CRUD so the UI can
// manage roles, users and role assignments. In v1 single-org mode the
// handler always operates under the synthetic default organization
// resolved from ent_system_defaults; when TenantContext middleware is
// wired later, the handler will honor the resolved org.

package handlers

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/fasthttp/router"
	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

// RBACHandler serves /api/rbac/*.
type RBACHandler struct {
	db     *gorm.DB
	roles  *tenancy.RoleRepo
	logger schemas.Logger
}

// NewRBACHandler constructs the handler.
func NewRBACHandler(db *gorm.DB, logger schemas.Logger) *RBACHandler {
	return &RBACHandler{db: db, roles: tenancy.NewRoleRepo(db), logger: logger}
}

// RegisterRoutes wires the RBAC endpoints.
func (h *RBACHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/rbac/meta", lib.ChainMiddlewares(h.handleMeta, middlewares...))
	r.GET("/api/rbac/me", lib.ChainMiddlewares(h.handleMe, middlewares...))

	r.GET("/api/rbac/roles", lib.ChainMiddlewares(h.listRoles, middlewares...))
	r.POST("/api/rbac/roles", lib.ChainMiddlewares(h.createRole, middlewares...))
	r.PATCH("/api/rbac/roles/{id}", lib.ChainMiddlewares(h.updateRole, middlewares...))
	r.DELETE("/api/rbac/roles/{id}", lib.ChainMiddlewares(h.deleteRole, middlewares...))

	r.GET("/api/rbac/users", lib.ChainMiddlewares(h.listUsers, middlewares...))
	r.POST("/api/rbac/users", lib.ChainMiddlewares(h.createUser, middlewares...))
	r.PATCH("/api/rbac/users/{id}", lib.ChainMiddlewares(h.updateUser, middlewares...))
	r.DELETE("/api/rbac/users/{id}", lib.ChainMiddlewares(h.deleteUser, middlewares...))
	r.GET("/api/rbac/users/{id}/assignments", lib.ChainMiddlewares(h.listAssignments, middlewares...))

	r.POST("/api/rbac/assignments", lib.ChainMiddlewares(h.assignRole, middlewares...))
	r.DELETE("/api/rbac/assignments/{id}", lib.ChainMiddlewares(h.unassignRole, middlewares...))
}

// defaultOrgID resolves the synthetic default org for single-org v1 mode.
func (h *RBACHandler) defaultOrgID() (string, error) {
	var sd tables_enterprise.TableSystemDefaults
	if err := h.db.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&sd).Error; err != nil {
		return "", err
	}
	return sd.DefaultOrganizationID, nil
}

// ---- Meta --------------------------------------------------------

type rbacMetaResponse struct {
	Resources  []string `json:"resources"`
	Operations []string `json:"operations"`
	BuiltinRoles []string `json:"builtin_roles"`
}

func (h *RBACHandler) handleMeta(ctx *fasthttp.RequestCtx) {
	SendJSON(ctx, rbacMetaResponse{
		Resources:    tenancy.Resources,
		Operations:   tenancy.Operations,
		BuiltinRoles: []string{"Owner", "Admin", "Manager", "Member"},
	})
}

// ---- Me (current user + resolved scopes) -------------------------

type rbacMeResponse struct {
	OrganizationID string              `json:"organization_id"`
	WorkspaceID    string              `json:"workspace_id,omitempty"`
	UserID         string              `json:"user_id,omitempty"`
	Email          string              `json:"email,omitempty"`
	DisplayName    string              `json:"display_name,omitempty"`
	Scopes         []string            `json:"scopes"`
	Permissions    map[string][]string `json:"permissions"`
}

func (h *RBACHandler) handleMe(ctx *fasthttp.RequestCtx) {
	orgID, err := h.defaultOrgID()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to resolve default org")
		return
	}

	// In v1 single-org mode we grant full access (Owner-equivalent) until
	// session-to-user resolution is wired. The UI's RbacProvider still
	// renders the permission matrix correctly.
	perms := make(map[string][]string, len(tenancy.Resources))
	for _, r := range tenancy.Resources {
		perms[r] = append([]string(nil), tenancy.Operations...)
	}

	SendJSON(ctx, rbacMeResponse{
		OrganizationID: orgID,
		Scopes:         []string{"*"},
		Permissions:    perms,
	})
}

// ---- Roles -------------------------------------------------------

type createRoleRequest struct {
	Name   string              `json:"name"`
	Scopes map[string][]string `json:"scopes"`
}

type updateRoleRequest struct {
	Name   *string              `json:"name,omitempty"`
	Scopes *map[string][]string `json:"scopes,omitempty"`
}

func (h *RBACHandler) listRoles(ctx *fasthttp.RequestCtx) {
	orgID, err := h.defaultOrgID()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to resolve default org")
		return
	}
	roles, err := h.roles.ListRoles(ctx, orgID)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to list roles")
		return
	}
	out := make([]map[string]any, 0, len(roles))
	for _, r := range roles {
		out = append(out, roleToJSON(r))
	}
	SendJSON(ctx, map[string]any{"roles": out, "total": len(out)})
}

func (h *RBACHandler) createRole(ctx *fasthttp.RequestCtx) {
	var req createRoleRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name is required")
		return
	}
	orgID, err := h.defaultOrgID()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to resolve default org")
		return
	}
	role, err := h.roles.CreateRole(ctx, tenancy.CreateRoleInput{
		OrganizationID: orgID,
		Name:           req.Name,
		Scopes:         req.Scopes,
	})
	if err != nil {
		if errors.Is(err, tenancy.ErrDuplicateRole) {
			SendError(ctx, fasthttp.StatusConflict, err.Error())
			return
		}
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to create role")
		return
	}
	emitAudit(ctx, orgID, "role.create", "role", role.ID, nil, roleToJSON(*role))
	SendJSONWithStatus(ctx, map[string]any{"role": roleToJSON(*role)}, fasthttp.StatusCreated)
}

func (h *RBACHandler) updateRole(ctx *fasthttp.RequestCtx) {
	id, _ := ctx.UserValue("id").(string)
	if id == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "role id required")
		return
	}
	var req updateRoleRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	before, _ := h.roles.GetRole(ctx, id)
	role, err := h.roles.UpdateRole(ctx, id, tenancy.UpdateRoleInput{Name: req.Name, Scopes: req.Scopes})
	if err != nil {
		switch {
		case errors.Is(err, tenancy.ErrRoleNotFound):
			SendError(ctx, fasthttp.StatusNotFound, err.Error())
		case errors.Is(err, tenancy.ErrRoleIsBuiltin):
			SendError(ctx, fasthttp.StatusForbidden, err.Error())
		default:
			SendError(ctx, fasthttp.StatusInternalServerError, "failed to update role")
		}
		return
	}
	var beforeJSON any
	if before != nil {
		beforeJSON = roleToJSON(*before)
	}
	emitAudit(ctx, role.OrganizationID, "role.update", "role", role.ID, beforeJSON, roleToJSON(*role))
	SendJSON(ctx, map[string]any{"role": roleToJSON(*role)})
}

func (h *RBACHandler) deleteRole(ctx *fasthttp.RequestCtx) {
	id, _ := ctx.UserValue("id").(string)
	if id == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "role id required")
		return
	}
	before, _ := h.roles.GetRole(ctx, id)
	if err := h.roles.DeleteRole(ctx, id); err != nil {
		switch {
		case errors.Is(err, tenancy.ErrRoleNotFound):
			SendError(ctx, fasthttp.StatusNotFound, err.Error())
		case errors.Is(err, tenancy.ErrRoleIsBuiltin):
			SendError(ctx, fasthttp.StatusForbidden, err.Error())
		default:
			SendError(ctx, fasthttp.StatusInternalServerError, "failed to delete role")
		}
		return
	}
	orgID := ""
	var beforeJSON any
	if before != nil {
		orgID = before.OrganizationID
		beforeJSON = roleToJSON(*before)
	}
	emitAudit(ctx, orgID, "role.delete", "role", id, beforeJSON, nil)
	SendJSON(ctx, map[string]any{"deleted": true})
}

func roleToJSON(r tables_enterprise.TableRole) map[string]any {
	var scopes map[string][]string
	if r.ScopeJSON != "" {
		_ = json.Unmarshal([]byte(r.ScopeJSON), &scopes)
	}
	return map[string]any{
		"id":              r.ID,
		"organization_id": r.OrganizationID,
		"name":            r.Name,
		"scopes":          scopes,
		"is_builtin":      r.IsBuiltin,
		"created_at":      r.CreatedAt,
	}
}

// ---- Users -------------------------------------------------------

type createUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	Status      string `json:"status,omitempty"`
}

type updateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Status      *string `json:"status,omitempty"`
}

func (h *RBACHandler) listUsers(ctx *fasthttp.RequestCtx) {
	orgID, err := h.defaultOrgID()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to resolve default org")
		return
	}
	var users []tables_enterprise.TableUser
	if err := h.db.WithContext(ctx).Where("organization_id = ?", orgID).Order("created_at DESC").Find(&users).Error; err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to list users")
		return
	}
	// Attach assignments per user so the UI can show role badges.
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		var assigns []tables_enterprise.TableUserRoleAssignment
		_ = h.db.WithContext(ctx).Where("user_id = ?", u.ID).Find(&assigns).Error
		out = append(out, userToJSON(u, assigns))
	}
	SendJSON(ctx, map[string]any{"users": out, "total": len(out)})
}

func (h *RBACHandler) createUser(ctx *fasthttp.RequestCtx) {
	var req createUserRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "email is required")
		return
	}
	orgID, err := h.defaultOrgID()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to resolve default org")
		return
	}
	status := req.Status
	if status == "" {
		status = "pending"
	}
	now := time.Now().UTC()
	u := tables_enterprise.TableUser{
		ID:             uuid.NewString(),
		OrganizationID: orgID,
		Email:          req.Email,
		DisplayName:    req.DisplayName,
		Status:         status,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := h.db.WithContext(ctx).Create(&u).Error; err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to create user: "+err.Error())
		return
	}
	emitAudit(ctx, orgID, "user.create", "user", u.ID, nil, userToJSON(u, nil))
	SendJSONWithStatus(ctx, map[string]any{"user": userToJSON(u, nil)}, fasthttp.StatusCreated)
}

func (h *RBACHandler) updateUser(ctx *fasthttp.RequestCtx) {
	id, _ := ctx.UserValue("id").(string)
	if id == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "user id required")
		return
	}
	var req updateUserRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	var before tables_enterprise.TableUser
	_ = h.db.WithContext(ctx).Where("id = ?", id).First(&before).Error
	if err := h.db.WithContext(ctx).Model(&tables_enterprise.TableUser{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to update user")
		return
	}
	var u tables_enterprise.TableUser
	if err := h.db.WithContext(ctx).Where("id = ?", id).First(&u).Error; err != nil {
		SendError(ctx, fasthttp.StatusNotFound, "user not found")
		return
	}
	var assigns []tables_enterprise.TableUserRoleAssignment
	_ = h.db.WithContext(ctx).Where("user_id = ?", u.ID).Find(&assigns).Error
	emitAudit(ctx, u.OrganizationID, "user.update", "user", u.ID, userToJSON(before, nil), userToJSON(u, assigns))
	SendJSON(ctx, map[string]any{"user": userToJSON(u, assigns)})
}

func (h *RBACHandler) deleteUser(ctx *fasthttp.RequestCtx) {
	id, _ := ctx.UserValue("id").(string)
	if id == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "user id required")
		return
	}
	var before tables_enterprise.TableUser
	_ = h.db.WithContext(ctx).Where("id = ?", id).First(&before).Error
	if err := h.db.WithContext(ctx).Where("user_id = ?", id).Delete(&tables_enterprise.TableUserRoleAssignment{}).Error; err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to delete user assignments")
		return
	}
	if err := h.db.WithContext(ctx).Where("id = ?", id).Delete(&tables_enterprise.TableUser{}).Error; err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to delete user")
		return
	}
	emitAudit(ctx, before.OrganizationID, "user.delete", "user", id, userToJSON(before, nil), nil)
	SendJSON(ctx, map[string]any{"deleted": true})
}

func userToJSON(u tables_enterprise.TableUser, assignments []tables_enterprise.TableUserRoleAssignment) map[string]any {
	return map[string]any{
		"id":              u.ID,
		"organization_id": u.OrganizationID,
		"email":           u.Email,
		"display_name":    u.DisplayName,
		"status":          u.Status,
		"last_login_at":   u.LastLoginAt,
		"created_at":      u.CreatedAt,
		"updated_at":      u.UpdatedAt,
		"assignments":     assignments,
	}
}

// ---- Assignments -------------------------------------------------

type assignRoleRequest struct {
	UserID      string `json:"user_id"`
	RoleID      string `json:"role_id"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

func (h *RBACHandler) listAssignments(ctx *fasthttp.RequestCtx) {
	userID, _ := ctx.UserValue("id").(string)
	if userID == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "user id required")
		return
	}
	assigns, err := h.roles.ListAssignments(ctx, userID)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to list assignments")
		return
	}
	SendJSON(ctx, map[string]any{"assignments": assigns, "total": len(assigns)})
}

func (h *RBACHandler) assignRole(ctx *fasthttp.RequestCtx) {
	var req assignRoleRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.RoleID == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "user_id and role_id are required")
		return
	}
	a, err := h.roles.AssignRole(ctx, tenancy.AssignRoleInput{
		UserID:      req.UserID,
		RoleID:      req.RoleID,
		WorkspaceID: req.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, tenancy.ErrRoleNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, err.Error())
			return
		}
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to assign role: "+err.Error())
		return
	}
	orgID, _ := h.defaultOrgID()
	emitAudit(ctx, orgID, "role.assign", "role_assignment", a.ID, nil, a)
	SendJSONWithStatus(ctx, map[string]any{"assignment": a}, fasthttp.StatusCreated)
}

func (h *RBACHandler) unassignRole(ctx *fasthttp.RequestCtx) {
	id, _ := ctx.UserValue("id").(string)
	if id == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "assignment id required")
		return
	}
	if err := h.roles.UnassignRole(ctx, id); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to unassign role")
		return
	}
	orgID, _ := h.defaultOrgID()
	emitAudit(ctx, orgID, "role.unassign", "role_assignment", id, nil, nil)
	SendJSON(ctx, map[string]any{"deleted": true})
}
