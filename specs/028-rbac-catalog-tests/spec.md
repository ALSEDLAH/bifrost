# Feature Specification: RBAC catalog/test reconciliation

**Feature Branch**: `028-rbac-catalog-tests`
**Created**: 2026-04-22
**Status**: Draft

## Overview

`tenancy.Resources` started life with 24 entries, matching the
frontend's RBAC enum. Two more entries (`AdaptiveRouter`,
`PIIRedactor`) were added subsequently, so the catalog now has 26.
But:

- The doc comment in `framework/tenancy/scopes.go` still says 24.
- Two RBAC handler tests (`TestRBAC_Meta_Returns24ResourcesAnd6Operations`
  and `TestRBAC_Me_ReturnsWildcardInSingleOrgMode`) hardcode the
  number 24, so `go test ./...` reports failures even though no
  behavior is broken.

Spec 028 reconciles this drift and makes the tests catalog-agnostic
so future additions don't trigger the same breakage.

## User Scenarios

### US1 — Engineer adds a new RBAC resource without breaking tests (P1)

**As a** backend engineer
**I want** to append a new resource to `tenancy.Resources` and have
the existing test suite still pass without any test edits  
**So that** evolving the RBAC catalog doesn't trigger noise commits.

## Functional Requirements

- **FR-001**: Doc comment in `framework/tenancy/scopes.go` MUST
  reflect the current resource count via the catalog itself rather
  than a hardcoded number — phrase as "matches the frontend
  RbacResource enum".
- **FR-002**: `TestRBAC_Meta_Returns24ResourcesAnd6Operations` MUST
  be renamed to drop the literal number and assert against
  `len(tenancy.Resources)` / `len(tenancy.Operations)` so it
  survives catalog growth.
- **FR-003**: `TestRBAC_Me_ReturnsWildcardInSingleOrgMode` MUST
  assert `len(got.Permissions) == len(tenancy.Resources)`.
- **FR-004**: The 6-operation count is more stable than the
  resource count (it tracks Read/View/Create/Update/Delete/Download
  exactly), so the test MAY keep that as a literal — but it MUST
  also assert that every operation in the response appears in
  `tenancy.Operations`.

## Non-Functional Requirements

- **NFR-001**: No behavior change to the RBAC handler or to
  `tenancy.Resources` — this spec is test + doc only.

## Success Criteria

- **SC-001**: `go test -run TestRBAC ./transports/bifrost-http/handlers/`
  passes after the changes.
- **SC-002**: Adding a 27th resource to `tenancy.Resources` does
  NOT cause any RBAC test to fail.

## Out of Scope (v1)

- Splitting the RBAC catalog out of `framework/tenancy/scopes.go`
  into its own package.
- Pruning the catalog (e.g., removing `Cluster` if cluster mode is
  not in v1).
- Generating the frontend enum from the Go catalog (or vice versa).
