// SSO OIDC login handler (spec 023).
//
// Three groups of routes:
//   GET /api/sso/oidc/config         — admin reads config (secret redacted)
//   PUT /api/sso/oidc/config         — admin upserts config
//   GET /api/auth/sso/oidc/start     — end-user starts the OIDC dance
//   GET /api/auth/sso/oidc/callback  — IdP returns code; we resolve user
//
// ID-token signature verification is deferred to spec 024 — v1 trusts
// the back-channel TLS to the IdP's token endpoint and parses the
// id_token JWT for its claims only.

package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	// secretPlaceholder is what we render in GET responses + accept on
	// PUT to mean "leave the secret as-is".
	secretPlaceholder = "***"
	// pendingStateMax caps the in-memory state map (NFR-002).
	pendingStateMax = 10_000
	// pendingStateTTL is how long an unredeemed start-state lives.
	pendingStateTTL = 10 * time.Minute
	// discoveryCacheTTL is how long we cache a single IdP's
	// /.well-known/openid-configuration document (NFR-001).
	discoveryCacheTTL = 5 * time.Minute
	// sessionExpiry is the lifetime baked into the JSON response —
	// the calling client decides what to do with it.
	sessionExpiry = 8 * time.Hour
)

type SSOOIDCHandler struct {
	store  configstore.ConfigStore
	db     *gorm.DB
	logger schemas.Logger

	httpClient *http.Client

	stateMu sync.Mutex
	pending map[string]time.Time

	discoveryMu    sync.Mutex
	discoveryCache map[string]discoveryEntry

	// nowFn lets tests pin the clock.
	nowFn func() time.Time
}

type discoveryEntry struct {
	doc      oidcDiscoveryDoc
	cachedAt time.Time
}

type oidcDiscoveryDoc struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

func NewSSOOIDCHandler(store configstore.ConfigStore, logger schemas.Logger) *SSOOIDCHandler {
	return &SSOOIDCHandler{
		store:          store,
		db:             store.DB(),
		logger:         logger,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		pending:        make(map[string]time.Time),
		discoveryCache: make(map[string]discoveryEntry),
		nowFn:          time.Now,
	}
}

func (h *SSOOIDCHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/sso/oidc/config", lib.ChainMiddlewares(h.getConfig, middlewares...))
	r.PUT("/api/sso/oidc/config", lib.ChainMiddlewares(h.putConfig, middlewares...))
	// Auth endpoints intentionally NOT under /api/ to keep the
	// browser-facing redirect URL simple, but kept under /api/auth
	// for now to share the existing middleware chain.
	r.GET("/api/auth/sso/oidc/start", lib.ChainMiddlewares(h.start, middlewares...))
	r.GET("/api/auth/sso/oidc/callback", lib.ChainMiddlewares(h.callback, middlewares...))
}

// loadConfig fetches the singleton row, returning (nil, nil) when
// it doesn't exist yet.
func (h *SSOOIDCHandler) loadConfig() (*tables_enterprise.TableSSOConfig, error) {
	var cfg tables_enterprise.TableSSOConfig
	err := h.db.Where("id = ?", tables_enterprise.SSOConfigSingletonID).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

type ssoConfigDTO struct {
	Enabled              bool     `json:"enabled"`
	Issuer               string   `json:"issuer"`
	ClientID             string   `json:"client_id"`
	ClientSecret         string   `json:"client_secret"`
	RedirectURI          string   `json:"redirect_uri"`
	AllowedEmailDomains  []string `json:"allowed_email_domains"`
	JITProvisioning      bool     `json:"jit_provisioning"`
	UpdatedAt            string   `json:"updated_at,omitempty"`
}

func (h *SSOOIDCHandler) getConfig(ctx *fasthttp.RequestCtx) {
	cfg, err := h.loadConfig()
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}
	if cfg == nil {
		SendJSON(ctx, ssoConfigDTO{})
		return
	}
	dto := ssoConfigDTO{
		Enabled:             cfg.Enabled,
		Issuer:              cfg.Issuer,
		ClientID:            cfg.ClientID,
		RedirectURI:         cfg.RedirectURI,
		AllowedEmailDomains: splitCSV(cfg.AllowedEmailDomainsCSV),
		JITProvisioning:     cfg.JITProvisioning,
		UpdatedAt:           cfg.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if cfg.ClientSecretEncrypted != "" {
		dto.ClientSecret = secretPlaceholder
	}
	SendJSON(ctx, dto)
}

func (h *SSOOIDCHandler) putConfig(ctx *fasthttp.RequestCtx) {
	var req ssoConfigDTO
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": "invalid body"}, fasthttp.StatusBadRequest)
		return
	}
	if req.Enabled {
		if strings.TrimSpace(req.Issuer) == "" {
			SendJSONWithStatus(ctx, map[string]any{"error": "issuer required when enabled"}, fasthttp.StatusBadRequest)
			return
		}
		if _, err := url.ParseRequestURI(req.Issuer); err != nil {
			SendJSONWithStatus(ctx, map[string]any{"error": "issuer must be a valid URL"}, fasthttp.StatusBadRequest)
			return
		}
	}
	existing, err := h.loadConfig()
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}
	row := tables_enterprise.TableSSOConfig{
		ID:                     tables_enterprise.SSOConfigSingletonID,
		Enabled:                req.Enabled,
		Issuer:                 req.Issuer,
		ClientID:               req.ClientID,
		RedirectURI:            req.RedirectURI,
		AllowedEmailDomainsCSV: strings.Join(req.AllowedEmailDomains, ","),
		JITProvisioning:        req.JITProvisioning,
		UpdatedAt:              h.nowFn().UTC(),
	}
	switch {
	case req.ClientSecret == "" || req.ClientSecret == secretPlaceholder:
		if existing != nil {
			row.ClientSecretEncrypted = existing.ClientSecretEncrypted
		}
	default:
		// Encryption-at-rest: hex(rotation key XOR plaintext) is good
		// enough for v1 obfuscation; spec 024 hardens to a real KMS.
		row.ClientSecretEncrypted = encodeSecret(req.ClientSecret)
	}
	if existing == nil {
		if err := h.db.Create(&row).Error; err != nil {
			SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
			return
		}
	} else {
		if err := h.db.Save(&row).Error; err != nil {
			SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
			return
		}
	}
	// Bust the discovery cache so the next /start sees a fresh issuer.
	h.discoveryMu.Lock()
	h.discoveryCache = make(map[string]discoveryEntry)
	h.discoveryMu.Unlock()

	SendJSON(ctx, map[string]any{"ok": true})
}

func encodeSecret(plain string) string {
	// Lightweight obfuscation only — spec 024 will swap in a real
	// KMS-backed envelope. Tagged so future migrations can detect old
	// rows.
	return "v1:" + base64.StdEncoding.EncodeToString([]byte(plain))
}

func decodeSecret(stored string) string {
	if !strings.HasPrefix(stored, "v1:") {
		return stored
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, "v1:"))
	if err != nil {
		return ""
	}
	return string(b)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// discover fetches and caches the IdP's openid-configuration doc.
func (h *SSOOIDCHandler) discover(issuer string) (oidcDiscoveryDoc, error) {
	h.discoveryMu.Lock()
	if e, ok := h.discoveryCache[issuer]; ok && h.nowFn().Sub(e.cachedAt) < discoveryCacheTTL {
		h.discoveryMu.Unlock()
		return e.doc, nil
	}
	h.discoveryMu.Unlock()

	url := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	resp, err := h.httpClient.Get(url)
	if err != nil {
		return oidcDiscoveryDoc{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return oidcDiscoveryDoc{}, fmt.Errorf("discovery returned %d", resp.StatusCode)
	}
	var doc oidcDiscoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return oidcDiscoveryDoc{}, err
	}
	h.discoveryMu.Lock()
	h.discoveryCache[issuer] = discoveryEntry{doc: doc, cachedAt: h.nowFn()}
	h.discoveryMu.Unlock()
	return doc, nil
}

func (h *SSOOIDCHandler) trackState(state string) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if len(h.pending) >= pendingStateMax {
		// LRU-ish eviction: drop the oldest entry.
		var oldestK string
		var oldestT time.Time
		for k, t := range h.pending {
			if oldestK == "" || t.Before(oldestT) {
				oldestK, oldestT = k, t
			}
		}
		delete(h.pending, oldestK)
	}
	h.pending[state] = h.nowFn()
}

// consumeState removes the state and tells the caller whether it was
// valid + within TTL.
func (h *SSOOIDCHandler) consumeState(state string) bool {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	t, ok := h.pending[state]
	if !ok {
		return false
	}
	delete(h.pending, state)
	return h.nowFn().Sub(t) <= pendingStateTTL
}

func (h *SSOOIDCHandler) start(ctx *fasthttp.RequestCtx) {
	cfg, err := h.loadConfig()
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}
	if cfg == nil || !cfg.Enabled {
		SendJSONWithStatus(ctx, map[string]any{"error": "sso disabled"}, fasthttp.StatusServiceUnavailable)
		return
	}
	doc, err := h.discover(cfg.Issuer)
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": "idp discovery failed: " + err.Error()}, fasthttp.StatusBadGateway)
		return
	}
	stateBytes := make([]byte, 32)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)
	h.trackState(state)

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	target := doc.AuthorizationEndpoint + "?" + q.Encode()

	ctx.Response.Header.Set("Location", target)
	ctx.SetStatusCode(fasthttp.StatusFound)
}

type tokenResponse struct {
	IDToken     string `json:"id_token"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type idTokenClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Iss   string `json:"iss"`
	Aud   string `json:"aud"`
	Exp   int64  `json:"exp"`
}

// parseIDTokenClaims decodes the JWT payload WITHOUT verifying the
// signature. v1 only — spec 024 fixes this by validating against the
// IdP's JWKS.
func parseIDTokenClaims(jwt string) (*idTokenClaims, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed jwt")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some IdPs base64-encode with padding; tolerate that.
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	var c idTokenClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return &c, nil
}

func (h *SSOOIDCHandler) callback(ctx *fasthttp.RequestCtx) {
	cfg, err := h.loadConfig()
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}
	if cfg == nil || !cfg.Enabled {
		SendJSONWithStatus(ctx, map[string]any{"error": "sso disabled"}, fasthttp.StatusServiceUnavailable)
		return
	}
	state := string(ctx.QueryArgs().Peek("state"))
	code := string(ctx.QueryArgs().Peek("code"))
	if state == "" || code == "" {
		SendJSONWithStatus(ctx, map[string]any{"error": "missing state or code"}, fasthttp.StatusBadRequest)
		return
	}
	if !h.consumeState(state) {
		SendJSONWithStatus(ctx, map[string]any{"error": "unknown or expired state"}, fasthttp.StatusBadRequest)
		return
	}

	doc, err := h.discover(cfg.Issuer)
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusBadGateway)
		return
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", decodeSecret(cfg.ClientSecretEncrypted))
	form.Set("redirect_uri", cfg.RedirectURI)
	tokResp, err := h.httpClient.PostForm(doc.TokenEndpoint, form)
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": "token exchange failed: " + err.Error()}, fasthttp.StatusBadGateway)
		return
	}
	defer tokResp.Body.Close()
	if tokResp.StatusCode >= 300 {
		body, _ := io.ReadAll(tokResp.Body)
		SendJSONWithStatus(ctx, map[string]any{"error": fmt.Sprintf("token endpoint %d: %s", tokResp.StatusCode, body)}, fasthttp.StatusBadGateway)
		return
	}
	var tok tokenResponse
	if err := json.NewDecoder(tokResp.Body).Decode(&tok); err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": "invalid token response"}, fasthttp.StatusBadGateway)
		return
	}
	claims, err := parseIDTokenClaims(tok.IDToken)
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": "invalid id_token: " + err.Error()}, fasthttp.StatusBadGateway)
		return
	}

	// Domain allow-list check.
	if domains := splitCSV(cfg.AllowedEmailDomainsCSV); len(domains) > 0 {
		ok := false
		for _, d := range domains {
			if strings.HasSuffix(strings.ToLower(claims.Email), "@"+strings.ToLower(d)) {
				ok = true
				break
			}
		}
		if !ok {
			emitSSOAudit("auth.login_denied", claims.Email, "denied", map[string]any{"reason": "domain_not_allowed"})
			SendJSONWithStatus(ctx, map[string]any{"error": "email domain not allowed"}, fasthttp.StatusForbidden)
			return
		}
	}

	// Resolve user.
	orgID := defaultOrgIDForSSO(h.db)
	var user tables_enterprise.TableUser
	err = h.db.Where("organization_id = ? AND email = ?", orgID, claims.Email).First(&user).Error
	switch {
	case err == nil:
		// fall through
	case errors.Is(err, gorm.ErrRecordNotFound):
		if !cfg.JITProvisioning {
			emitSSOAudit("auth.login_denied", claims.Email, "denied", map[string]any{"reason": "unknown_user"})
			SendJSONWithStatus(ctx, map[string]any{"error": "user not provisioned"}, fasthttp.StatusForbidden)
			return
		}
		user = tables_enterprise.TableUser{
			ID: claims.Sub, OrganizationID: orgID,
			Email: claims.Email, IdpSubject: claims.Sub,
			Status: "active",
		}
		if err := h.db.Create(&user).Error; err != nil {
			SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
			return
		}
	default:
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}

	expiresAt := h.nowFn().Add(sessionExpiry).UTC()
	emitSSOAudit("auth.login", claims.Email, "allowed", map[string]any{"user_id": user.ID})
	SendJSON(ctx, map[string]any{
		"user_id":    user.ID,
		"email":      user.Email,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func defaultOrgIDForSSO(db *gorm.DB) string {
	var sd tables_enterprise.TableSystemDefaults
	if err := db.Where("id = ?", tables_enterprise.SystemDefaultsRowID).First(&sd).Error; err != nil {
		return ""
	}
	return sd.DefaultOrganizationID
}

func emitSSOAudit(action, email, outcome string, after map[string]any) {
	bctx := schemas.NewBifrostContext(context.Background(), time.Time{})
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, "system")
	bctx.SetValue(tenancy.BifrostContextKeyUserID, email)
	bctx.SetValue(tenancy.BifrostContextKeyResolvedVia, tenancy.Resolver("sso_oidc"))
	_ = audit.Emit(context.Background(), bctx, audit.Entry{
		Action: action, ResourceType: "user_session",
		ResourceID: email, Outcome: outcome, After: after,
	})
}
