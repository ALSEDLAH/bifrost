# Feature Specification: Session middleware + /me + /logout

**Feature Branch**: `025-session-middleware`
**Created**: 2026-04-21
**Status**: Draft

## Overview

Spec 024 issued `bf_session` cookies + JSON `session_token` after a
successful OIDC callback, and exported `VerifySessionToken`. Spec
025 turns that primitive into a usable session by:

1. Providing a reusable `RequireSession` middleware that resolves
   either the cookie or an `Authorization: Bearer <session_token>`
   header into a user_id user-value on the ctx, or rejects with 401.
2. Wiring two new endpoints behind that middleware:
   - `GET /api/auth/me` — echo the current user from the session.
   - `POST /api/auth/logout` — clear the cookie (stateless, since
     there is no server-side session table yet).

The middleware is **opt-in** — existing routes (SCIM bearer auth,
governance HMAC, etc.) keep their current auth. Any handler that
opts in declares it explicitly via `RegisterRoutes`.

## User Scenarios

### US1 — UI fetches the current user (P1)

**As a** logged-in employee
**I want** the dashboard to fetch `/api/auth/me` after the OIDC
callback set my session cookie
**So that** the UI knows my user_id + email without having to
re-parse the callback response.

### US2 — Unauthenticated request is rejected (P1)

**As a** security engineer
**I want** `/api/auth/me` to return 401 if the cookie is missing,
malformed, expired, or tampered
**So that** API access requires a valid session.

### US3 — User logs out (P2)

**As a** logged-in employee
**I want** `POST /api/auth/logout` to clear my cookie
**So that** the next /me call returns 401.

## Functional Requirements

- **FR-001**: `RequireSession` returns a `BifrostHTTPMiddleware`. On
  invocation:
  1. Reads `bf_session` cookie.
  2. If absent, falls back to `Authorization: Bearer <token>`.
  3. If both empty → 401 with `{error:"unauthenticated"}`.
  4. Calls `VerifySessionToken(token)`. On failure → 401 with
     `{error:"invalid_session"}`.
  5. On success, calls `ctx.SetUserValue("user_id", uid)` and
     `ctx.SetUserValue("session_expires_at", expiresAt)` then
     invokes `next(ctx)`.
- **FR-002**: `GET /api/auth/me` is registered behind `RequireSession`.
  Reads `user_id` from ctx, fetches the row from `ent_users`, and
  returns `{id, email, display_name, status, organization_id,
  expires_at}`. Returns 404 if the user_id no longer maps to a row
  (i.e., user was deleted after sign-in).
- **FR-003**: `POST /api/auth/logout` clears `bf_session` by setting
  the cookie with `Max-Age=0; Path=/`. Returns 204. Idempotent
  (does NOT require a valid session — clearing an already-cleared
  cookie still 204s).
- **FR-004**: Audit emits `auth.me` (outcome=allowed) on the /me
  endpoint and `auth.logout` on /logout, with the user email when
  available.

## Non-Functional Requirements

- **NFR-001**: Middleware adds <1ms overhead per request (HMAC
  verify is ~10µs).

## Success Criteria

- **SC-001**: After spec 024 callback, `GET /api/auth/me` with the
  cookie returns the user record.
- **SC-002**: With a tampered session token, `/api/auth/me` returns
  401 — never reveals which check failed (uniform error envelope).
- **SC-003**: After `/logout`, the same browser hitting `/me`
  returns 401.

## Out of Scope (v1)

- Server-side session revocation table (forced logout across
  devices). Future spec.
- Refresh-token rotation.
- Applying RequireSession globally to existing API surfaces — each
  handler opts in, audit-trail-style.
- CSRF protection on /logout — `bf_session` is already
  `SameSite=Lax`, which Okta + similar consider sufficient for
  logout; a future spec adds the X-CSRF-Token pattern.
