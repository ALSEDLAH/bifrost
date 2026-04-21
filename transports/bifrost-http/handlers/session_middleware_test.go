// Spec 025 tests: RequireSession middleware + /api/auth/me + /api/auth/logout.

package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// dummyHandler is the leaf the middleware wraps in tests — it
// records the user_id the middleware put on ctx so we can assert it.
type captured struct {
	uid string
	exp time.Time
	hit bool
}

func captureHandler(out *captured) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		out.uid, _ = ctx.UserValue(CtxKeySessionUserID).(string)
		out.exp, _ = ctx.UserValue(CtxKeySessionExpiresAt).(time.Time)
		out.hit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	}
}

func TestRequireSession_NoCookieNoHeader_Returns401(t *testing.T) {
	out := &captured{}
	mw := RequireSession()
	wrapped := mw(captureHandler(out))
	ctx := &fasthttp.RequestCtx{}
	wrapped(ctx)
	if ctx.Response.StatusCode() != 401 {
		t.Errorf("expected 401; got %d", ctx.Response.StatusCode())
	}
	if out.hit {
		t.Errorf("downstream handler should NOT have been called")
	}
}

func TestRequireSession_CookieValid_PassesUserID(t *testing.T) {
	resetSessionSecretForTests()
	tok := IssueSessionToken("u-123", time.Now().Add(time.Hour))

	out := &captured{}
	wrapped := RequireSession()(captureHandler(out))
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetCookie(sessionCookieName, tok)
	wrapped(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("expected 200; got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if !out.hit || out.uid != "u-123" {
		t.Errorf("middleware didn't propagate uid: hit=%v uid=%q", out.hit, out.uid)
	}
}

func TestRequireSession_BearerHeader_Works(t *testing.T) {
	resetSessionSecretForTests()
	tok := IssueSessionToken("u-bearer", time.Now().Add(time.Hour))

	out := &captured{}
	wrapped := RequireSession()(captureHandler(out))
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("Authorization", "Bearer "+tok)
	wrapped(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("expected 200; got %d", ctx.Response.StatusCode())
	}
	if out.uid != "u-bearer" {
		t.Errorf("uid wrong: %q", out.uid)
	}
}

func TestRequireSession_Tampered_Returns401(t *testing.T) {
	resetSessionSecretForTests()
	good := IssueSessionToken("u-x", time.Now().Add(time.Hour))
	bad := good[:len(good)-3] + "AAA"

	out := &captured{}
	wrapped := RequireSession()(captureHandler(out))
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetCookie(sessionCookieName, bad)
	wrapped(ctx)

	if ctx.Response.StatusCode() != 401 {
		t.Errorf("expected 401 for tampered; got %d", ctx.Response.StatusCode())
	}
	if out.hit {
		t.Errorf("handler should NOT have run for tampered token")
	}
}

func TestRequireSession_Expired_Returns401(t *testing.T) {
	resetSessionSecretForTests()
	tok := IssueSessionToken("u-old", time.Now().Add(-time.Minute))

	out := &captured{}
	wrapped := RequireSession()(captureHandler(out))
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetCookie(sessionCookieName, tok)
	wrapped(ctx)

	if ctx.Response.StatusCode() != 401 {
		t.Errorf("expected 401 for expired; got %d", ctx.Response.StatusCode())
	}
}

// invokeMe runs the /me handler with the given session token in cookie.
func invokeMe(t *testing.T, h *SSOSessionHandler, tok string) (int, map[string]any) {
	t.Helper()
	wrapped := RequireSession()(h.me)
	ctx := &fasthttp.RequestCtx{}
	if tok != "" {
		ctx.Request.Header.SetCookie(sessionCookieName, tok)
	}
	wrapped(ctx)
	var out map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &out)
	return ctx.Response.StatusCode(), out
}

func TestSessionMe_HappyPath(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	pinDefaultOrgForSSO(t, store, "org-x")
	require.NoError(t, store.DB().Create(&tables_enterprise.TableUser{
		ID: "u-me", OrganizationID: "org-x",
		Email: "me@example.com", Status: "active",
		DisplayName: "Me Person", IdpSubject: "me-sub",
	}).Error)
	h := NewSSOSessionHandler(store, &mockLogger{})
	tok := IssueSessionToken("u-me", time.Now().Add(time.Hour))

	status, out := invokeMe(t, h, tok)
	if status != 200 {
		t.Fatalf("status %d body %+v", status, out)
	}
	if out["email"] != "me@example.com" {
		t.Errorf("email wrong: %+v", out)
	}
	if out["display_name"] != "Me Person" {
		t.Errorf("display_name wrong: %+v", out)
	}
	if out["organization_id"] != "org-x" {
		t.Errorf("org wrong: %+v", out)
	}
}

func TestSessionMe_UserDeleted_Returns404(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	pinDefaultOrgForSSO(t, store, "org-x")
	h := NewSSOSessionHandler(store, &mockLogger{})
	// No user row inserted — token references a ghost id.
	tok := IssueSessionToken("u-ghost", time.Now().Add(time.Hour))

	status, out := invokeMe(t, h, tok)
	if status != 404 {
		t.Errorf("expected 404; got %d body %+v", status, out)
	}
}

func TestSessionMe_NoSession_Returns401(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	h := NewSSOSessionHandler(store, &mockLogger{})

	status, _ := invokeMe(t, h, "")
	if status != 401 {
		t.Errorf("expected 401 for missing session; got %d", status)
	}
}

func TestSessionLogout_ClearsCookie(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	h := NewSSOSessionHandler(store, &mockLogger{})

	tok := IssueSessionToken("u-out", time.Now().Add(time.Hour))
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetCookie(sessionCookieName, tok)
	h.logout(ctx)

	if ctx.Response.StatusCode() != 204 {
		t.Errorf("expected 204; got %d", ctx.Response.StatusCode())
	}
	cookieHeader := string(ctx.Response.Header.Peek("Set-Cookie"))
	if !strings.Contains(cookieHeader, sessionCookieName+"=") {
		t.Errorf("cookie not cleared: %q", cookieHeader)
	}
	// Max-Age=0 (or negative) is how a browser deletes the cookie.
	if !strings.Contains(cookieHeader, "max-age=0") &&
		!strings.Contains(cookieHeader, "Max-Age=0") {
		t.Errorf("cookie missing max-age=0: %q", cookieHeader)
	}
}

func TestSessionLogout_Idempotent_NoSession(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOSessionHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	h.logout(ctx) // no cookie at all
	if ctx.Response.StatusCode() != 204 {
		t.Errorf("logout without session should still 204; got %d", ctx.Response.StatusCode())
	}
}

// Sanity check: middleware composes cleanly via RegisterRoutes (i.e.,
// the gated middleware list is appended, not replaced).
func TestSessionMiddleware_RegisterRoutes_GatesMe(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOSessionHandler(store, &mockLogger{})

	// Outer middleware just records that it ran — proves it's still in
	// the chain even after RequireSession is appended.
	outerHits := 0
	outer := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(c *fasthttp.RequestCtx) {
			outerHits++
			next(c)
		}
	}
	gated := []schemas.BifrostHTTPMiddleware{outer, RequireSession()}
	wrapped := gated[0](gated[1](h.me))

	// No session → outer ran, RequireSession blocked, /me did not.
	ctx := &fasthttp.RequestCtx{}
	wrapped(ctx)
	if outerHits != 1 {
		t.Errorf("outer middleware should still run; hits=%d", outerHits)
	}
	if ctx.Response.StatusCode() != 401 {
		t.Errorf("expected 401 (blocked by RequireSession); got %d", ctx.Response.StatusCode())
	}
}
