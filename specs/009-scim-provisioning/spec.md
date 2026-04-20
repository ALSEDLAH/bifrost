# Feature Specification: SCIM Provisioning

**Feature Branch**: `009-scim-provisioning`
**Created**: 2026-04-20
**Status**: Draft

## Overview

SCIM 2.0 is the industry standard for provisioning users/groups from
an IdP (Okta, Azure AD, JumpCloud, etc.) into downstream apps. Full
SCIM 2.0 protocol support (filter expressions, PATCH, groups,
bulk ops) is substantial work. This spec ships **phase 1**: the
admin surface and bearer-token management, so IT can integrate their
IdP today even though the underlying endpoint is still evolving.

Phase 2 (a separate spec) implements the `/scim/v2/*` HTTP
endpoints against existing `ent_users` and the governance `teams`
table.

## User Scenarios

### US1 — IT admin generates a SCIM bearer token (P1)

**As an** IT admin
**I want** to click "Regenerate token" on the SCIM page and copy the
new bearer token
**So that** I can paste it into Okta's provisioning integration and
trigger a test import.

### US2 — IT admin disables SCIM during an outage (P2)

**As an** IT admin
**I want** to toggle SCIM off
**So that** Okta provisioning pauses without deleting the token.

## Functional Requirements

- **FR-001**: A new singleton table `ent_scim_config` stores
  `{enabled, bearer_token_hash, token_prefix, token_created_at,
  created_at, updated_at}`. Only the hash of the token is stored;
  the plaintext is shown once at generation time.
- **FR-002**: API at `/api/scim/config`:
  - `GET` → returns `{enabled, endpoint_url, token_prefix,
    token_created_at}`.
  - `POST /rotate` → generates a new token, stores hash, returns the
    plaintext **once**.
  - `PATCH` → toggle `enabled`.
- **FR-003**: `/workspace/scim` view: endpoint URL display,
  enabled switch, "Regenerate token" button with a one-shot reveal
  modal for the plaintext.
- **FR-004**: SCIM 2.0 HTTP endpoints (`/scim/v2/Users`, etc.) are
  **out of scope** for v1 — tracked as phase 2. The UI makes this
  explicit with a banner.

## Success Criteria

- **SC-001**: An admin can generate a SCIM bearer token and copy it
  to the clipboard in < 30 seconds.
- **SC-002**: Regenerating a token invalidates the previous one
  (hash changes).
- **SC-003**: SC-020 scanner no longer matches `scimView.tsx`.
