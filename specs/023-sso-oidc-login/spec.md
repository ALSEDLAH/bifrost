# Feature Specification: SSO OIDC Login (config + auth flow)

**Feature Branch**: `023-sso-oidc-login`
**Created**: 2026-04-21
**Status**: Draft

## Overview

Specs 020/021/022 push Users + Groups into Bifrost from an IdP via
SCIM. Spec 023 closes the loop on the inbound side: admins point
Bifrost at an OIDC IdP (Okta / Azure AD / Google), and end users
authenticate via the IdP's authorization-code flow. JIT user
provisioning is OFF by default in v1 — the user must already exist
in `ent_users` (typically pushed via SCIM beforehand).

Session-cookie issuance (signed JWT, middleware integration) is
deliberately deferred to spec 024 so this spec can ship the OIDC
flow end-to-end without entangling auth-middleware refactors.

## User Scenarios

### US1 — Admin configures the OIDC IdP (P1)

**As an** IT admin  
**I want** to PUT `/api/sso/oidc/config` with the IdP issuer URL,
client_id, client_secret, and redirect_uri  
**So that** Bifrost can discover the IdP's OIDC endpoints + verify
incoming ID tokens.

### US2 — Employee authenticates via Okta (P1)

**As an** employee whose row exists in `ent_users`  
**I want** to hit `/api/auth/sso/oidc/start`, get redirected to
Okta, sign in there, get redirected back to
`/api/auth/sso/oidc/callback?code=&state=`, and receive
`{user_id, email, expires_at}` back  
**So that** the calling client knows who I am and can mint its own
session.

### US3 — Unknown user is rejected (P1)

**As a** security-conscious admin  
**I want** OIDC callback to return 403 when the IdP's email isn't
in `ent_users` (and JIT is off)  
**So that** an attacker cannot register a new account just by
authenticating to our OIDC tenant.

## Functional Requirements

- **FR-001**: New singleton table `ent_sso_config` holds the OIDC
  configuration: `enabled` (bool), `issuer` (string URL),
  `client_id`, `client_secret_encrypted`, `redirect_uri`,
  `allowed_email_domains` (JSON array; empty = allow all),
  `jit_provisioning` (bool, default false). Single row pinned by
  `id = "ent_sso_config_singleton"`.
- **FR-002**: `GET /api/sso/oidc/config` returns the config (with
  `client_secret_encrypted` redacted to a "***" placeholder if set).
- **FR-003**: `PUT /api/sso/oidc/config` upserts. Validates `issuer`
  is a non-empty URL when `enabled=true`. Encrypted secret rotates
  only when the new payload sends a non-placeholder string.
- **FR-004**: `GET /api/auth/sso/oidc/start` generates a 32-byte
  random state, stores it in an in-memory pending-state map (TTL=10
  min), discovers the IdP's `authorization_endpoint` from
  `<issuer>/.well-known/openid-configuration`, builds the auth URL
  (response_type=code, scope=openid+email+profile, state, redirect
  URI), and redirects (302).
- **FR-005**: `GET /api/auth/sso/oidc/callback` consumes
  `?code=&state=`, looks up + deletes the state (rejects unknown
  state with 400), POSTs the code to the IdP's `token_endpoint`,
  parses the returned `id_token` JWT (header.payload only — does
  NOT cryptographically verify in v1, will be tightened in spec 024),
  extracts `email` + `sub`, looks up the user in `ent_users` by
  email within the default org. If found → 200 JSON `{user_id,
  email, expires_at}` (expires_at = now + 8h). If not found and
  `jit_provisioning=false` → 403. If not found and JIT on →
  create the row then return the same JSON.
- **FR-006**: When `enabled=false`, both `/start` and `/callback`
  return 503 with `{error: "sso disabled"}`.
- **FR-007**: `audit.Emit` fires for `auth.login` (success +
  outcome=allowed) and `auth.login_denied` (outcome=denied) with
  the user email.

## Non-Functional Requirements

- **NFR-001**: `/start` returns the redirect within 500ms when the
  IdP discovery doc is cached in memory (TTL=5 min).
- **NFR-002**: Pending-state map is bounded (max 10 000 entries)
  with LRU eviction to prevent memory exhaustion.

## Success Criteria

- **SC-001**: Manual smoke against a stub IdP (httptest) completes
  the round-trip: start → IdP login → callback → JSON.
- **SC-002**: An ID token whose email isn't in `ent_users` is
  rejected with 403 when JIT is off.
- **SC-003**: Replaying a callback with a state that has already
  been consumed returns 400.

## Out of Scope (v1)

- Cryptographic verification of the ID-token signature against the
  IdP's JWKS — deferred to spec 024 (the security-tightening pass).
- Session-cookie issuance + middleware enforcement — spec 024.
- SAML 2.0.
- Multi-IdP routing (one config row only).
- Refresh tokens / silent reauthentication.

## Key Entities

- **ent_sso_config**: singleton `{id (pk), enabled, issuer,
  client_id, client_secret_encrypted, redirect_uri,
  allowed_email_domains_json, jit_provisioning, updated_at}`.
