// SCIM 2.0 Users read-only — /scim/v2/Users (spec 020).
//
// Implements the minimum subset Okta / Azure AD need to "Import
// Users from Bifrost": GET list with pagination + userName-eq
// filtering, GET by id. Bearer-auth via the hash stored in
// ent_scim_config (spec 009). Write-side provisioning is spec 021.

package handlers

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

const (
	scimUserSchema     = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimListRespSchema = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	scimErrorSchema    = "urn:ietf:params:scim:api:messages:2.0:Error"
)

type SCIMUsersHandler struct {
	store  configstore.ConfigStore
	db     *gorm.DB
	logger schemas.Logger
}

func NewSCIMUsersHandler(store configstore.ConfigStore, logger schemas.Logger) *SCIMUsersHandler {
	return &SCIMUsersHandler{store: store, db: store.DB(), logger: logger}
}

func (h *SCIMUsersHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	// SCIM endpoints are NOT under /api/ — Okta + other IdPs pin
	// the /scim/v2/... prefix directly.
	r.GET("/scim/v2/Users", lib.ChainMiddlewares(h.list, middlewares...))
	r.GET("/scim/v2/Users/{id}", lib.ChainMiddlewares(h.get, middlewares...))
}

type scimUserResource struct {
	Schemas  []string            `json:"schemas"`
	ID       string              `json:"id"`
	UserName string              `json:"userName"`
	Name     scimUserName        `json:"name"`
	Emails   []scimUserEmail     `json:"emails"`
	Active   bool                `json:"active"`
}

type scimUserName struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type scimUserEmail struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary"`
}

type scimListResponse struct {
	Schemas      []string           `json:"schemas"`
	TotalResults int64              `json:"totalResults"`
	StartIndex   int                `json:"startIndex"`
	ItemsPerPage int                `json:"itemsPerPage"`
	Resources    []scimUserResource `json:"Resources"`
}

type scimError struct {
	Schemas []string `json:"schemas"`
	Detail  string   `json:"detail"`
	Status  string   `json:"status"`
}

func scimSendError(ctx *fasthttp.RequestCtx, status int, detail string) {
	ctx.Response.Header.Set("Content-Type", "application/scim+json")
	ctx.SetStatusCode(status)
	SendJSONWithStatus(ctx, scimError{
		Schemas: []string{scimErrorSchema},
		Status:  strconv.Itoa(status),
		Detail:  detail,
	}, status)
}

// auth validates the bearer token against the stored hash. Returns
// true when the token matches + SCIM is enabled. Sends the 401
// response envelope itself on failure so callers can just return.
func (h *SCIMUsersHandler) auth(ctx *fasthttp.RequestCtx) bool {
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		scimSendError(ctx, fasthttp.StatusUnauthorized, "missing bearer token")
		return false
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	cfg, err := h.store.GetSCIMConfig(context.Background())
	if err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, "scim config lookup failed")
		return false
	}
	if cfg == nil || !cfg.Enabled || cfg.BearerTokenHash == "" {
		scimSendError(ctx, fasthttp.StatusUnauthorized, "scim provisioning disabled")
		return false
	}
	wantHash := cfg.BearerTokenHash
	gotHash := sha256.Sum256([]byte(token))
	gotHex := hex.EncodeToString(gotHash[:])
	if subtle.ConstantTimeCompare([]byte(gotHex), []byte(wantHash)) != 1 {
		scimSendError(ctx, fasthttp.StatusUnauthorized, "invalid bearer token")
		return false
	}
	return true
}

// filterUserName extracts the value from a `userName eq "x"`
// expression. Returns (value, true, nil) on match, (nil, false, nil)
// when filter is empty, (nil, false, err) on any other form.
func parseUserNameFilter(filter string) (string, bool, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return "", false, nil
	}
	// Accept only: userName eq "value" (case-insensitive attr name).
	lower := strings.ToLower(filter)
	if !strings.HasPrefix(lower, "username eq ") {
		return "", false, fmt.Errorf("only `userName eq \"…\"` filters are supported in v1")
	}
	rest := strings.TrimPrefix(filter, filter[:12]) // preserve case of value
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, `"`) || !strings.HasSuffix(rest, `"`) || len(rest) < 2 {
		return "", false, fmt.Errorf("filter value must be a quoted string")
	}
	return rest[1 : len(rest)-1], true, nil
}

// toResource converts TableUser → SCIM User. Splits DisplayName on
// first whitespace for givenName/familyName (best-effort).
func toResource(u *tables_enterprise.TableUser) scimUserResource {
	given, family := "", ""
	if name := strings.TrimSpace(u.DisplayName); name != "" {
		if i := strings.IndexByte(name, ' '); i >= 0 {
			given = strings.TrimSpace(name[:i])
			family = strings.TrimSpace(name[i+1:])
		} else {
			given = name
		}
	}
	return scimUserResource{
		Schemas:  []string{scimUserSchema},
		ID:       u.ID,
		UserName: u.Email,
		Name:     scimUserName{GivenName: given, FamilyName: family},
		Emails:   []scimUserEmail{{Value: u.Email, Primary: true}},
		Active:   u.Status == "active",
	}
}

func (h *SCIMUsersHandler) list(ctx *fasthttp.RequestCtx) {
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
	filter := string(ctx.QueryArgs().Peek("filter"))

	q := h.db.Model(&tables_enterprise.TableUser{})
	if filterValue, matched, err := parseUserNameFilter(filter); err != nil {
		scimSendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	} else if matched {
		q = q.Where("email = ?", filterValue)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	var rows []tables_enterprise.TableUser
	if err := q.Order("email ASC").
		Offset(startIdx - 1).
		Limit(count).
		Find(&rows).Error; err != nil {
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	resources := make([]scimUserResource, len(rows))
	for i, u := range rows {
		resources[i] = toResource(&u)
	}
	resp := scimListResponse{
		Schemas:      []string{scimListRespSchema},
		TotalResults: total,
		StartIndex:   startIdx,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}
	ctx.Response.Header.Set("Content-Type", "application/scim+json")
	SendJSON(ctx, resp)
}

func (h *SCIMUsersHandler) get(ctx *fasthttp.RequestCtx) {
	if !h.auth(ctx) {
		return
	}
	id := ctx.UserValue("id").(string)
	var u tables_enterprise.TableUser
	if err := h.db.Where("id = ?", id).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			scimSendError(ctx, fasthttp.StatusNotFound, "user not found")
			return
		}
		scimSendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	ctx.Response.Header.Set("Content-Type", "application/scim+json")
	SendJSON(ctx, toResource(&u))
}
