// SSO OIDC handler tests (specs 023 + 024). The stub IdP RSA-signs
// id_tokens so it exercises the spec-024 verifier end-to-end.

package handlers

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// signingKey holds the RSA private key + matching kid the stub IdP
// uses to sign tokens.
type signingKey struct {
	priv *rsa.PrivateKey
	kid  string
}

// b64u is shorthand for raw-URL base64 encode.
func b64u(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// signRS256 builds and RSA-signs a JWT with the given header overrides
// and payload claims.
func signRS256(t *testing.T, k *signingKey, headerOverride map[string]string, payload map[string]any) string {
	t.Helper()
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": k.kid}
	for kk, vv := range headerOverride {
		header[kk] = vv
	}
	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	signingInput := b64u(hb) + "." + b64u(pb)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k.priv, crypto.SHA256, sum[:])
	require.NoError(t, err)
	return signingInput + "." + b64u(sig)
}

// stubIdP returns an httptest.Server that publishes a discovery doc,
// a JWKS doc backed by a fresh RSA key, and a token endpoint that
// signs claims with that key. Returns the server + the signing key
// so individual tests can mint extra tokens.
func stubIdP(t *testing.T, sub, email string) (*httptest.Server, *signingKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	key := &signingKey{priv: priv, kid: "test-kid"}

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		nBytes := key.priv.N.Bytes()
		eBytes := big.NewInt(int64(key.priv.E)).Bytes()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kid": key.kid, "kty": "RSA", "use": "sig", "alg": "RS256",
				"n": b64u(nBytes), "e": b64u(eBytes),
			}},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		idTok := signRS256(t, key, nil, map[string]any{
			"sub": sub, "email": email,
			"iss": srv.URL, "aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id_token":     idTok,
			"access_token": "ignored",
			"token_type":   "Bearer",
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, key
}

// newSSOTestStore: same shape as the SCIM test store but separately
// declared to avoid coupling.
func newSSOTestStore(t *testing.T) configstore.ConfigStore {
	t.Helper()
	dir := t.TempDir()
	store, err := configstore.NewConfigStore(context.Background(), &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: filepath.Join(dir, "cs.db")},
	}, &mockLogger{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	require.NoError(t, configstore.RegisterEnterpriseMigrations(context.Background(), store.DB()))
	return store
}

// pinDefaultOrg pins the synthetic default org id used by the SSO
// handler when resolving user rows.
func pinDefaultOrgForSSO(t *testing.T, store configstore.ConfigStore, orgID string) {
	t.Helper()
	require.NoError(t, store.DB().
		Model(&tables_enterprise.TableSystemDefaults{}).
		Where("id = ?", tables_enterprise.SystemDefaultsRowID).
		Update("default_organization_id", orgID).Error)
}

func enableSSO(t *testing.T, h *SSOOIDCHandler, issuer string, jit bool, allowedDomains []string) {
	t.Helper()
	dto := ssoConfigDTO{
		Enabled: true, Issuer: issuer,
		ClientID: "test-client", ClientSecret: "test-secret",
		RedirectURI:         "http://localhost/cb",
		AllowedEmailDomains: allowedDomains,
		JITProvisioning:     jit,
	}
	body, _ := json.Marshal(dto)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	h.putConfig(ctx)
	if ctx.Response.StatusCode() >= 300 {
		t.Fatalf("enableSSO PUT failed: %d %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestSSO_GetConfig_RedactsSecret(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	enableSSO(t, h, "https://example.com", false, nil)

	ctx := &fasthttp.RequestCtx{}
	h.getConfig(ctx)
	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("status %d", ctx.Response.StatusCode())
	}
	var out ssoConfigDTO
	_ = json.Unmarshal(ctx.Response.Body(), &out)
	if out.ClientSecret != secretPlaceholder {
		t.Errorf("secret should be redacted; got %q", out.ClientSecret)
	}
	if out.ClientID != "test-client" {
		t.Errorf("client_id missing: %+v", out)
	}
}

func TestSSO_PutConfig_RotateSecretOnNonPlaceholder(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	enableSSO(t, h, "https://example.com", false, nil)

	// PUT with placeholder leaves secret intact.
	dto := ssoConfigDTO{
		Enabled: true, Issuer: "https://example.com",
		ClientID: "new-id", ClientSecret: secretPlaceholder,
		RedirectURI: "http://localhost/cb",
	}
	body, _ := json.Marshal(dto)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	h.putConfig(ctx)
	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("PUT status %d", ctx.Response.StatusCode())
	}
	cfg, _ := h.loadConfig()
	if decodeSecret(cfg.ClientSecretEncrypted) != "test-secret" {
		t.Errorf("placeholder PUT should leave secret untouched; got %q", decodeSecret(cfg.ClientSecretEncrypted))
	}

	// PUT with new secret rotates.
	dto.ClientSecret = "fresh-secret"
	body, _ = json.Marshal(dto)
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	h.putConfig(ctx)
	cfg, _ = h.loadConfig()
	if decodeSecret(cfg.ClientSecretEncrypted) != "fresh-secret" {
		t.Errorf("rotation failed; got %q", decodeSecret(cfg.ClientSecretEncrypted))
	}
}

func TestSSO_PutConfig_RejectsEnabledWithoutIssuer(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	dto := ssoConfigDTO{Enabled: true, ClientID: "x"}
	body, _ := json.Marshal(dto)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	h.putConfig(ctx)
	if ctx.Response.StatusCode() != 400 {
		t.Errorf("expected 400; got %d", ctx.Response.StatusCode())
	}
}

func TestSSO_Start_RedirectsToIdP(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	idp, _ := stubIdP(t, "user-1", "fred@example.com")
	enableSSO(t, h, idp.URL, false, nil)

	ctx := &fasthttp.RequestCtx{}
	h.start(ctx)
	if ctx.Response.StatusCode() != 302 {
		t.Fatalf("expected 302; got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	loc := string(ctx.Response.Header.Peek("Location"))
	if !strings.Contains(loc, "/authorize?") || !strings.Contains(loc, "state=") {
		t.Errorf("redirect URL malformed: %s", loc)
	}
	if !strings.Contains(loc, "client_id=test-client") {
		t.Errorf("client_id missing from redirect: %s", loc)
	}
}

func TestSSO_Start_DisabledReturns503(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	ctx := &fasthttp.RequestCtx{}
	h.start(ctx)
	if ctx.Response.StatusCode() != 503 {
		t.Errorf("expected 503; got %d", ctx.Response.StatusCode())
	}
}

// runFullCallback: fire /start, scrape state from the redirect, then
// hit /callback with that state + a fake code. Returns (status, body).
func runFullCallback(t *testing.T, h *SSOOIDCHandler) (int, map[string]any) {
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
	require.NotEmpty(t, state, "no state in redirect URL")

	cbCtx := &fasthttp.RequestCtx{}
	cbCtx.Request.SetRequestURI("/api/auth/sso/oidc/callback?state=" + state + "&code=test-code")
	h.callback(cbCtx)
	var out map[string]any
	_ = json.Unmarshal(cbCtx.Response.Body(), &out)
	return cbCtx.Response.StatusCode(), out
}

func TestSSO_Callback_ResolvesExistingUser(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	idp, _ := stubIdP(t, "user-1", "fred@example.com")
	enableSSO(t, h, idp.URL, false, nil)
	pinDefaultOrgForSSO(t, store, "org-x")
	require.NoError(t, store.DB().Create(&tables_enterprise.TableUser{
		ID: "u-fred", OrganizationID: "org-x",
		Email: "fred@example.com", Status: "active", IdpSubject: "fred-sub",
	}).Error)

	status, out := runFullCallback(t, h)
	if status != 200 {
		t.Fatalf("status %d body %+v", status, out)
	}
	if out["user_id"] != "u-fred" {
		t.Errorf("user_id wrong: %+v", out["user_id"])
	}
	if out["email"] != "fred@example.com" {
		t.Errorf("email wrong: %+v", out["email"])
	}
	if _, ok := out["expires_at"].(string); !ok {
		t.Errorf("expires_at missing or not a string: %+v", out["expires_at"])
	}
}

func TestSSO_Callback_UnknownUser_NoJIT_Returns403(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	idp, _ := stubIdP(t, "ghost-sub", "ghost@example.com")
	enableSSO(t, h, idp.URL, false, nil) // JIT off
	pinDefaultOrgForSSO(t, store, "org-x")

	status, out := runFullCallback(t, h)
	if status != 403 {
		t.Fatalf("expected 403; got %d body %+v", status, out)
	}
}

func TestSSO_Callback_UnknownUser_JITOn_CreatesUser(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	idp, _ := stubIdP(t, "jit-sub", "jit@example.com")
	enableSSO(t, h, idp.URL, true, nil) // JIT on
	pinDefaultOrgForSSO(t, store, "org-x")

	status, out := runFullCallback(t, h)
	if status != 200 {
		t.Fatalf("expected 200; got %d body %+v", status, out)
	}
	if out["email"] != "jit@example.com" {
		t.Errorf("jit user not returned: %+v", out)
	}
	var u tables_enterprise.TableUser
	require.NoError(t, store.DB().Where("email = ?", "jit@example.com").First(&u).Error)
	if u.OrganizationID != "org-x" {
		t.Errorf("jit user org wrong: %+v", u)
	}
}

func TestSSO_Callback_RejectsUnknownState(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	idp, _ := stubIdP(t, "x", "x@example.com")
	enableSSO(t, h, idp.URL, false, nil)

	cbCtx := &fasthttp.RequestCtx{}
	cbCtx.Request.SetRequestURI("/api/auth/sso/oidc/callback?state=notreal&code=anything")
	h.callback(cbCtx)
	if cbCtx.Response.StatusCode() != 400 {
		t.Errorf("expected 400 for unknown state; got %d", cbCtx.Response.StatusCode())
	}
}

func TestSSO_Callback_StateIsSingleUse(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	idp, _ := stubIdP(t, "user-1", "fred@example.com")
	enableSSO(t, h, idp.URL, false, nil)
	pinDefaultOrgForSSO(t, store, "org-x")
	require.NoError(t, store.DB().Create(&tables_enterprise.TableUser{
		ID: "u-fred", OrganizationID: "org-x",
		Email: "fred@example.com", Status: "active", IdpSubject: "fred-sub",
	}).Error)

	// First callback OK.
	status, _ := runFullCallback(t, h)
	if status != 200 {
		t.Fatalf("first callback failed: %d", status)
	}
	// Try replaying the same callback URL by hand — but we don't have
	// the state any more, so just assert that the in-memory map is empty.
	h.stateMu.Lock()
	left := len(h.pending)
	h.stateMu.Unlock()
	if left != 0 {
		t.Errorf("state should be consumed; %d pending entries left", left)
	}
}

func TestSSO_Callback_DomainAllowList_BlocksOther(t *testing.T) {
	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	idp, _ := stubIdP(t, "x-sub", "x@evil.com")
	enableSSO(t, h, idp.URL, true, []string{"example.com"})
	pinDefaultOrgForSSO(t, store, "org-x")

	status, out := runFullCallback(t, h)
	if status != 403 {
		t.Fatalf("expected 403 for blocked domain; got %d body %+v", status, out)
	}
}

func TestSSO_DiscoveryCacheReused(t *testing.T) {
	hits := 0
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/auth",
			"token_endpoint":         srv.URL + "/tok",
		})
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	store := newSSOTestStore(t)
	h := NewSSOOIDCHandler(store, &mockLogger{})
	enableSSO(t, h, srv.URL, false, nil)

	for i := 0; i < 3; i++ {
		ctx := &fasthttp.RequestCtx{}
		h.start(ctx)
		if ctx.Response.StatusCode() != 302 {
			t.Fatalf("start %d status %d", i, ctx.Response.StatusCode())
		}
	}
	if hits != 1 {
		t.Errorf("expected 1 discovery hit (others cached); got %d", hits)
	}
}

// silenced-unused suppression for fmt
var _ = fmt.Sprintf
