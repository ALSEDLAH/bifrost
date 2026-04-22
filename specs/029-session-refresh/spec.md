# Feature Specification: Session refresh endpoint

**Feature Branch**: `029-session-refresh`
**Created**: 2026-04-22
**Status**: Draft

## Overview

Spec 024 issues `bf_session` cookies with a fixed 8h lifetime. Once
that window elapses the user has to complete the full OIDC round
trip again, even if they were actively using the app moments before.

Spec 029 adds `POST /api/auth/sessions/refresh` — a single-hop
re-issue that takes a still-valid session and returns a fresh token
with a new 8h expiry. It reuses every spec 025/026 protection:
RequireSession covers token verification + revocation check, so
there is no separate auth path to harden.

## User Scenarios

### US1 — Active user extends their session (P1)

**As a** logged-in user hitting the dashboard every 10 minutes
**I want** the UI to silently POST `/api/auth/sessions/refresh`
before my 8h cookie expires and receive a new one
**So that** I can keep working past the 8h ceiling without
completing another full OIDC redirect.

### US2 — Revoked user can't refresh (P1)

**As a** security engineer
**I want** the refresh endpoint to return 401 once the user's
session has been revoked by spec 026
**So that** refresh cannot outlive a forced logout.

## Functional Requirements

- **FR-001**: `POST /api/auth/sessions/refresh` MUST be gated by
  `RequireSession`. Any failure of the middleware (missing token,
  tampered, expired, revoked) propagates its 401 uniformly.
- **FR-002**: On success the handler MUST:
  1. Read `user_id` from ctx.
  2. Call `IssueSessionToken(user_id, now + sessionExpiry)` for a
     new 8h token.
  3. Set the `bf_session` cookie with the same attributes as the
     spec-024 callback (HttpOnly, Secure, SameSite=Lax, Max-Age =
     8h).
  4. Return JSON `{user_id, session_token, expires_at (RFC3339)}`.
- **FR-003**: `audit.Emit` fires `auth.session_refreshed`
  (outcome=allowed) with `{user_id}`.
- **FR-004**: Refresh MUST NOT extend a token beyond the normal 8h
  expiry — the new token is a full-lifetime re-issue, not a cumulative
  extension. A user who refreshes 7h 59m into the session gets a
  fresh 8h window.

## Non-Functional Requirements

- **NFR-001**: Handler adds <1ms after the middleware overhead
  (HMAC sign is ~10µs).

## Success Criteria

- **SC-001**: Given a valid cookie, POST /api/auth/sessions/refresh
  returns 200 + a session_token that passes
  `VerifySessionToken(...)` and whose expiry is strictly later than
  the original cookie's expiry.
- **SC-002**: After `/api/auth/sessions/revoke-self` (spec 026),
  refresh returns 401.
- **SC-003**: A missing/invalid cookie returns 401.

## Out of Scope (v1)

- Refresh-token rotation (OAuth semantics) — out of scope; we
  re-issue the same cookie format, not introduce a separate
  refresh token.
- Absolute session-lifetime caps (e.g., "must re-login every 30
  days regardless of refresh") — follow-up spec.
- Silent refresh headers (X-Renew-Before) — the UI decides the
  cadence; server is stateless about it.
