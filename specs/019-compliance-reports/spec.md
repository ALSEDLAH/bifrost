# Feature Specification: Compliance Reports

**Feature Branch**: `019-compliance-reports`
**Created**: 2026-04-21
**Status**: Draft

## Overview

Public docs mention "compliance reports with SOC 2 / GDPR / HIPAA /
ISO 27001 widgets" as part of the enterprise surface. Full
compliance-framework tooling is big — partner-signed evidence
packages, retention attestations, etc. This spec ships **phase 1**:
two aggregate endpoints that summarise existing audit + governance
data over a rolling window, plus a minimal `/workspace/reports`
page that renders them. Auditors can screenshot; compliance officers
can use the numbers as starting points for their own workpapers.

## User Scenarios

### US1 — Compliance officer checks admin activity volume (P1)

**As a** compliance officer preparing SOC 2 CC6.1 evidence
**I want** a "last 30 days admin activity" widget showing a count
of audit entries per action type
**So that** I can say "we had N role changes and M VK rotations in
the period" without SQL access.

### US2 — SecOps checks access-control cadence (P2)

**As a** SecOps reviewer
**I want** a "last 30 days access-control events" widget counting
role grants / revokes / user creates / user deletes
**So that** I can verify the cadence is reasonable (not zero, not
thousands).

## Functional Requirements

- **FR-001**: `GET /api/reports/admin-activity?days=N` returns
  `{window_days, since, buckets: [{action, outcome, count}]}`
  sourced from `ent_audit_entries` in the last N days. N is
  clamped to [1, 365].
- **FR-002**: `GET /api/reports/access-control?days=N` returns
  `{window_days, since, role_changes, role_assignments,
  user_creates, user_deletes, key_rotations}` where each value is
  an integer count sourced from audit entries whose action matches
  the feature.
- **FR-003**: Both endpoints require `AuditLogs.Read` (reuses the
  existing RBAC resource — compliance is downstream of audit).
- **FR-004**: UI: new route `/workspace/reports` renders two
  cards, default window=30 days, with a "Window" selector
  (7/30/90). Empty DB reads as zero counts, not an error.

## Non-Functional Requirements

- **NFR-001**: Each endpoint returns in <500ms on SQLite over
  ≤50k audit rows.
- **NFR-002**: No caching — reports reflect current DB state.
  If volume scales up this becomes a follow-up optimisation.

## Success Criteria

- **SC-001**: Compliance officer can open `/workspace/reports`,
  toggle window to 7 days, copy the counts into their evidence
  doc in under 60 seconds.
- **SC-002**: SC-020 scanner clean — the page doesn't pass through
  any `feature-status-panel`.

## Out of Scope (v1)

- PDF / CSV export of reports.
- Framework-specific dashboards (SOC 2 / GDPR / HIPAA mappings).
- Retention attestations, evidence signing, chain-of-custody.
- Time-series graphs — v1 is "total count over window".
