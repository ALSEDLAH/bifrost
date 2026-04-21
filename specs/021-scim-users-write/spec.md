# Feature Specification: SCIM 2.0 Users write-side

**Feature Branch**: `021-scim-users-write`
**Created**: 2026-04-21
**Status**: Draft

## Overview

Spec 020 shipped read-only `GET /scim/v2/Users`. This spec adds the
three write verbs that complete Okta / Azure AD "Push new users"
flows: `POST /scim/v2/Users` (create), `PATCH /scim/v2/Users/:id`
(partial update), `DELETE /scim/v2/Users/:id` (soft-deactivate).
SCIM Groups remain out of scope (spec 022).

## User Scenarios

### US1 — Okta provisions a new employee into Bifrost (P1)

**As an** IT admin
**I want** Okta to POST a new User resource to Bifrost's SCIM
endpoint and receive back 201 with the persisted User
**So that** the employee has a row in `ent_users` before they
attempt SSO login.

### US2 — Okta de-activates a terminated user (P1)

**As an** IT admin
**I want** Okta to PATCH `active=false` on a user and have Bifrost
flip their status to `suspended`
**So that** the terminated employee can no longer complete SSO.

### US3 — Okta hard-deletes a user (P2)

**As an** IT admin
**I want** Okta's DELETE /scim/v2/Users/:id to soft-deactivate
(status=suspended), not hard-delete
**So that** audit history is preserved while access is revoked.

## Functional Requirements

- **FR-001**: `POST /scim/v2/Users` accepts a SCIM User resource
  and creates an `ent_users` row. Required fields: `userName`
  (stored as `email`). Optional: `name.{givenName,familyName}`
  (joined into `display_name`), `externalId` (stored as
  `idp_subject`), `active` (defaults true → status=active).
  Returns 201 + the SCIM User shape (schema from spec 020).
- **FR-002**: On duplicate `userName` within the same
  `organization_id`, returns 409 with SCIM error schema.
- **FR-003**: `PATCH /scim/v2/Users/:id` accepts the SCIM PATCH
  operations envelope. v1 supports ONLY the common Okta patterns:
  * `{"op":"Replace","path":"active","value":false}` → status=suspended
  * `{"op":"Replace","path":"active","value":true}` → status=active
  * `{"op":"Replace","path":"name.givenName","value":"…"}` → merges into display_name
  * `{"op":"Replace","path":"name.familyName","value":"…"}` → merges into display_name
  * `{"op":"Replace","path":"userName","value":"…"}` → updates email
  Every other op path returns 400.
- **FR-004**: `DELETE /scim/v2/Users/:id` soft-deactivates:
  status=suspended + returns 204. Idempotent (missing id → 204).
- **FR-005**: All three endpoints reuse the spec-020 bearer-token
  auth helper. Audit.Emit fires for every create/update/delete
  with action `user.create` / `user.update` / `user.delete`.
- **FR-006**: Default organization_id for created users = synthetic
  default org from spec 001 (single-org v1). Multi-org sourcing
  via an explicit `urn:…:scim:…:User` extension is a phase-2
  follow-up.

## Non-Functional Requirements

- **NFR-001**: POST/PATCH/DELETE each return in <300ms on SQLite.
- **NFR-002**: Request-body size limit 64 KB (Okta payloads are
  always well under this).

## Success Criteria

- **SC-001**: Okta's provisioning "push users" workflow completes
  end-to-end: POST creates, PATCH deactivates, DELETE hard-signals.
- **SC-002**: Every successful SCIM mutation produces an audit
  entry with the matching `action` verb.
- **SC-003**: Unsupported PATCH paths return 400 with a clear
  SCIM error body.

## Out of Scope (v1)

- Bulk operations.
- `/scim/v2/Groups` (spec 022).
- PUT (full replace) — Okta uses PATCH, not PUT.
- Hard-delete (all deletes soft-deactivate; evidence retention).
- Multi-org creates via extension schema.
