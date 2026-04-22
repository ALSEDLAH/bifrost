// Spec 029 tests: session refresh endpoint.

package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// invokeRefresh hits POST /api/auth/sessions/refresh with the given
// cookie, driving the full RequireSession → refresh chain.
func invokeRefresh(t *testing.T, h *SSOSessionHandler, tok string) (int, map[string]any, string) {
	t.Helper()
	wrapped := RequireSession()(h.refresh)
	ctx := &fasthttp.RequestCtx{}
	if tok != "" {
		ctx.Request.Header.SetCookie(sessionCookieName, tok)
	}
	wrapped(ctx)
	var out map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &out)
	return ctx.Response.StatusCode(), out, string(ctx.Response.Header.Peek("Set-Cookie"))
}

func TestRefresh_HappyPath_IssuesNewToken(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	pinDefaultOrgForSSO(t, store, "org-x")
	require.NoError(t, store.DB().Create(&tables_enterprise.TableUser{
		ID: "u-refresh", OrganizationID: "org-x",
		Email: "r@example.com", Status: "active", IdpSubject: "sub-r",
	}).Error)
	h := NewSSOSessionHandler(store, &mockLogger{})
	t.Cleanup(func() { SetSessionRevocationChecker(nil) })

	// Original token expires in 1 hour — refresh should push it out to ~8h.
	originalExp := time.Now().Add(time.Hour)
	oldTok := IssueSessionToken("u-refresh", originalExp)

	status, out, cookie := invokeRefresh(t, h, oldTok)
	if status != 200 {
		t.Fatalf("status %d body %+v", status, out)
	}
	newTok, _ := out["session_token"].(string)
	if newTok == "" || newTok == oldTok {
		t.Errorf("new token should differ from old; newTok=%q", newTok)
	}
	// New token must verify and have a later expiry than the old one.
	uid, newExp, ok := VerifySessionToken(newTok)
	if !ok || uid != "u-refresh" {
		t.Fatalf("new token didn't verify: uid=%q ok=%v", uid, ok)
	}
	if !newExp.After(originalExp) {
		t.Errorf("refresh should extend expiry; newExp=%v originalExp=%v", newExp, originalExp)
	}
	// Cookie attributes must match the spec-024 shape.
	if !strings.Contains(cookie, sessionCookieName+"=") {
		t.Errorf("cookie not set: %q", cookie)
	}
	if !strings.Contains(cookie, "HttpOnly") {
		t.Errorf("cookie not HttpOnly: %q", cookie)
	}
	if !strings.Contains(strings.ToLower(cookie), "samesite=lax") {
		t.Errorf("cookie not SameSite=Lax: %q", cookie)
	}
}

func TestRefresh_NoCookie_Returns401(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	h := NewSSOSessionHandler(store, &mockLogger{})
	t.Cleanup(func() { SetSessionRevocationChecker(nil) })

	status, _, _ := invokeRefresh(t, h, "")
	if status != 401 {
		t.Errorf("no-cookie refresh should 401; got %d", status)
	}
}

func TestRefresh_TamperedToken_Returns401(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	h := NewSSOSessionHandler(store, &mockLogger{})
	t.Cleanup(func() { SetSessionRevocationChecker(nil) })

	good := IssueSessionToken("u-x", time.Now().Add(time.Hour))
	status, _, _ := invokeRefresh(t, h, good[:len(good)-3]+"AAA")
	if status != 401 {
		t.Errorf("tampered token should 401; got %d", status)
	}
}

func TestRefresh_ExpiredToken_Returns401(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	h := NewSSOSessionHandler(store, &mockLogger{})
	t.Cleanup(func() { SetSessionRevocationChecker(nil) })

	expired := IssueSessionToken("u-x", time.Now().Add(-time.Minute))
	status, _, _ := invokeRefresh(t, h, expired)
	if status != 401 {
		t.Errorf("expired token should 401; got %d", status)
	}
}

func TestRefresh_Revoked_Returns401(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	pinDefaultOrgForSSO(t, store, "org-x")
	require.NoError(t, store.DB().Create(&tables_enterprise.TableUser{
		ID: "u-rev", OrganizationID: "org-x",
		Email: "rev@example.com", Status: "active", IdpSubject: "sub-rev",
	}).Error)
	h := NewSSOSessionHandler(store, &mockLogger{})
	t.Cleanup(func() { SetSessionRevocationChecker(nil) })

	tok := IssueSessionToken("u-rev", time.Now().Add(time.Hour))
	// Revoke user — same pattern as spec 026 self-revoke.
	require.NoError(t, h.upsertRevocation("u-rev"))

	status, _, _ := invokeRefresh(t, h, tok)
	if status != 401 {
		t.Errorf("revoked user refresh should 401; got %d", status)
	}
}

func TestRefresh_NewTokenRespectedByMe(t *testing.T) {
	resetSessionSecretForTests()
	store := newSSOTestStore(t)
	pinDefaultOrgForSSO(t, store, "org-x")
	require.NoError(t, store.DB().Create(&tables_enterprise.TableUser{
		ID: "u-chain", OrganizationID: "org-x",
		Email: "chain@example.com", Status: "active",
		DisplayName: "Chain Person", IdpSubject: "sub-chain",
	}).Error)
	h := NewSSOSessionHandler(store, &mockLogger{})
	t.Cleanup(func() { SetSessionRevocationChecker(nil) })

	oldTok := IssueSessionToken("u-chain", time.Now().Add(time.Hour))
	_, out, _ := invokeRefresh(t, h, oldTok)
	newTok, _ := out["session_token"].(string)
	require.NotEmpty(t, newTok)

	// /me with the fresh token must work.
	wrapped := RequireSession()(h.me)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetCookie(sessionCookieName, newTok)
	wrapped(ctx)
	if ctx.Response.StatusCode() != 200 {
		t.Errorf("/me with refreshed token should 200; got %d body %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var me map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &me)
	if me["email"] != "chain@example.com" {
		t.Errorf("/me returned wrong user: %+v", me)
	}
}
