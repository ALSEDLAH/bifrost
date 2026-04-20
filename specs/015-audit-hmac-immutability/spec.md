# Feature Specification: Audit Log HMAC Chain

**Feature Branch**: `015-audit-hmac-immutability`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Public docs promise "tamper-proof audit logs with cryptographic
verification." Today every `audit.Emit` call inserts a plain GORM
row — a DBA (or anyone with table-level write) could silently rewrite
or delete entries. This spec adds an HMAC-SHA256 chain: every row
carries `HMAC = HMAC(key, prev_hmac || canonical_bytes)`. A
`/api/audit-logs/verify` endpoint walks the chain and reports any
break — evidence of tampering.

## User Scenarios

### US1 — Compliance officer proves log integrity (P1)

**As a** compliance officer preparing a SOC 2 interview
**I want** to hit `/api/audit-logs/verify` and see
`{valid: true, entries: 12345}`
**So that** I can attach the evidence to the auditor's package.

### US2 — SecOps detects a tampered row (P2)

**As a** SecOps engineer investigating a suspected insider
**I want** verification to flag the *first* row whose HMAC doesn't
match, so I can bisect when and where the chain broke.

## Functional Requirements

- **FR-001**: `TableAuditEntry` grows two columns: `HMAC` (hex) and
  `PrevHMAC` (hex, empty on the first row).
- **FR-002**: The audit sink loads `BIFROST_AUDIT_HMAC_KEY` (base64
  or hex) at init. If unset, the chain is disabled — rows still
  insert (backwards compatible) but `HMAC` / `PrevHMAC` stay empty
  and verification reports "no key configured".
- **FR-003**: `canonical_bytes(row)` = pipe-joined fields in a fixed
  order: `id|org|workspace|actor_type|actor_id|actor_display|actor_ip|
  action|resource_type|resource_id|outcome|reason|before_json|after_json|
  request_id|created_at_rfc3339_nanos`. HMAC / PrevHMAC themselves
  are **not** part of the canonical bytes.
- **FR-004**: Insertion is serialized behind a `sync.Mutex` in the
  sink so concurrent Emits cannot both read the same predecessor
  HMAC (which would fork the chain).
- **FR-005**: `GET /api/audit-logs/verify` walks every row ordered by
  `created_at, id` ascending, recomputes each HMAC, and returns
  `{valid, entries_checked, first_break: {id, reason}}`. Returns
  `{valid: true}` on empty tables.
- **FR-006**: RBAC: verify endpoint gated by
  `AuditLogs.Download` (same as export) — treating chain integrity
  as a sensitive read.

## Non-Functional Requirements

- **NFR-001**: HMAC computation is <1ms per row on a commodity CPU;
  verify across 1M rows should complete in under 30 seconds on a
  single core.
- **NFR-002**: If the HMAC key rotates, the chain restarts at the
  next row with empty `PrevHMAC`; verification reports a "key
  rotation boundary" rather than "broken chain" when a run of empty
  PrevHMAC is detected after a populated one. (v2 may store a key
  generation number — not in v1.)

## Success Criteria

- **SC-001**: After manually `UPDATE`ing a `Reason` field in the DB,
  `/verify` flips from `valid: true` to `valid: false` with the
  tampered row's ID surfaced.
- **SC-002**: The existing `audit.Emit` API surface is unchanged;
  every call site keeps working.

## Out of Scope (v1)

- Key rotation automation (operator manages the env var manually).
- Signing with an HSM or KMS-managed key.
- Merkle tree / block-style batching — linear chain is enough at
  audit volumes.
- Retroactive HMAC computation for rows written before the chain
  was enabled.
