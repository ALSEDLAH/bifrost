// Spec 024 hardening tests: JWKS verification edge cases + session
// token issuance/verification roundtrips.

package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// callbackWithCustomToken: like runFullCallback but lets the test
// override what the IdP returns via a one-shot token-endpoint hook.
// We swap out the IdP's mux to inject a specific id_token.
func callbackWithCustomToken(t *testing.T, h *SSOOIDCHandler, idp *httptest.Server, idTokenOverride func() string) (int, map[string]any) {
	t.Helper()
	// Re-register /token to hand out the override.
	if idTokenOverride != nil {
		// The mux is not directly accessible — but we can spin a new
		// httptest server in front of the original and override only
		// /token. Simpler: build a fresh stub IdP with the override.
		t.Fatal("override path requires fresh stub — use freshIdPWithToken")
	}
	startCtx := &fasthttp.RequestCtx{}
	h.start(startCtx)
	loc := string(startCtx.Response.Header.Peek("Location"))
	state := ""
	for _, p := range strings.Split(strings.SplitN(loc, "?", 2)[1], "&") {
		if strings.HasPrefix(p, "state=") {
			state = strings.TrimPrefix(p, "state=")
		}
	}
	require.NotEmpty(t, state)
	cb := &fasthttp.RequestCtx{}
	cb.Request.SetRequestURI("/api/auth/sso/oidc/callback?state=" + state + "&code=anycode")
	h.callback(cb)
	var out map[string]any
	_ = json.Unmarshal(cb.Response.Body(), &out)
	return cb.Response.StatusCode(), out
}

// freshIdPWithToken stands up an IdP whose /token always returns the
// id_token produced by tokFn(). JWKS still serves the canonical key.
func freshIdPWithToken(t *testing.T, key *signingKey, tokFn func() string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		nB, eB := key.priv.N.Bytes(), bigIntBytes(key.priv.E)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kid": key.kid, "kty": "RSA", "use": "sig", "alg": "RS256",
				"n": b64u(nB), "e": b64u(eB),
			}},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id_token": tokFn(), "token_type": "Bearer",
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(clearJWKSCacheForTests)
	return srv
}

func bigIntBytes(e int) []byte {
	// Mirror x/oauth2's behavior: serialize the int as big-endian bytes
	// without leading zeros.
	out := []byte{}
	for v := e; v > 0; v >>= 8 {
		out = append([]byte{byte(v & 0xff)}, out...)
	}
	if len(out) == 0 {
		out = []byte{0}
	}
	return out
}

func setupHandlerWithIdP(t *testing.T, idpURL string) *SSOOIDCHandler {
	t.Helper()
	store := newSSOTestStore(t)
	pinDefaultOrgForSSO(t, store, "org-x")
	require.NoError(t, store.DB().Create(&tables_enterprise.TableUser{
		ID: "u-1", OrganizationID: "org-x",
		Email: "fred@example.com", Status: "active", IdpSubject: "fred-sub",
	}).Error)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	enableSSO(t, h, idpURL, false, nil)
	return h
}

// runCallbackHelper drives /start then /callback against the handler,
// returning status + body.
func runCallbackHelper(t *testing.T, h *SSOOIDCHandler) (int, map[string]any) {
	t.Helper()
	startCtx := &fasthttp.RequestCtx{}
	h.start(startCtx)
	if startCtx.Response.StatusCode() != 302 {
		t.Fatalf("start failed: %d %s", startCtx.Response.StatusCode(), startCtx.Response.Body())
	}
	loc := string(startCtx.Response.Header.Peek("Location"))
	state := ""
	for _, p := range strings.Split(strings.SplitN(loc, "?", 2)[1], "&") {
		if strings.HasPrefix(p, "state=") {
			state = strings.TrimPrefix(p, "state=")
		}
	}
	cb := &fasthttp.RequestCtx{}
	cb.Request.SetRequestURI("/api/auth/sso/oidc/callback?state=" + state + "&code=anycode")
	h.callback(cb)
	var out map[string]any
	_ = json.Unmarshal(cb.Response.Body(), &out)
	return cb.Response.StatusCode(), out
}

func TestSSO_Verify_HappyPath_IssuesSessionCookie(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	key := &signingKey{priv: priv, kid: "test-kid"}

	idp := freshIdPWithToken(t, key, func() string {
		return signRS256(t, key, nil, map[string]any{
			"sub": "fred-sub", "email": "fred@example.com",
			"iss": "ISSUER_PLACEHOLDER", "aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
	})
	// We need iss == idp.URL — rebuild closure with the URL now known.
	srv := idp
	closeFn := srv.Close
	srv.Close()
	mux := http.NewServeMux()
	var srv2 *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv2.URL,
			"authorization_endpoint": srv2.URL + "/authorize",
			"token_endpoint":         srv2.URL + "/token",
			"jwks_uri":               srv2.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		nB, eB := key.priv.N.Bytes(), bigIntBytes(key.priv.E)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kid": key.kid, "kty": "RSA", "alg": "RS256",
				"n": b64u(nB), "e": b64u(eB),
			}},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		tok := signRS256(t, key, nil, map[string]any{
			"sub": "fred-sub", "email": "fred@example.com",
			"iss": srv2.URL, "aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id_token": tok, "token_type": "Bearer",
		})
	})
	srv2 = httptest.NewServer(mux)
	t.Cleanup(srv2.Close)
	t.Cleanup(clearJWKSCacheForTests)
	_ = closeFn

	h := setupHandlerWithIdP(t, srv2.URL)
	status, out := runCallbackHelper(t, h)
	if status != 200 {
		t.Fatalf("status %d body %+v", status, out)
	}
	tokStr, _ := out["session_token"].(string)
	if tokStr == "" {
		t.Errorf("session_token missing from JSON body")
	}
	uid, _, ok := VerifySessionToken(tokStr)
	if !ok || uid != "u-1" {
		t.Errorf("session token didn't roundtrip: uid=%q ok=%v", uid, ok)
	}
}

// signedTokenIdP: variant of stubIdP whose /token returns an id_token
// produced by callerSigner — used for negative-path verification tests.
func signedTokenIdP(t *testing.T, key *signingKey, callerSigner func(issuerURL string) string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		nB, eB := key.priv.N.Bytes(), bigIntBytes(key.priv.E)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kid": key.kid, "kty": "RSA", "alg": "RS256",
				"n": b64u(nB), "e": b64u(eB),
			}},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		tok := callerSigner(srv.URL)
		_ = json.NewEncoder(w).Encode(map[string]any{"id_token": tok, "token_type": "Bearer"})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(clearJWKSCacheForTests)
	return srv
}

func TestSSO_Verify_TamperedPayload_Returns401(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	key := &signingKey{priv: priv, kid: "k1"}
	idp := signedTokenIdP(t, key, func(issuer string) string {
		good := signRS256(t, key, nil, map[string]any{
			"sub": "x", "email": "x@example.com",
			"iss": issuer, "aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		// Tamper: swap payload but keep header+sig.
		parts := strings.Split(good, ".")
		evilPayload := map[string]any{
			"sub": "evil", "email": "admin@example.com",
			"iss": issuer, "aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		}
		eb, _ := json.Marshal(evilPayload)
		return parts[0] + "." + base64.RawURLEncoding.EncodeToString(eb) + "." + parts[2]
	})
	h := setupHandlerWithIdP(t, idp.URL)
	status, _ := runCallbackHelper(t, h)
	if status != 401 {
		t.Errorf("tampered payload should 401; got %d", status)
	}
}

func TestSSO_Verify_DifferentSigningKey_Returns401(t *testing.T) {
	publishedKey := &signingKey{kid: "k1"}
	publishedKey.priv, _ = rsa.GenerateKey(rand.Reader, 2048)
	otherKey := &signingKey{kid: "k1"}
	otherKey.priv, _ = rsa.GenerateKey(rand.Reader, 2048)

	idp := signedTokenIdP(t, publishedKey, func(issuer string) string {
		// Sign with otherKey but advertise publishedKey's kid.
		return signRS256(t, otherKey, nil, map[string]any{
			"sub": "x", "email": "x@example.com",
			"iss": issuer, "aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
	})
	h := setupHandlerWithIdP(t, idp.URL)
	status, _ := runCallbackHelper(t, h)
	if status != 401 {
		t.Errorf("token signed by other key should 401; got %d", status)
	}
}

func TestSSO_Verify_ExpiredToken_Returns401(t *testing.T) {
	key := &signingKey{kid: "k1"}
	key.priv, _ = rsa.GenerateKey(rand.Reader, 2048)
	idp := signedTokenIdP(t, key, func(issuer string) string {
		return signRS256(t, key, nil, map[string]any{
			"sub": "x", "email": "x@example.com",
			"iss": issuer, "aud": "test-client",
			"exp": time.Now().Add(-time.Minute).Unix(), // expired
		})
	})
	h := setupHandlerWithIdP(t, idp.URL)
	status, _ := runCallbackHelper(t, h)
	if status != 401 {
		t.Errorf("expired token should 401; got %d", status)
	}
}

func TestSSO_Verify_WrongAud_Returns401(t *testing.T) {
	key := &signingKey{kid: "k1"}
	key.priv, _ = rsa.GenerateKey(rand.Reader, 2048)
	idp := signedTokenIdP(t, key, func(issuer string) string {
		return signRS256(t, key, nil, map[string]any{
			"sub": "x", "email": "x@example.com",
			"iss": issuer, "aud": "wrong-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
	})
	h := setupHandlerWithIdP(t, idp.URL)
	status, _ := runCallbackHelper(t, h)
	if status != 401 {
		t.Errorf("wrong aud should 401; got %d", status)
	}
}

func TestSSO_Verify_WrongIss_Returns401(t *testing.T) {
	key := &signingKey{kid: "k1"}
	key.priv, _ = rsa.GenerateKey(rand.Reader, 2048)
	idp := signedTokenIdP(t, key, func(issuer string) string {
		return signRS256(t, key, nil, map[string]any{
			"sub": "x", "email": "x@example.com",
			"iss": "https://impostor.example.com", "aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
	})
	h := setupHandlerWithIdP(t, idp.URL)
	status, _ := runCallbackHelper(t, h)
	if status != 401 {
		t.Errorf("wrong iss should 401; got %d", status)
	}
}

func TestSSO_Verify_RejectsNonRS256(t *testing.T) {
	key := &signingKey{kid: "k1"}
	key.priv, _ = rsa.GenerateKey(rand.Reader, 2048)
	idp := signedTokenIdP(t, key, func(issuer string) string {
		// alg=HS256 signaled but we still RSA-sign — verifier should
		// short-circuit on the alg check before signature even runs.
		return signRS256(t, key, map[string]string{"alg": "HS256"}, map[string]any{
			"sub": "x", "email": "x@example.com",
			"iss": issuer, "aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
	})
	h := setupHandlerWithIdP(t, idp.URL)
	status, out := runCallbackHelper(t, h)
	if status != 401 {
		t.Errorf("non-RS256 alg should 401; got %d body %+v", status, out)
	}
}

func TestSSO_SessionToken_Roundtrip(t *testing.T) {
	resetSessionSecretForTests()
	exp := time.Now().Add(time.Hour).UTC()
	tok := IssueSessionToken("u-roundtrip", exp)
	uid, gotExp, ok := VerifySessionToken(tok)
	if !ok {
		t.Fatalf("verify failed for fresh token")
	}
	if uid != "u-roundtrip" {
		t.Errorf("uid mismatch: got %q", uid)
	}
	if gotExp.Unix() != exp.Unix() {
		t.Errorf("exp mismatch: got %v want %v", gotExp, exp)
	}
}

func TestSSO_SessionToken_Tampered_Rejected(t *testing.T) {
	resetSessionSecretForTests()
	tok := IssueSessionToken("u-x", time.Now().Add(time.Hour))
	parts := strings.Split(tok, ".")
	require.Len(t, parts, 2)
	// Flip a single byte in the payload.
	pBytes, _ := base64.RawURLEncoding.DecodeString(parts[0])
	pBytes[0] ^= 0x01
	bad := base64.RawURLEncoding.EncodeToString(pBytes) + "." + parts[1]
	if _, _, ok := VerifySessionToken(bad); ok {
		t.Errorf("tampered token should be rejected")
	}
}

func TestSSO_SessionToken_Expired_Rejected(t *testing.T) {
	resetSessionSecretForTests()
	tok := IssueSessionToken("u-exp", time.Now().Add(-time.Minute))
	if _, _, ok := VerifySessionToken(tok); ok {
		t.Errorf("expired token should be rejected")
	}
}

func TestSSO_SessionToken_Malformed_Rejected(t *testing.T) {
	for _, bad := range []string{"", "abc", "abc.def.ghi", "...."} {
		if _, _, ok := VerifySessionToken(bad); ok {
			t.Errorf("expected reject for %q", bad)
		}
	}
}

func TestSSO_Callback_SetsSessionCookie(t *testing.T) {
	key := &signingKey{kid: "k1"}
	key.priv, _ = rsa.GenerateKey(rand.Reader, 2048)
	idp := signedTokenIdP(t, key, func(issuer string) string {
		return signRS256(t, key, nil, map[string]any{
			"sub": "fred-sub", "email": "fred@example.com",
			"iss": issuer, "aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
	})
	h := setupHandlerWithIdP(t, idp.URL)

	startCtx := &fasthttp.RequestCtx{}
	h.start(startCtx)
	loc := string(startCtx.Response.Header.Peek("Location"))
	state := ""
	for _, p := range strings.Split(strings.SplitN(loc, "?", 2)[1], "&") {
		if strings.HasPrefix(p, "state=") {
			state = strings.TrimPrefix(p, "state=")
		}
	}
	cb := &fasthttp.RequestCtx{}
	cb.Request.SetRequestURI("/api/auth/sso/oidc/callback?state=" + state + "&code=anycode")
	h.callback(cb)
	if cb.Response.StatusCode() != 200 {
		t.Fatalf("callback status %d body %s", cb.Response.StatusCode(), cb.Response.Body())
	}
	cookieHeader := string(cb.Response.Header.Peek("Set-Cookie"))
	if !strings.Contains(cookieHeader, sessionCookieName+"=") {
		t.Errorf("session cookie not set: %q", cookieHeader)
	}
	if !strings.Contains(cookieHeader, "HttpOnly") {
		t.Errorf("cookie not HttpOnly: %q", cookieHeader)
	}
	if !strings.Contains(strings.ToLower(cookieHeader), "samesite=lax") {
		t.Errorf("cookie not SameSite=Lax: %q", cookieHeader)
	}
}
