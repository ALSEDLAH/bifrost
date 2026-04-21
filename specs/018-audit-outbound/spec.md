# Feature Specification: Audit Outbound

**Feature Branch**: `018-audit-outbound`
**Created**: 2026-04-21
**Status**: Draft

## Overview

Spec 015 added an HMAC-tamper chain to every audit row. Spec 004
shipped alert channels (webhook + Slack) that already dispatch
governance budget-threshold events. This spec piggybacks audit
events on the **same** dispatcher so every admin action (role
change, VK create/delete, guardrail block) lands in the operator's
SIEM webhook / Slack channel in near-real-time.

No new table, no new UI. The existing alert channels become the
audit export pipe.

## User Scenarios

### US1 — SecOps gets a Slack ping on every role change (P1)

**As a** SecOps engineer
**I want** a message in `#audit` Slack whenever someone in the
admin UI creates, updates, or deletes a role
**So that** I have a second-of-the-minute trail for SOC-2 review.

### US2 — Compliance pipes audit to Splunk HEC (P2)

**As a** compliance team
**I want** to point an `ent_alert_channels` webhook at Splunk's HEC
endpoint with a custom `Authorization: Splunk <token>` header
**So that** our existing SIEM ingests every Bifrost audit entry
without us standing up a separate exporter.

## Functional Requirements

- **FR-001**: The audit plugin gains an exported setter
  `SetAlertDispatcher(*alertchannels.Dispatcher, func() []TableAlertChannel)`
  mirroring the governance-plugin hook from spec 004.
- **FR-002**: On every successful audit insert (both the sync Emit
  path and the async worker flush), the plugin calls
  `dispatcher.Send(channels(), Event{type="audit.entry", data=…})`.
- **FR-003**: The Event.Data shape: `{id, action, resource_type,
  resource_id, outcome, actor_type, actor_id, reason, request_id,
  created_at}`. `HMAC` + `PrevHMAC` are deliberately excluded —
  they're internal chain state.
- **FR-004**: Dispatch is fire-and-forget (reuses the alert
  channels dispatcher's goroutine fan-out with 5s timeout).
- **FR-005**: Failures log at Warn (via the dispatcher's existing
  logger) and never roll back the audit row insert.
- **FR-006**: Server startup wires up the dispatcher after audit
  plugin init — same pattern as spec 004 (governance).

## Non-Functional Requirements

- **NFR-001**: Zero measurable overhead when no alert channels are
  configured (channelsFn returns empty → dispatcher.Send no-ops in
  <1µs).
- **NFR-002**: Audit insert latency must not increase — dispatch
  happens after `Create()` returns, via goroutine.

## Success Criteria

- **SC-001**: With one enabled Slack channel, every audit row also
  arrives in Slack within 5 seconds.
- **SC-002**: Deleting all alert channels stops audit outbound on
  the next event (<1s, tied to the alert-channel fetcher's live
  DB read).

## Out of Scope (v1)

- Per-event-type filtering (`subscribe to audit.* only`) — v1 sends
  all threshold crossings AND all audit events to every enabled
  channel. Filter field on the channel table is a v2 addition.
- Retries, batching, dead-letter queue.
- Dedicated Splunk HEC adapter — the existing webhook channel type
  is flexible enough via custom headers.
- Elastic, PagerDuty, email — any HTTP-receiving SIEM works via
  webhook today.
