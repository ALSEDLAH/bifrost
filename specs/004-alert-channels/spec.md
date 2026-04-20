# Feature Specification: Alert Channels

**Status**: draft
**Authors**: autonomous (on behalf of fayez.alsedlah@tradingcentral.com)
**Created**: 2026-04-20

## Overview

Bifrost's governance plugin already broadcasts `budget.threshold.crossed`
events over a process-internal channel (WebSocket to connected UIs) at
50/75/90% of a budget. Operators who aren't sitting on the dashboard miss
them. This feature adds **persistent alert channels** (generic webhook or
Slack) that subscribe to threshold crossings (and, over time, other
events) so teams get paged where they already work.

## User Scenarios

### US1 — SRE sets up a Slack channel for budget alerts (P1)

**As an** SRE with dashboard/governance access
**I want** to register a Slack incoming-webhook URL as an alert channel
**So that** when a virtual key crosses 75% of its budget, my team's
`#ai-platform-alerts` Slack channel gets a message within seconds.

**Acceptance**:
- Navigate to `/workspace/alert-channels`
- Click "New channel" → select `slack` → paste webhook URL →
  name → save
- Channel appears in the list with a green "Enabled" badge
- Within 5 seconds of a 75% crossing, Slack shows a message with
  `{budget_id, virtual_key, team, current_usage, max_limit}`
- Failure modes (5xx, non-2xx, timeout) are logged but do **not**
  block inference traffic

### US2 — Platform admin disables a noisy channel without deleting it (P2)

**As a** platform admin
**I want** to toggle a channel off while keeping its config
**So that** I can temporarily silence a Slack workspace during a
maintenance window and re-enable later without re-entering the URL.

**Acceptance**:
- Toggle the row's "Enabled" switch off
- Next crossing does not dispatch to that channel
- Toggle back on → next crossing dispatches again

### US3 — Operator tests the channel before waiting for a real event (P2)

**As an** operator who just created a channel
**I want** a "Send test" action
**So that** I can confirm the webhook actually reaches Slack without
waiting for a real budget crossing.

**Acceptance**:
- Click "Send test" on the row
- A fake payload (`event_type: "alert.test"`) is dispatched
- UI toasts "Test sent — check Slack" on 2xx or
  "Delivery failed: <message>" on non-2xx / timeout

## Functional Requirements

- **FR-001**: Alert channels persist in a new `ent_alert_channels`
  table with: `id (uuid)`, `name (varchar)`, `type (varchar: webhook|slack)`,
  `config (jsonb)`, `enabled (bool)`, `created_at`, `updated_at`.
  `config` for `webhook`: `{url, headers?, method?}`. For `slack`:
  `{webhook_url}`.
- **FR-002**: CRUD API at `/api/alert-channels`:
  - `GET /api/alert-channels` → list
  - `POST /api/alert-channels` → create
  - `PATCH /api/alert-channels/:id` → update (fields: name, config, enabled)
  - `DELETE /api/alert-channels/:id` → delete
  - `POST /api/alert-channels/:id/test` → dispatch a synthetic event
- **FR-003**: Governance plugin's `emitThreshold` must dispatch the same
  payload to every enabled channel after the existing log + broadcast
  calls. Dispatch is fire-and-forget in a goroutine; errors logged only.
- **FR-004**: Slack dispatcher formats the payload as a text message
  (no rich blocks in v1). Webhook dispatcher POSTs the raw JSON event.
- **FR-005**: RBAC: list/create/update/delete gated by
  `AlertChannels` resource × standard operations. Read-only roles see
  the page; only admin-like roles can mutate.
- **FR-006**: `/workspace/alert-channels` page renders a table of
  channels with columns `name`, `type`, `enabled`, `last_dispatch_at`
  (optional v2), and per-row actions `Edit`, `Test`, `Delete`. Empty
  state shows "New channel" CTA.

## Non-Functional Requirements

- **NFR-001**: Dispatch latency budget: p95 < 2s per channel from
  crossing to HTTP request issued (not response). Failure must not
  block governance evaluation of the same or future requests.
- **NFR-002**: Webhook/Slack HTTP timeout: 5s. After timeout, log Warn
  and drop the event (no retry in v1).
- **NFR-003**: Plugin must be safe against channel misconfiguration
  (e.g., 404 / malformed URL) — no panics, no goroutine leaks.

## Success Criteria

- **SC-001**: An operator can go from empty `/workspace/alert-channels`
  to a working Slack delivery in < 90 seconds.
- **SC-002**: A deliberately broken channel (e.g., 404 URL) does not
  affect p99 inference latency measured over 1 minute of synthetic
  traffic.
- **SC-003**: `/workspace/alert-channels` no longer renders
  `feature-status-panel` (spec 002 CI check still passes).

## Out of Scope (v1)

- Email, PagerDuty, SMS channels
- Templated message bodies / rich Slack blocks
- Delivery retries + dead-letter queue
- Per-channel event-type filtering (all channels get all events in v1)
- `last_dispatch_at` / per-channel metrics (v2)

## Assumptions

- Users providing webhook URLs trust their own endpoints; no
  out-of-process sandboxing or egress allow-lists in v1.
- Existing `emitThreshold` is the only event source in v1; new event
  types added later will opt into the same dispatch path.
