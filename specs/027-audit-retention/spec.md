# Feature Specification: Audit log retention policy

**Feature Branch**: `027-audit-retention`
**Created**: 2026-04-22
**Status**: Draft

## Overview

`ent_audit_entries` grows monotonically. Spec 015 added an HMAC chain
to keep the table tamper-evident; spec 019 builds compliance reports
on top of it. But there is no retention policy — every audit row
ever produced lives forever, which conflicts with SOC2/GDPR
data-minimization expectations and breaks SSD budgets after months
of high-traffic enforcement.

Spec 027 adds:

- A singleton `ent_audit_retention_config` row holding the policy
  (enabled flag, retention days, last-pruned timestamp).
- Admin endpoints to inspect and update the policy.
- An on-demand prune endpoint and a background tick that prunes once
  per day when enabled.
- An explicit interaction note with spec 015: pruning rows breaks
  the HMAC chain. The default behavior is to refuse pruning when the
  oldest non-pruned row would no longer have its predecessor; the
  caller must pass `force=true` to override.

## User Scenarios

### US1 — Admin sets a 90-day retention (P1)

**As a** compliance lead
**I want** to PUT `/api/audit-logs/retention` with `{enabled:true,
retention_days:90}` and have rows older than 90 days disappear
within 24h
**So that** we satisfy our SOC2 commitment to delete identifying
audit data after 90 days.

### US2 — Admin runs a manual prune after a one-off cleanup (P2)

**As an** ops engineer
**I want** to POST `/api/audit-logs/retention/prune` to immediately
delete rows older than the configured cutoff
**So that** I do not have to wait up to 24h for the background tick.

### US3 — HMAC-chain protection (P1)

**As a** compliance lead
**I want** the prune to refuse if it would break the HMAC chain
integrity (spec 015)
**So that** I do not silently lose tamper-evidence for the records I
kept.

## Functional Requirements

- **FR-001**: New singleton table `ent_audit_retention_config` with
  columns `{id (pk = ent_audit_retention_singleton), enabled (bool,
  default false), retention_days (int, default 90), last_pruned_at
  (nullable timestamp), updated_at}`.
- **FR-002**: `GET /api/audit-logs/retention` returns the current
  config (fills in defaults if the row hasn't been written yet).
- **FR-003**: `PUT /api/audit-logs/retention` upserts. Validates
  `retention_days >= 1`. Returns 400 otherwise.
- **FR-004**: `POST /api/audit-logs/retention/prune` runs the prune
  immediately. Body may include `{force: true}`. Returns
  `{deleted: <count>, refused: <reason?>}`. If `enabled=false`,
  returns 409 with `{error: "retention disabled"}`.
- **FR-005**: Background tick — when the handler is constructed,
  start a goroutine that runs every 6h. If `enabled=true` and
  `last_pruned_at` is null OR older than 24h, run the prune.
  Goroutine stops cleanly via the handler's `Close()`.
- **FR-006**: HMAC-chain protection — the prune walks the surviving
  rows in `created_at` order. If the oldest survivor was preceded by
  a now-pruned row whose `chain_hash` was the previous row's
  `prev_hash`, the chain is broken. Default refuses unless `force=true`.
- **FR-007**: `audit.Emit` fires `audit.retention.updated` on
  config changes and `audit.retention.pruned` (with deleted count)
  on every prune run.

## Non-Functional Requirements

- **NFR-001**: Prune query must use a single bounded `DELETE` with
  `created_at < ?` — no per-row loops.
- **NFR-002**: Background goroutine must not block startup; it
  starts asynchronously after construction.

## Success Criteria

- **SC-001**: Setting retention_days=30 + manual prune deletes all
  rows whose `created_at` is older than 30 days from "now".
- **SC-002**: Prune returns 0 deleted when no rows are old enough.
- **SC-003**: When the HMAC chain is populated and pruning would
  drop a chain ancestor, the call fails with `refused`. Adding
  `force=true` succeeds.
- **SC-004**: Background tick runs at most once per 24h even when
  the goroutine wakes every 6h.

## Out of Scope (v1)

- Per-actor or per-resource-type retention policies.
- Vacuuming the table on Postgres / SQLite — the deleted rows leave
  tombstones; reclaiming disk is a separate ops task.
- Retention for the spec 003 logstore raw request/response logs —
  that table has its own retention story (out of this spec).
- Cron-style scheduling beyond "once per day".

## Key Entities

- **ent_audit_retention_config**: singleton `{id, enabled,
  retention_days, last_pruned_at, updated_at}`.
