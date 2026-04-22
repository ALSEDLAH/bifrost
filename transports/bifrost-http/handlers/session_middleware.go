// Session middleware + /api/auth/me + /api/auth/logout (spec 025).
//
// Reads the bf_session cookie or Authorization: Bearer header issued
// by the spec 024 SSO callback, validates via VerifySessionToken,
// and exposes the resolved user_id on the ctx.

package handlers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

const (
	// CtxKeySessionUserID is the user-value key set by RequireSession.
	CtxKeySessionUserID    = "session_user_id"
	CtxKeySessionExpiresAt = "session_expires_at"
)

// RequireSession is the spec-025 opt-in session-cookie auth middleware.
// Place it in a handler's RegisterRoutes middleware list to gate
// access on a valid bf_session cookie or Bearer token.
func RequireSession() schemas.BifrostHTTPMiddleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			tok := extractSessionToken(ctx)
			if tok == "" {
				sendSessionError(ctx, fasthttp.StatusUnauthorized, "unauthenticated")
				return
			}
			uid, exp, ok := VerifySessionToken(tok)
			if !ok {
				sendSessionError(ctx, fasthttp.StatusUnauthorized, "invalid_session")
				return
			}
			if IsTokenRevoked(uid, exp) {
				sendSessionError(ctx, fasthttp.StatusUnauthorized, "session_revoked")
				return
			}
			ctx.SetUserValue(CtxKeySessionUserID, uid)
			ctx.SetUserValue(CtxKeySessionExpiresAt, exp)
			next(ctx)
		}
	}
}

// extractSessionToken pulls the token from the cookie first, then
// the Authorization header. Both forms must use the spec-024
// IssueSessionToken format.
func extractSessionToken(ctx *fasthttp.RequestCtx) string {
	if c := ctx.Request.Header.Cookie(sessionCookieName); len(c) > 0 {
		return string(c)
	}
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	return ""
}

func sendSessionError(ctx *fasthttp.RequestCtx, status int, msg string) {
	ctx.Response.Header.Set("Content-Type", "application/json")
	SendJSONWithStatus(ctx, map[string]any{"error": msg}, status)
}

// SSOSessionHandler exposes the /me + /logout endpoints. Lives in this
// file rather than alongside SSO so the auth-handler-vs-session-handler
// split is obvious.
type SSOSessionHandler struct {
	store        configstore.ConfigStore
	db           *gorm.DB
	logger       schemas.Logger
	revokeCache  *dbRevocationChecker
}

func NewSSOSessionHandler(store configstore.ConfigStore, logger schemas.Logger) *SSOSessionHandler {
	h := &SSOSessionHandler{store: store, db: store.DB(), logger: logger}
	h.revokeCache = newDBRevocationChecker(h.db)
	// Wire the package-level revocation checker so RequireSession sees
	// it. Multiple handler instances would just overwrite each other,
	// which is fine in practice since the production path constructs
	// exactly one.
	SetSessionRevocationChecker(h.revokeCache.check)
	return h
}

func (h *SSOSessionHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	// /me requires a valid session — chain the spec 025 middleware on
	// top of whatever the caller already had.
	gated := append([]schemas.BifrostHTTPMiddleware{}, middlewares...)
	gated = append(gated, RequireSession())
	r.GET("/api/auth/me", lib.ChainMiddlewares(h.me, gated...))
	// /logout intentionally NOT gated — clearing an already-cleared
	// cookie should still 204.
	r.POST("/api/auth/logout", lib.ChainMiddlewares(h.logout, middlewares...))
	// Spec 026 revoke endpoints — same handler, separate registration
	// so the new routes are obvious in startup logs.
	h.RegisterRevokeRoutes(r, middlewares...)
}

func (h *SSOSessionHandler) me(ctx *fasthttp.RequestCtx) {
	uid, _ := ctx.UserValue(CtxKeySessionUserID).(string)
	if uid == "" {
		// Belt-and-braces — RequireSession should have rejected.
		sendSessionError(ctx, fasthttp.StatusUnauthorized, "unauthenticated")
		return
	}
	var u tables_enterprise.TableUser
	err := h.db.Where("id = ?", uid).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			emitSessionAudit("auth.me", uid, "denied", map[string]any{"reason": "user_deleted"})
			sendSessionError(ctx, fasthttp.StatusNotFound, "user_not_found")
			return
		}
		sendSessionError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	exp, _ := ctx.UserValue(CtxKeySessionExpiresAt).(time.Time)
	emitSessionAudit("auth.me", u.Email, "allowed", map[string]any{"user_id": u.ID})
	SendJSON(ctx, map[string]any{
		"id":              u.ID,
		"email":           u.Email,
		"display_name":    u.DisplayName,
		"status":          u.Status,
		"organization_id": u.OrganizationID,
		"expires_at":      exp.UTC().Format(time.RFC3339),
	})
}

func (h *SSOSessionHandler) logout(ctx *fasthttp.RequestCtx) {
	// Best-effort audit — capture user_id IF a valid cookie is present,
	// but never block the logout itself.
	if tok := extractSessionToken(ctx); tok != "" {
		if uid, _, ok := VerifySessionToken(tok); ok {
			emitSessionAudit("auth.logout", uid, "allowed", nil)
		}
	}
	clear := fasthttp.AcquireCookie()
	clear.SetKey(sessionCookieName)
	clear.SetValue("")
	clear.SetPath("/")
	clear.SetMaxAge(-1) // delete now
	clear.SetHTTPOnly(true)
	clear.SetSecure(true)
	clear.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	ctx.Response.Header.SetCookie(clear)
	fasthttp.ReleaseCookie(clear)
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func emitSessionAudit(action, principal, outcome string, after map[string]any) {
	bctx := schemas.NewBifrostContext(context.Background(), time.Time{})
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, "system")
	bctx.SetValue(tenancy.BifrostContextKeyUserID, principal)
	bctx.SetValue(tenancy.BifrostContextKeyResolvedVia, tenancy.Resolver("session"))
	_ = audit.Emit(context.Background(), bctx, audit.Entry{
		Action: action, ResourceType: "user_session",
		ResourceID: principal, Outcome: outcome, After: after,
	})
}
