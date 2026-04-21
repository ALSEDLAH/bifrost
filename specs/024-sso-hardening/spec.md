# Feature Specification: SSO OIDC Hardening — JWKS verify + session cookies

**Feature Branch**: `024-sso-hardening`
**Created**: 2026-04-21
**Status**: Draft

## Overview

Spec 023 shipped the OIDC round-trip but explicitly skipped two
production blockers:

1. The id_token's signature was NOT verified — any party that
   could MITM the back-channel could mint claims.
2. The callback returned `{user_id, email, expires_at}` as JSON
   without issuing anything the calling client could use to make
   later requests.

Spec 024 closes both gaps:

- Cryptographically verify the id_token against the IdP's JWKS
  (RS256 only — covers Okta / Azure AD / Google).
- Verify standard claims (iss, aud, exp).
- Issue a signed session token after a successful callback. Token
  format: HMAC-SHA256 over `<user_id>.<expires_at_unix>` with a
  startup-generated secret. Token is set as an `HttpOnly; Secure`
  cookie AND returned in the JSON body for non-browser clients.
- Expose `VerifySessionToken(string) (userID, ok)` so future
  middleware (spec 025) can gate API requests.

## User Scenarios

### US1 — Forged id_token is rejected (P1)

**As a** security engineer  
**I want** an id_token with a tampered payload (or wrong signing
key) to be rejected at /callback with 401  
**So that** downstream user-resolution never sees forged claims.

### US2 — Session cookie roundtrips (P1)

**As an** end user  
**I want** the cookie set on /callback to be presentable to a
later VerifySessionToken call and resolve to my user_id  
**So that** I can stay logged in across requests once spec 025
wires the middleware.

### US3 — Expired session is rejected (P1)

**As a** security engineer  
**I want** a session token whose expiry has passed to fail
verification  
**So that** stale credentials cannot be replayed.

## Functional Requirements

- **FR-001**: After token exchange, the handler MUST fetch the
  IdP's JWKS from `discoveryDoc.jwks_uri` (cached 5 min) and look
  up the JWK whose `kid` matches the id_token JWS header `kid`.
  RS256 only — other algs return 401.
- **FR-002**: The handler MUST verify the id_token signature
  using the JWK's modulus + exponent. Verification failure → 401
  with `{error: "invalid_id_token"}`.
- **FR-003**: After signature verification, the handler MUST
  validate: `iss == config.issuer`, `aud == config.client_id`,
  `exp > now`. Any failure → 401 with the specific reason.
- **FR-004**: On successful verification + user resolution, the
  handler MUST issue a session token with format
  `base64url(<user_id>.<expires_unix>).<base64url(hmac256)>`.
  Server-side HMAC secret is generated once at process start
  (32 random bytes) and held in memory.
- **FR-005**: The session token MUST be set as
  `Set-Cookie: bf_session=<tok>; HttpOnly; Secure; SameSite=Lax;
  Path=/; Max-Age=28800` AND returned in the JSON body as
  `session_token`. JSON body still includes `user_id`, `email`,
  `expires_at` (RFC3339).
- **FR-006**: Public function `VerifySessionToken(tok string)
  (userID string, expiresAt time.Time, ok bool)` MUST be exported
  from the same package. Returns `ok=false` when:
  - Token is malformed
  - HMAC fails constant-time compare
  - Expiry is in the past
- **FR-007**: `audit.Emit` continues to fire on success/denied as
  in spec 023.

## Non-Functional Requirements

- **NFR-001**: JWKS fetch is cached per-issuer (TTL 5 min) using
  the same cache as the discovery doc.
- **NFR-002**: HMAC compare MUST use `crypto/subtle.ConstantTimeCompare`.

## Success Criteria

- **SC-001**: Stub IdP signing with a known RSA key produces a
  token that callback verifies + accepts.
- **SC-002**: Tampering any byte of the payload (then re-base64'ing)
  with the same signature → 401.
- **SC-003**: A token signed by a different RSA key but with the
  correct kid → 401.
- **SC-004**: VerifySessionToken roundtrips for a freshly issued
  token; same token rejected after artificially advancing the
  clock past expiry; same token rejected after one byte flip.

## Out of Scope (v1)

- ES256 / EdDSA / HS256 id_tokens — RS256 covers >99% of IdPs.
- Refresh-token flows.
- Cookie-encrypted payloads (AES-GCM) — HMAC-only is fine for
  user_id (not a secret).
- Persisting session tokens server-side (revocation list) —
  follow-up spec.
- Wiring session enforcement into existing API middleware —
  spec 025.
