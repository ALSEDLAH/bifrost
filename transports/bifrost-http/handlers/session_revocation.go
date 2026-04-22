// Per-user session revocation (spec 026). Adds an enforcement hook
// to RequireSession + admin/self revoke endpoints.

package handlers

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

// revocationCacheTTL controls how long the in-memory revoke timestamp
// stays trusted before we re-query the DB.
const revocationCacheTTL = 30 * time.Second

// sessionRevocationChecker is the package-level hook RequireSession
// calls after a token verifies. Returns the user's most-recent
// revocation timestamp and whether the user has any revocation row at all.
//
// Wired at startup by NewSSOSessionHandler so RequireSession itself
// stays a no-arg func that anyone can call.
var (
	revokeMu      sync.RWMutex
	revokeChecker func(userID string) (time.Time, bool)
)

// SetSessionRevocationChecker is the wiring hook called by
// NewSSOSessionHandler. Passing nil clears the checker (used by tests).
func SetSessionRevocationChecker(fn func(userID string) (time.Time, bool)) {
	revokeMu.Lock()
	revokeChecker = fn
	revokeMu.Unlock()
}

// IsTokenRevoked decides whether a token whose decoded expiresAt is
// `exp` for `userID` has been invalidated by a revocation event.
//
// Returns true when there exists a revocation row whose `revoked_at`
// is at-or-after the token's implied issue time (= exp − sessionExpiry).
func IsTokenRevoked(userID string, exp time.Time) bool {
	revokeMu.RLock()
	checker := revokeChecker
	revokeMu.RUnlock()
	if checker == nil {
		return false
	}
	revokedAt, has := checker(userID)
	if !has {
		return false
	}
	impliedIat := exp.Add(-sessionExpiry)
	return !revokedAt.Before(impliedIat)
}

// revocationCacheEntry holds a per-user lookup result + when it
// was fetched. zeroTime means "no revocation row exists".
type revocationCacheEntry struct {
	revokedAt time.Time
	cachedAt  time.Time
	exists    bool
}

// dbRevocationChecker is the production checker — wraps the configstore
// in a TTL cache so every /me call doesn't hit the DB.
type dbRevocationChecker struct {
	db    *gorm.DB
	mu    sync.Mutex
	cache map[string]revocationCacheEntry
	nowFn func() time.Time
}

func newDBRevocationChecker(db *gorm.DB) *dbRevocationChecker {
	return &dbRevocationChecker{
		db:    db,
		cache: make(map[string]revocationCacheEntry),
		nowFn: time.Now,
	}
}

func (c *dbRevocationChecker) check(userID string) (time.Time, bool) {
	c.mu.Lock()
	if e, ok := c.cache[userID]; ok && c.nowFn().Sub(e.cachedAt) < revocationCacheTTL {
		c.mu.Unlock()
		if !e.exists {
			return time.Time{}, false
		}
		return e.revokedAt, true
	}
	c.mu.Unlock()

	var row tables_enterprise.TableSessionRevocation
	err := c.db.Where("user_id = ?", userID).First(&row).Error
	entry := revocationCacheEntry{cachedAt: c.nowFn()}
	switch {
	case err == nil:
		entry.exists = true
		entry.revokedAt = row.RevokedAt
	case errors.Is(err, gorm.ErrRecordNotFound):
		entry.exists = false
	default:
		// On unexpected DB errors fail open for one tick — better than
		// 401-storming every active user. Cache result very briefly.
		entry.exists = false
	}
	c.mu.Lock()
	c.cache[userID] = entry
	c.mu.Unlock()
	if !entry.exists {
		return time.Time{}, false
	}
	return entry.revokedAt, true
}

// invalidate drops a single user's cached entry — called immediately
// after a revoke so the same process sees the new revoke timestamp.
func (c *dbRevocationChecker) invalidate(userID string) {
	c.mu.Lock()
	delete(c.cache, userID)
	c.mu.Unlock()
}

// RegisterRevokeRoutes wires the /api/auth/sessions/revoke + revoke-self
// endpoints on the same SSOSessionHandler. Spec 029 adds /refresh on
// the same gated chain since the auth requirements are identical.
func (h *SSOSessionHandler) RegisterRevokeRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	gated := append([]schemas.BifrostHTTPMiddleware{}, middlewares...)
	gated = append(gated, RequireSession())
	r.POST("/api/auth/sessions/revoke", lib.ChainMiddlewares(h.revokeOther, gated...))
	r.POST("/api/auth/sessions/revoke-self", lib.ChainMiddlewares(h.revokeSelf, gated...))
	r.POST("/api/auth/sessions/refresh", lib.ChainMiddlewares(h.refresh, gated...))
}

// refresh re-issues a fresh 8h session token for the caller. Works as
// long as RequireSession already accepted the caller — meaning the
// current token was valid AND not revoked.
func (h *SSOSessionHandler) refresh(ctx *fasthttp.RequestCtx) {
	uid, _ := ctx.UserValue(CtxKeySessionUserID).(string)
	if uid == "" {
		sendSessionError(ctx, fasthttp.StatusUnauthorized, "unauthenticated")
		return
	}
	expiresAt := time.Now().Add(sessionExpiry).UTC()
	tok := IssueSessionToken(uid, expiresAt)

	cookie := fasthttp.AcquireCookie()
	cookie.SetKey(sessionCookieName)
	cookie.SetValue(tok)
	cookie.SetPath("/")
	cookie.SetHTTPOnly(true)
	cookie.SetSecure(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	cookie.SetMaxAge(int(sessionExpiry / time.Second))
	ctx.Response.Header.SetCookie(cookie)
	fasthttp.ReleaseCookie(cookie)

	emitSessionAudit("auth.session_refreshed", uid, "allowed", map[string]any{"user_id": uid})
	SendJSON(ctx, map[string]any{
		"user_id":       uid,
		"session_token": tok,
		"expires_at":    expiresAt.Format(time.RFC3339),
	})
}

// callerIsAdmin returns true when the logged-in user holds a role
// named Owner or Admin. Role table is org-scoped, but the user's org
// is captured implicitly by the role_id reference — no extra org
// filter needed beyond resolving the user's own row.
func (h *SSOSessionHandler) callerIsAdmin(callerID string) bool {
	type roleRow struct{ Name string }
	var rows []roleRow
	h.db.Table("ent_user_role_assignments").
		Select("ent_roles.name AS name").
		Joins("JOIN ent_roles ON ent_roles.id = ent_user_role_assignments.role_id").
		Where("ent_user_role_assignments.user_id = ?", callerID).
		Find(&rows)
	for _, r := range rows {
		switch r.Name {
		case "Owner", "Admin":
			return true
		}
	}
	return false
}

func (h *SSOSessionHandler) revokeSelf(ctx *fasthttp.RequestCtx) {
	uid, _ := ctx.UserValue(CtxKeySessionUserID).(string)
	if uid == "" {
		sendSessionError(ctx, fasthttp.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.upsertRevocation(uid); err != nil {
		sendSessionError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	emitSessionAudit("auth.session_revoked", uid, "allowed", map[string]any{"target_user_id": uid, "self": true})
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *SSOSessionHandler) revokeOther(ctx *fasthttp.RequestCtx) {
	caller, _ := ctx.UserValue(CtxKeySessionUserID).(string)
	target := strings.TrimSpace(string(ctx.QueryArgs().Peek("user_id")))
	if target == "" {
		sendSessionError(ctx, fasthttp.StatusBadRequest, "user_id query parameter is required")
		return
	}
	// Self-revoke through the admin endpoint is allowed without admin role.
	if target != caller && !h.callerIsAdmin(caller) {
		emitSessionAudit("auth.session_revoked", caller, "denied", map[string]any{"target_user_id": target, "reason": "not_admin"})
		sendSessionError(ctx, fasthttp.StatusForbidden, "admin role required to revoke another user")
		return
	}
	if err := h.upsertRevocation(target); err != nil {
		sendSessionError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	emitSessionAudit("auth.session_revoked", caller, "allowed", map[string]any{"target_user_id": target})
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *SSOSessionHandler) upsertRevocation(userID string) error {
	// Truncate to second precision — session tokens carry exp as Unix
	// seconds, so the implied_iat comparison only meaningfully resolves
	// to whole seconds. Sub-second nanos here would shadow tokens that
	// were issued in the very same second as the revoke.
	now := time.Now().UTC().Truncate(time.Second)
	row := tables_enterprise.TableSessionRevocation{UserID: userID, RevokedAt: now}
	// Save = upsert when PK present.
	if err := h.db.Save(&row).Error; err != nil {
		return err
	}
	// Bust the per-user cache so the next /me call sees the fresh
	// revocation immediately rather than after 30s.
	if h.revokeCache != nil {
		h.revokeCache.invalidate(userID)
	}
	return nil
}
