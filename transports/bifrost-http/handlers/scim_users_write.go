// SCIM 2.0 Users write-side — POST / PATCH / DELETE (spec 021).
//
// Lives next to scim_users.go (the read-only half from spec 020)
// and reuses its auth helper + schema constants. Audit.Emit fires
// on every successful mutation.

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fasthttp/router"
	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

// RegisterWriteRoutes wires POST / PATCH / DELETE on /scim/v2/Users.
// Separate from the read-only RegisterRoutes so tests can exercise
// either surface independently.
func (h *SCIMUsersHandler) RegisterWriteRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.POST("/scim/v2/Users", lib.ChainMiddlewares(h.create, middlewares...))
	r.PATCH("/scim/v2/Users/{id}", lib.ChainMiddlewares(h.patch, middlewares...))
	r.DELETE("/scim/v2/Users/{id}", lib.ChainMiddlewares(h.del, middlewares...))
}

type scimCreateBody struct {
	Schemas    []string        `json:"schemas"`
	UserName   string          `json:"userName"`
	ExternalID string          `json:"externalId,omitempty"`
	Name       scimUserName    `json:"name"`
	Emails     []scimUserEmail `json:"emails,omitempty"`
	Active     *bool           `json:"active,omitempty"`
}

type scimPatchBody struct {
	Schemas    []string         `json:"schemas"`
	Operations []scimPatchOp    `json:"Operations"`
}

type scimPatchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// defaultOrgID returns the synthetic default org UUID from ent_system_defaults.
func (h *SCIMUsersHandler) defaultOrgID() (string, error) {
	var sd tables_enterprise.TableSystemDefaults
	if err := h.db.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&sd).Error; err != nil {
		return "", err
	}
	return sd.DefaultOrganizationID, nil
}

// combineName joins givenName + familyName into a single string,
// skipping empties.
func combineName(n scimUserName) string {
	parts := []string{}
	if g := strings.TrimSpace(n.GivenName); g != "" {
		parts = append(parts, g)
	}
	if f := strings.TrimSpace(n.FamilyName); f != "" {
		parts = append(parts, f)
	}
	return strings.Join(parts, " ")
}

func (h *SCIMUsersHandler) create(ctx *fasthttp.RequestCtx) {
	if !h.auth(ctx) {
		return
	}
	if len(ctx.PostBody()) > 64*1024 {
		scimSendError(ctx, fasthttp.StatusRequestEntityTooLarge, "body exceeds 64 KiB")
		return
	}
	var req scimCreateBody
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		scimSendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.UserName == "" {
		scimSendError(ctx, fasthttp.StatusBadRequest, "userName is required")
		return
	}
	orgID, err := h.defaultOrgID()
	if err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, "default org lookup failed")
		return
	}
	u := tables_enterprise.TableUser{
		ID:             uuid.NewString(),
		OrganizationID: orgID,
		Email:          req.UserName,
		DisplayName:    combineName(req.Name),
		IdpSubject:     req.ExternalID,
		Status:         "active",
	}
	if req.Active != nil && !*req.Active {
		u.Status = "suspended"
	}

	if err := h.db.Create(&u).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			scimSendError(ctx, fasthttp.StatusConflict, fmt.Sprintf("userName %q already exists", req.UserName))
			return
		}
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	emitSCIMAudit(ctx, "user.create", u.ID, map[string]any{
		"userName":    u.Email,
		"displayName": u.DisplayName,
		"active":      u.Status == "active",
	})

	ctx.Response.Header.Set("Content-Type", "application/scim+json")
	SendJSONWithStatus(ctx, toResource(&u), fasthttp.StatusCreated)
}

func (h *SCIMUsersHandler) patch(ctx *fasthttp.RequestCtx) {
	if !h.auth(ctx) {
		return
	}
	id := ctx.UserValue("id").(string)
	var u tables_enterprise.TableUser
	if err := h.db.Where("id = ?", id).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			scimSendError(ctx, fasthttp.StatusNotFound, "user not found")
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

	givenBefore, familyBefore := splitDisplay(u.DisplayName)
	given, family := givenBefore, familyBefore

	for _, op := range req.Operations {
		opName := strings.ToLower(strings.TrimSpace(op.Op))
		if opName != "replace" {
			scimSendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("only Replace ops are supported in v1; got %q", op.Op))
			return
		}
		switch op.Path {
		case "active":
			var b bool
			if err := json.Unmarshal(op.Value, &b); err != nil {
				scimSendError(ctx, fasthttp.StatusBadRequest, "active must be boolean")
				return
			}
			if b {
				u.Status = "active"
			} else {
				u.Status = "suspended"
			}
		case "userName":
			var s string
			if err := json.Unmarshal(op.Value, &s); err != nil || s == "" {
				scimSendError(ctx, fasthttp.StatusBadRequest, "userName must be non-empty string")
				return
			}
			u.Email = s
		case "name.givenName":
			var s string
			if err := json.Unmarshal(op.Value, &s); err != nil {
				scimSendError(ctx, fasthttp.StatusBadRequest, "name.givenName must be string")
				return
			}
			given = s
		case "name.familyName":
			var s string
			if err := json.Unmarshal(op.Value, &s); err != nil {
				scimSendError(ctx, fasthttp.StatusBadRequest, "name.familyName must be string")
				return
			}
			family = s
		default:
			scimSendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("unsupported path %q (v1 supports active, userName, name.givenName, name.familyName)", op.Path))
			return
		}
	}
	u.DisplayName = combineName(scimUserName{GivenName: given, FamilyName: family})

	if err := h.db.Save(&u).Error; err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	emitSCIMAudit(ctx, "user.update", u.ID, map[string]any{
		"userName": u.Email, "status": u.Status,
	})

	ctx.Response.Header.Set("Content-Type", "application/scim+json")
	SendJSON(ctx, toResource(&u))
}

func (h *SCIMUsersHandler) del(ctx *fasthttp.RequestCtx) {
	if !h.auth(ctx) {
		return
	}
	id := ctx.UserValue("id").(string)
	var u tables_enterprise.TableUser
	if err := h.db.Where("id = ?", id).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Idempotent — missing id is still a 204 for Okta.
			ctx.SetStatusCode(fasthttp.StatusNoContent)
			return
		}
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	u.Status = "suspended"
	if err := h.db.Save(&u).Error; err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	emitSCIMAudit(ctx, "user.delete", u.ID, map[string]any{
		"userName": u.Email, "soft_delete": true,
	})
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// splitDisplay reverses combineName for PATCH semantics — splits at
// the first whitespace. Matches toResource()'s name split in scim_users.go.
func splitDisplay(s string) (given, family string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

// emitSCIMAudit is a thin wrapper around audit.Emit that swallows
// the ErrNoSink case (happens during unit tests without the audit
// plugin initialised). Builds a synthetic tenant context attributing
// the action to the "scim" resolver — matches the pattern used by
// other handlers that run outside the normal auth middleware chain.
func emitSCIMAudit(_ *fasthttp.RequestCtx, action, resourceID string, after map[string]any) {
	bctx := schemas.NewBifrostContext(context.Background(), time.Time{})
	// Seed a synthetic tenant context so audit.Emit's FromContext
	// call finds something to attribute the action to.
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, "system")
	bctx.SetValue(tenancy.BifrostContextKeyUserID, "scim")
	bctx.SetValue(tenancy.BifrostContextKeyResolvedVia, tenancy.Resolver("scim"))
	_ = audit.Emit(context.Background(), bctx, audit.Entry{
		Action:       action,
		ResourceType: "user",
		ResourceID:   resourceID,
		Outcome:      "allowed",
		After:        after,
	})
}
