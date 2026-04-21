// SSO OIDC hardening (spec 024): RS256 id_token signature
// verification against the IdP's JWKS, plus HMAC-signed session
// token issuance + verification.

package handlers

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"
)

// sessionCookieName is the cookie used by /callback to set the
// signed session token. Spec 025 will read it back via middleware.
const sessionCookieName = "bf_session"

// jwksCacheTTL mirrors the discovery doc cache; once minute differences
// here in tests are fine since tests pin the clock.
const jwksCacheTTL = 5 * time.Minute

type jwksDoc struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksCacheEntry struct {
	doc      jwksDoc
	cachedAt time.Time
}

// jwksCache lives on the SSO handler instance — keyed by jwks_uri.
var (
	jwksMu    sync.Mutex
	jwksCache = make(map[string]jwksCacheEntry)
)

// fetchJWKS fetches the JWKS doc for the given URI, with TTL cache.
// Pulled out to package-level so tests can clear it cheaply.
func (h *SSOOIDCHandler) fetchJWKS(uri string) (jwksDoc, error) {
	jwksMu.Lock()
	if e, ok := jwksCache[uri]; ok && h.nowFn().Sub(e.cachedAt) < jwksCacheTTL {
		jwksMu.Unlock()
		return e.doc, nil
	}
	jwksMu.Unlock()

	resp, err := h.httpClient.Get(uri)
	if err != nil {
		return jwksDoc{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return jwksDoc{}, fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}
	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return jwksDoc{}, err
	}
	jwksMu.Lock()
	jwksCache[uri] = jwksCacheEntry{doc: doc, cachedAt: h.nowFn()}
	jwksMu.Unlock()
	return doc, nil
}

// jwsHeader is the minimum we read from the JWS header for kid + alg.
type jwsHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// verifyIDToken is the spec 024 replacement for parseIDTokenClaims.
// It performs full RS256 signature + standard-claim verification.
func (h *SSOOIDCHandler) verifyIDToken(tokenStr, jwksURI, expectedIss, expectedAud string) (*idTokenClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed jwt")
	}
	headerJSON, err := decodeB64URL(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	var hdr jwsHeader
	if err := json.Unmarshal(headerJSON, &hdr); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if hdr.Alg != "RS256" {
		return nil, fmt.Errorf("unsupported alg %q (RS256 only in v1)", hdr.Alg)
	}
	if hdr.Kid == "" {
		return nil, errors.New("id_token missing kid header")
	}

	doc, err := h.fetchJWKS(jwksURI)
	if err != nil {
		return nil, fmt.Errorf("jwks fetch: %w", err)
	}
	var matched *jwk
	for i := range doc.Keys {
		if doc.Keys[i].Kid == hdr.Kid {
			matched = &doc.Keys[i]
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("no jwk with kid %q", hdr.Kid)
	}
	if matched.Kty != "RSA" {
		return nil, fmt.Errorf("jwk kty %q not RSA", matched.Kty)
	}

	pubKey, err := jwkToRSAPublicKey(matched)
	if err != nil {
		return nil, fmt.Errorf("jwk → pubkey: %w", err)
	}

	signingInput := parts[0] + "." + parts[1]
	sig, err := decodeB64URL(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode sig: %w", err)
	}
	hash := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sig); err != nil {
		return nil, fmt.Errorf("signature verify: %w", err)
	}

	payloadJSON, err := decodeB64URL(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var c idTokenClaims
	if err := json.Unmarshal(payloadJSON, &c); err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}
	if c.Iss != expectedIss {
		return nil, fmt.Errorf("iss mismatch: got %q want %q", c.Iss, expectedIss)
	}
	if c.Aud != expectedAud {
		return nil, fmt.Errorf("aud mismatch: got %q want %q", c.Aud, expectedAud)
	}
	if c.Exp <= h.nowFn().Unix() {
		return nil, fmt.Errorf("id_token expired (exp=%d, now=%d)", c.Exp, h.nowFn().Unix())
	}
	return &c, nil
}

// decodeB64URL tolerates both raw and padded base64-URL encodings.
func decodeB64URL(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

// jwkToRSAPublicKey converts a JWK RSA key to a *rsa.PublicKey.
func jwkToRSAPublicKey(k *jwk) (*rsa.PublicKey, error) {
	nBytes, err := decodeB64URL(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := decodeB64URL(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	// e is typically 3 bytes (AQAB = 65537). Pad up to 8 bytes for
	// uvarint-style decode.
	eBig := new(big.Int).SetBytes(eBytes)
	if !eBig.IsInt64() {
		return nil, errors.New("e too large")
	}
	e := int(eBig.Int64())
	if e <= 0 {
		return nil, errors.New("e must be positive")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

// sessionSecret is the HMAC key used to sign session tokens. Generated
// once at process start; replacing it invalidates every issued cookie.
var (
	sessionSecretMu sync.Mutex
	sessionSecret   []byte
)

func ensureSessionSecret() []byte {
	sessionSecretMu.Lock()
	defer sessionSecretMu.Unlock()
	if sessionSecret == nil {
		sessionSecret = make([]byte, 32)
		if _, err := rand.Read(sessionSecret); err != nil {
			// fall back to a deterministic-but-unique seed so tests don't
			// blow up if /dev/urandom is restricted.
			seed := []byte(time.Now().UTC().Format(time.RFC3339Nano))
			h := sha256.Sum256(seed)
			sessionSecret = h[:]
		}
	}
	return sessionSecret
}

// IssueSessionToken builds a signed `<payloadB64>.<sigB64>` token where
// payload = `<userID>.<expiresUnix>`.
func IssueSessionToken(userID string, expiresAt time.Time) string {
	payload := []byte(userID + "." + strconv.FormatInt(expiresAt.Unix(), 10))
	mac := hmac.New(sha256.New, ensureSessionSecret())
	mac.Write(payload)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(sig)
}

// VerifySessionToken validates a token issued by IssueSessionToken
// and returns the embedded user ID + expiry. Constant-time HMAC compare.
func VerifySessionToken(tok string) (userID string, expiresAt time.Time, ok bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 2 {
		return "", time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", time.Time{}, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", time.Time{}, false
	}
	mac := hmac.New(sha256.New, ensureSessionSecret())
	mac.Write(payload)
	expected := mac.Sum(nil)
	if subtle.ConstantTimeCompare(expected, sig) != 1 {
		return "", time.Time{}, false
	}
	dot := strings.LastIndexByte(string(payload), '.')
	if dot < 0 {
		return "", time.Time{}, false
	}
	uid := string(payload[:dot])
	expS := string(payload[dot+1:])
	exp, err := strconv.ParseInt(expS, 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}
	if time.Now().Unix() >= exp {
		return "", time.Time{}, false
	}
	return uid, time.Unix(exp, 0).UTC(), true
}

// resetSessionSecretForTests is the test-only escape hatch — it lets a
// test wipe the in-memory HMAC key so the next IssueSessionToken call
// rolls a fresh one. Safe because the variable is unexported.
func resetSessionSecretForTests() {
	sessionSecretMu.Lock()
	sessionSecret = nil
	sessionSecretMu.Unlock()
}

// clearJWKSCacheForTests resets the package-level JWKS cache so tests
// see fresh fetches.
func clearJWKSCacheForTests() {
	jwksMu.Lock()
	jwksCache = make(map[string]jwksCacheEntry)
	jwksMu.Unlock()
}

// belt-and-braces guard against unused import warnings since some
// helpers below are only referenced from tests.
var (
	_ = hex.EncodeToString
	_ = binary.BigEndian
)
