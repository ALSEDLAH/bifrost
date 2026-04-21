// SCIM 2.0 Groups (spec 022). Implements GET list, GET by id,
// POST create, PATCH, and DELETE for /scim/v2/Groups. Reuses the
// bearer-auth helper + SCIM error envelope from scim_users.go.

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fasthttp/router"
	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

const scimGroupSchema = "urn:ietf:params:scim:schemas:core:2.0:Group"

type SCIMGroupsHandler struct {
	store  configstore.ConfigStore
	db     *gorm.DB
	logger schemas.Logger
}

func NewSCIMGroupsHandler(store configstore.ConfigStore, logger schemas.Logger) *SCIMGroupsHandler {
	return &SCIMGroupsHandler{store: store, db: store.DB(), logger: logger}
}

func (h *SCIMGroupsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/scim/v2/Groups", lib.ChainMiddlewares(h.list, middlewares...))
	r.GET("/scim/v2/Groups/{id}", lib.ChainMiddlewares(h.get, middlewares...))
	r.POST("/scim/v2/Groups", lib.ChainMiddlewares(h.create, middlewares...))
	r.PATCH("/scim/v2/Groups/{id}", lib.ChainMiddlewares(h.patch, middlewares...))
	r.DELETE("/scim/v2/Groups/{id}", lib.ChainMiddlewares(h.del, middlewares...))
}

// auth delegates to the Users handler's auth helper — same token, same table.
func (h *SCIMGroupsHandler) auth(ctx *fasthttp.RequestCtx) bool {
	return (&SCIMUsersHandler{store: h.store, db: h.db, logger: h.logger}).auth(ctx)
}

type scimGroupMember struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

type scimGroupResource struct {
	Schemas     []string          `json:"schemas"`
	ID          string            `json:"id"`
	DisplayName string            `json:"displayName"`
	ExternalID  string            `json:"externalId,omitempty"`
	Members     []scimGroupMember `json:"members"`
}

type scimGroupListResponse struct {
	Schemas      []string            `json:"schemas"`
	TotalResults int64               `json:"totalResults"`
	StartIndex   int                 `json:"startIndex"`
	ItemsPerPage int                 `json:"itemsPerPage"`
	Resources    []scimGroupResource `json:"Resources"`
}

type scimGroupCreateBody struct {
	Schemas     []string          `json:"schemas"`
	DisplayName string            `json:"displayName"`
	ExternalID  string            `json:"externalId,omitempty"`
	Members     []scimGroupMember `json:"members,omitempty"`
}

// toGroupResource assembles a SCIM Group resource. Members are looked
// up by joining on ent_users so we can populate the display field.
func (h *SCIMGroupsHandler) toGroupResource(g *tables_enterprise.TableSCIMGroup) scimGroupResource {
	type memberRow struct {
		UserID      string
		DisplayName string
		Email       string
	}
	var rows []memberRow
	h.db.Table("ent_scim_group_members").
		Select("ent_scim_group_members.user_id AS user_id, ent_users.display_name AS display_name, ent_users.email AS email").
		Joins("LEFT JOIN ent_users ON ent_users.id = ent_scim_group_members.user_id").
		Where("ent_scim_group_members.group_id = ?", g.ID).
		Order("ent_scim_group_members.user_id ASC").
		Find(&rows)

	members := make([]scimGroupMember, 0, len(rows))
	for _, r := range rows {
		disp := r.DisplayName
		if disp == "" {
			disp = r.Email
		}
		members = append(members, scimGroupMember{Value: r.UserID, Display: disp})
	}
	ext := ""
	if g.ExternalID != nil {
		ext = *g.ExternalID
	}
	return scimGroupResource{
		Schemas:     []string{scimGroupSchema},
		ID:          g.ID,
		DisplayName: g.DisplayName,
		ExternalID:  ext,
		Members:     members,
	}
}

func (h *SCIMGroupsHandler) list(ctx *fasthttp.RequestCtx) {
	if !h.auth(ctx) {
		return
	}
	startIdx, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("startIndex")))
	if startIdx < 1 {
		startIdx = 1
	}
	count, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("count")))
	if count < 1 {
		count = 50
	}
	if count > 100 {
		count = 100
	}

	q := h.db.Model(&tables_enterprise.TableSCIMGroup{})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	var rows []tables_enterprise.TableSCIMGroup
	if err := q.Order("display_name ASC").
		Offset(startIdx - 1).
		Limit(count).
		Find(&rows).Error; err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	resources := make([]scimGroupResource, len(rows))
	for i := range rows {
		resources[i] = h.toGroupResource(&rows[i])
	}
	ctx.Response.Header.Set("Content-Type", "application/scim+json")
	SendJSON(ctx, scimGroupListResponse{
		Schemas:      []string{scimListRespSchema},
		TotalResults: total,
		StartIndex:   startIdx,
		ItemsPerPage: len(resources),
		Resources:    resources,
	})
}

func (h *SCIMGroupsHandler) get(ctx *fasthttp.RequestCtx) {
	if !h.auth(ctx) {
		return
	}
	id := ctx.UserValue("id").(string)
	var g tables_enterprise.TableSCIMGroup
	if err := h.db.Where("id = ?", id).First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			scimSendError(ctx, fasthttp.StatusNotFound, "group not found")
			return
		}
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	ctx.Response.Header.Set("Content-Type", "application/scim+json")
	SendJSON(ctx, h.toGroupResource(&g))
}

// resolveMembers keeps only the user IDs that correspond to an existing
// ent_users row in the default org. Missing IDs are silently dropped —
// FR-007.
func (h *SCIMGroupsHandler) resolveMembers(orgID string, requested []scimGroupMember) []string {
	if len(requested) == 0 {
		return nil
	}
	wanted := make([]string, 0, len(requested))
	for _, m := range requested {
		if strings.TrimSpace(m.Value) != "" {
			wanted = append(wanted, m.Value)
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	var found []string
	h.db.Model(&tables_enterprise.TableUser{}).
		Where("organization_id = ? AND id IN ?", orgID, wanted).
		Pluck("id", &found)
	return found
}

func (h *SCIMGroupsHandler) defaultOrgID() (string, error) {
	var sd tables_enterprise.TableSystemDefaults
	if err := h.db.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&sd).Error; err != nil {
		return "", err
	}
	return sd.DefaultOrganizationID, nil
}

func (h *SCIMGroupsHandler) create(ctx *fasthttp.RequestCtx) {
	if !h.auth(ctx) {
		return
	}
	if len(ctx.PostBody()) > 64*1024 {
		scimSendError(ctx, fasthttp.StatusRequestEntityTooLarge, "body exceeds 64 KiB")
		return
	}
	var req scimGroupCreateBody
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		scimSendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		scimSendError(ctx, fasthttp.StatusBadRequest, "displayName is required")
		return
	}
	orgID, err := h.defaultOrgID()
	if err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, "default org lookup failed")
		return
	}
	g := tables_enterprise.TableSCIMGroup{
		ID:             uuid.NewString(),
		OrganizationID: orgID,
		DisplayName:    req.DisplayName,
	}
	if req.ExternalID != "" {
		ext := req.ExternalID
		g.ExternalID = &ext
	}
	if err := h.db.Create(&g).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			scimSendError(ctx, fasthttp.StatusConflict, fmt.Sprintf("group with externalId %q already exists", req.ExternalID))
			return
		}
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	for _, uid := range h.resolveMembers(orgID, req.Members) {
		_ = h.db.Create(&tables_enterprise.TableSCIMGroupMember{
			GroupID: g.ID, UserID: uid, CreatedAt: time.Now().UTC(),
		}).Error
	}
	emitSCIMGroupAudit(ctx, "group.create", g.ID, map[string]any{
		"displayName": g.DisplayName, "externalId": req.ExternalID,
	})
	ctx.Response.Header.Set("Content-Type", "application/scim+json")
	SendJSONWithStatus(ctx, h.toGroupResource(&g), fasthttp.StatusCreated)
}

// patch implements the four PATCH patterns listed in FR-004.
var scimRemoveMemberFilter = regexp.MustCompile(`^members\[value\s+eq\s+"([^"]+)"\]$`)

func (h *SCIMGroupsHandler) patch(ctx *fasthttp.RequestCtx) {
	if !h.auth(ctx) {
		return
	}
	id := ctx.UserValue("id").(string)
	var g tables_enterprise.TableSCIMGroup
	if err := h.db.Where("id = ?", id).First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			scimSendError(ctx, fasthttp.StatusNotFound, "group not found")
			return
		}
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	var req scimPatchBody
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		scimSendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if len(req.Operations) == 0 {
		scimSendError(ctx, fasthttp.StatusBadRequest, "Operations array is required")
		return
	}
	for _, op := range req.Operations {
		opName := strings.ToLower(strings.TrimSpace(op.Op))
		path := strings.TrimSpace(op.Path)

		switch {
		case opName == "replace" && path == "displayName":
			var s string
			if err := json.Unmarshal(op.Value, &s); err != nil || s == "" {
				scimSendError(ctx, fasthttp.StatusBadRequest, "displayName must be non-empty string")
				return
			}
			g.DisplayName = s
		case opName == "replace" && path == "members":
			var members []scimGroupMember
			if err := json.Unmarshal(op.Value, &members); err != nil {
				scimSendError(ctx, fasthttp.StatusBadRequest, "members must be an array")
				return
			}
			if err := h.db.Where("group_id = ?", g.ID).Delete(&tables_enterprise.TableSCIMGroupMember{}).Error; err != nil {
				scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
				return
			}
			for _, uid := range h.resolveMembers(g.OrganizationID, members) {
				_ = h.db.Create(&tables_enterprise.TableSCIMGroupMember{
					GroupID: g.ID, UserID: uid, CreatedAt: time.Now().UTC(),
				}).Error
			}
		case opName == "add" && path == "members":
			var members []scimGroupMember
			if err := json.Unmarshal(op.Value, &members); err != nil {
				scimSendError(ctx, fasthttp.StatusBadRequest, "members must be an array")
				return
			}
			for _, uid := range h.resolveMembers(g.OrganizationID, members) {
				_ = h.db.Create(&tables_enterprise.TableSCIMGroupMember{
					GroupID: g.ID, UserID: uid, CreatedAt: time.Now().UTC(),
				}).Error
			}
		case opName == "remove":
			if m := scimRemoveMemberFilter.FindStringSubmatch(path); len(m) == 2 {
				_ = h.db.
					Where("group_id = ? AND user_id = ?", g.ID, m[1]).
					Delete(&tables_enterprise.TableSCIMGroupMember{}).Error
				continue
			}
			scimSendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("unsupported remove path %q", op.Path))
			return
		default:
			scimSendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("unsupported op %q path %q", op.Op, op.Path))
			return
		}
	}
	g.UpdatedAt = time.Now().UTC()
	if err := h.db.Save(&g).Error; err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	emitSCIMGroupAudit(ctx, "group.update", g.ID, map[string]any{
		"displayName": g.DisplayName,
	})
	ctx.Response.Header.Set("Content-Type", "application/scim+json")
	SendJSON(ctx, h.toGroupResource(&g))
}

func (h *SCIMGroupsHandler) del(ctx *fasthttp.RequestCtx) {
	if !h.auth(ctx) {
		return
	}
	id := ctx.UserValue("id").(string)
	var g tables_enterprise.TableSCIMGroup
	if err := h.db.Where("id = ?", id).First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.SetStatusCode(fasthttp.StatusNoContent)
			return
		}
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.Where("group_id = ?", g.ID).Delete(&tables_enterprise.TableSCIMGroupMember{}).Error; err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.Delete(&g).Error; err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	emitSCIMGroupAudit(ctx, "group.delete", id, map[string]any{"displayName": g.DisplayName})
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func emitSCIMGroupAudit(_ *fasthttp.RequestCtx, action, resourceID string, after map[string]any) {
	bctx := schemas.NewBifrostContext(context.Background(), time.Time{})
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, "system")
	bctx.SetValue(tenancy.BifrostContextKeyUserID, "scim")
	bctx.SetValue(tenancy.BifrostContextKeyResolvedVia, tenancy.Resolver("scim"))
	_ = audit.Emit(context.Background(), bctx, audit.Entry{
		Action:       action,
		ResourceType: "group",
		ResourceID:   resourceID,
		Outcome:      "allowed",
		After:        after,
	})
}
