# Feature Specification: Session revocation (forced logout)

**Feature Branch**: `026-session-revocation`
**Created**: 2026-04-22
**Status**: Draft

## Overview

Spec 025 issues HMAC-signed `bf_session` cookies that expire 8h
after the OIDC callback. Until that 8h elapses there is no way to
forcibly invalidate a stolen or no-longer-trusted token — clearing
the cookie via /logout only nukes the browser's copy.

Spec 026 closes that gap with a tiny server-side revocation list,
keyed by user_id (NOT by individual token, so it works without
adding a JTI to the existing token format). When a user is
revoked, every session token whose implied issue-time (= exp − 8h)
predates the revocation timestamp is rejected by RequireSession.

## User Scenarios

### US1 — Admin force-logs-out a compromised account (P1)

**As an** IT admin
**I want** to POST `/api/auth/sessions/revoke?user_id=<x>` and
have every existing bf_session for that user fail on the next
/me call
**So that** I can contain a credential compromise without waiting
8h for the natural expiry.

### US2 — User self-revokes (P1)

**As a** user who left their session open at an internet cafe
**I want** to POST `/api/auth/sessions/revoke-self` from a trusted
device and have my still-open browser tabs lose access on next
request
**So that** I do not have to wait until the cookie expires.

### US3 — Re-authenticating after revoke restores access (P2)

**As a** revoked user
**I want** a fresh OIDC login flow to issue a new token whose
implied iat is *after* the revocation timestamp, so that token
works
**So that** revocation behaves like a "log out everywhere", not a
permanent ban.

## Functional Requirements

- **FR-001**: New table `ent_session_revocations` with columns
  `{user_id (pk + fk), revoked_at}`. Singleton per user — repeated
  revoke calls update `revoked_at` to the latest timestamp.
- **FR-002**: `POST /api/auth/sessions/revoke-self` — requires a
  valid session via the spec-025 RequireSession middleware. Upserts
  the row with `revoked_at = now`. Returns 204.
- **FR-003**: `POST /api/auth/sessions/revoke?user_id=<x>` —
  requires a valid session AND the caller's user_id must be a
  member of a role named "Owner" or "Admin" (the seed roles from
  E004). On unauthorized → 403. On success → 204. Returns 400 if
  `user_id` is missing.
- **FR-004**: `RequireSession` (extended) MUST consult the
  revocation list after token verification. The token's implied
  issue-time = `expiresAt − sessionExpiry`. If a revocation row
  exists for the resolved user_id with
  `revoked_at >= implied_iat`, return 401 with
  `{error:"session_revoked"}`.
- **FR-005**: Revocation lookups MUST be cheap — cache the per-user
  revocation timestamp in-memory (sync.Map, TTL 30s) so a steady
  stream of /me calls does not hit the DB on every request.
  Cache is invalidated on revoke.
- **FR-006**: `audit.Emit` fires `auth.session_revoked` with the
  caller's user_id + the targeted user_id.

## Non-Functional Requirements

- **NFR-001**: Revocation lookup adds <100µs per request when the
  cache is warm.
- **NFR-002**: The revocation list MUST survive process restarts
  (it lives in the configstore, not in process memory).

## Success Criteria

- **SC-001**: After admin revokes user X, X's existing /me calls
  return 401 within 30s (cache TTL).
- **SC-002**: User X re-authenticates → new token works.
- **SC-003**: Self-revocation works for any logged-in user.
- **SC-004**: Non-admin users get 403 when targeting another
  user_id.

## Out of Scope (v1)

- Per-token (JTI) revocation — would require changing the spec-024
  token format. Per-user is sufficient for credential-rotation use
  cases.
- Distributed cache invalidation across multiple Bifrost nodes (the
  cache is per-process; cross-node revocation propagates within
  30s via the DB lookup).
- "Expire all sessions globally" admin button.
- Revocation reason metadata.

## Key Entities

- **ent_session_revocations**: `{user_id (pk), revoked_at}`. One
  row per user; updated in place on each revocation.
