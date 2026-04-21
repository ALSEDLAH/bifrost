# Feature Specification: SCIM 2.0 Groups (read + write)

**Feature Branch**: `022-scim-groups`
**Created**: 2026-04-21
**Status**: Draft

## Overview

Specs 020 + 021 shipped the full Users half of SCIM. Spec 022
adds the Groups half — enough for Azure AD / JumpCloud / Okta to
push IdP groups into Bifrost and keep their membership in sync.
Groups here are SCIM-only identity groupings (NOT the upstream
`governance_teams`, which are billing entities). Groups do NOT
yet grant permissions in v1 — that gate is a follow-up spec.

## User Scenarios

### US1 — Azure AD pushes an "Engineering" group (P1)

**As an** IT admin  
**I want** Azure AD to POST a Group resource with members to
Bifrost's SCIM endpoint and receive back 201  
**So that** the group row + member links exist before I can reason
about group-based access in a later spec.

### US2 — Okta PATCHes members into an existing group (P1)

**As an** IT admin  
**I want** Okta's incremental `Add members` and `Remove members`
PATCH ops to stay consistent with our Group's members list  
**So that** group membership tracks the IdP without periodic full
re-pushes.

### US3 — Admin deletes a group (P2)

**As an** IT admin  
**I want** DELETE /scim/v2/Groups/:id to remove the group and its
member links but leave the Users untouched  
**So that** a retired group stops being listable while the people
continue to exist.

## Functional Requirements

- **FR-001**: `GET /scim/v2/Groups` lists groups in the default org
  with startIndex/count pagination. Response uses the SCIM
  ListResponse envelope. Each resource MUST include `id`,
  `displayName`, `members[].{value,display}`.
- **FR-002**: `GET /scim/v2/Groups/:id` returns one group or 404
  (SCIM error envelope).
- **FR-003**: `POST /scim/v2/Groups` creates `ent_scim_groups` row
  and `ent_scim_group_members` link rows for every member. Required
  field: `displayName`. Optional: `members[]`, `externalId`.
  Returns 201 + the SCIM Group shape.
- **FR-004**: `PATCH /scim/v2/Groups/:id` supports these ops:
  * `{"op":"Replace","path":"displayName","value":"…"}`
  * `{"op":"Add","path":"members","value":[{"value":"userId"}]}`
  * `{"op":"Remove","path":"members[value eq \"userId\"]"}`
  * `{"op":"Replace","path":"members","value":[{"value":"uid"}]}` (full replace)
  Every other path returns 400 with SCIM error envelope.
- **FR-005**: `DELETE /scim/v2/Groups/:id` hard-deletes the group
  and cascades to `ent_scim_group_members`. Users are NOT touched.
  Idempotent (missing id → 204).
- **FR-006**: All endpoints reuse the spec-020 bearer-auth helper.
  `audit.Emit` fires for every create/update/delete with actions
  `group.create` / `group.update` / `group.delete`.
- **FR-007**: Member values that don't resolve to an `ent_users` id
  are skipped silently (warning logged) — this matches Okta's
  behavior when user and group pushes race.
- **FR-008**: `organization_id` for created groups = synthetic default
  org from spec 001 (single-org v1).

## Non-Functional Requirements

- **NFR-001**: GET list with 100 members returns in <300ms on SQLite.
- **NFR-002**: Request body size limit 64 KiB.

## Success Criteria

- **SC-001**: Azure AD's "push groups" workflow completes end-to-end:
  POST creates, PATCH adds/removes members, DELETE tears down.
- **SC-002**: Every successful mutation yields an audit entry with
  the matching `action` verb.
- **SC-003**: Unsupported PATCH paths return 400 with a clear SCIM
  error body.

## Out of Scope (v1)

- Using Groups to grant permissions / RBAC roles (future spec).
- Bulk operations (`/scim/v2/Bulk`).
- Member type distinction (User vs Group nesting).
- Paging inside `members[]` for >1000-member groups.
- Multi-org group sourcing via extension schema.

## Key Entities

- **ent_scim_groups**: `{id (pk), organization_id (fk),
  display_name, external_id (nullable unique per org), created_at,
  updated_at}`.
- **ent_scim_group_members**: `{group_id (fk cascade),
  user_id (fk), created_at, primary key (group_id, user_id)}`.
