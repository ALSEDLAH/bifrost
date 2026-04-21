# Feature Specification: SCIM 2.0 Users (read-only)

**Feature Branch**: `020-scim-users-read`
**Created**: 2026-04-21
**Status**: Draft

## Overview

Spec 009 shipped the SCIM token admin surface (rotate / enable /
endpoint URL). The endpoints the token gates were deferred. This
spec closes phase 2a — the **read-only** half of `/scim/v2/Users`,
which is enough for Okta / Azure AD "import users from Bifrost"
flows to succeed. Write-side provisioning (POST / PATCH / DELETE)
is deferred to spec 021.

## User Scenarios

### US1 — Okta lists Bifrost users (P1)

**As an** IT admin who connected Okta to Bifrost's SCIM endpoint
**I want** Okta's "Import Users" button to succeed and return
every user in `ent_users`
**So that** I can see which Bifrost accounts already exist before
granting SSO assignments.

## Functional Requirements

- **FR-001**: `GET /scim/v2/Users` with `Authorization: Bearer
  <token>` returns a SCIM 2.0 ListResponse body:
  ```
  {"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"],
   "totalResults": N, "startIndex": 1, "itemsPerPage": M,
   "Resources": [ <User> ... ]}
  ```
- **FR-002**: Each `<User>` has `schemas=["urn:ietf:params:scim:schemas:core:2.0:User"]`,
  `id`, `userName` (= email), `name.{givenName, familyName}` (best-effort
  split of DisplayName), `emails=[{value, primary:true}]`, `active`
  (true when status="active").
- **FR-003**: `GET /scim/v2/Users/:id` returns the single User or
  `{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],
  "status":"404"}`.
- **FR-004**: Authentication — the request's Bearer token is
  sha256-hashed and compared against `ent_scim_config.bearer_token_hash`.
  Mismatch or config.enabled=false returns 401 with the SCIM
  error body.
- **FR-005**: Pagination — `?startIndex=N&count=M`, 1-indexed per
  SCIM spec. Clamped: startIndex≥1, count∈[1, 100], default count=50.
- **FR-006**: Filtering — `?filter=userName eq "alice@example.com"`.
  v1 supports ONLY the `userName eq "literal"` form; every other
  filter expression returns 400 with a SCIM error telling the
  client to upgrade our spec.

## Non-Functional Requirements

- **NFR-001**: List endpoint returns in <300ms for 10k users on
  SQLite (the pagination is DB-side, not memory-side).
- **NFR-002**: Bearer-token validation uses constant-time compare
  so a timing attack can't leak the hash.

## Success Criteria

- **SC-001**: Okta's "Test Connection" button against `/scim/v2/Users`
  returns 200 and a well-formed ListResponse.
- **SC-002**: A hand-rolled `filter=userName eq "x"` returns the
  filtered user or an empty Resources array.
- **SC-003**: Invalid token → 401 with SCIM error envelope (not
  an HTML page).

## Out of Scope (v1)

- POST / PATCH / DELETE on `/scim/v2/Users` (spec 021).
- `/scim/v2/Groups` (spec 022).
- Complex filter expressions (`and`, `or`, `co`, `sw`, `pr`, `gt`,
  multi-attribute filters).
- `meta` attributes (created, lastModified, version, location).
- SCIM ServiceProviderConfig / ResourceTypes / Schemas discovery
  endpoints — Okta's probe handles missing endpoints gracefully.
