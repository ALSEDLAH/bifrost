# Feature Specification: Log Export Runtime (Datadog)

**Feature Branch**: `017-log-export-runtime`
**Created**: 2026-04-21
**Status**: Draft

## Overview

Spec 008 shipped the config surface for log-export connectors
(Datadog + BigQuery rows in `ent_log_export_connectors`). This spec
ships the **Datadog forwarder** — a new `ObservabilityPlugin` that
reads enabled Datadog connectors and POSTs every completed trace's
log record to Datadog's Logs API.

BigQuery runtime is deliberately deferred to a follow-up spec — it
needs `cloud.google.com/go/bigquery` + streaming-insert auth
plumbing, which is multi-spec scope. Phase 2 shaping: the same
plugin will grow a `bigquery` adapter alongside `datadog`.

## User Scenarios

### US1 — SRE forwards every inference to Datadog (P1)

**As an** SRE
**I want** every completed request to be mirrored to Datadog Logs
within ~5 seconds of the response landing
**So that** I can correlate inference traffic with provider APM
dashboards without standing up a separate OTEL pipeline.

### US2 — Ops disables forwarding without deleting the config (P2)

**As an** ops engineer
**I want** to toggle a Datadog connector off from the admin UI
(already works via spec 008)
**So that** forwarding pauses without losing the API key.

## Functional Requirements

- **FR-001**: New plugin `plugins/logexport/` implementing
  `ObservabilityPlugin.Inject(ctx, trace)` + `BasePlugin`.
- **FR-002**: On Inject, the plugin iterates every enabled connector
  of type `datadog` from `ent_log_export_connectors` and POSTs a
  single log-intake record:
  ```json
  POST https://http-intake.logs.{site}/api/v2/logs
  [{"ddsource":"bifrost","service":"bifrost","message":"…",
    "hostname":"…","ddtags":"env:…","request_id":"…",
    "provider":"…","model":"…","status":"…",
    "tokens_prompt":…,"tokens_completion":…,"cost_usd":…,
    "latency_ms":…}]
  ```
- **FR-003**: Connector list is cached with 30s TTL; admin CRUD
  handlers from spec 008 call `plugin.Invalidate()` on writes so
  edits take effect within 1s.
- **FR-004**: Network errors log Warn and drop the event
  (fire-and-forget). HTTP 4xx surfaces the body in the Warn line so
  auth issues are diagnosable.
- **FR-005**: Payload extraction is best-effort — missing fields
  are omitted rather than defaulted to zero/empty.
- **FR-006**: `bigquery` connector type is explicitly skipped with
  a one-time Info log per (process-lifetime × connector-ID) noting
  "BigQuery forwarding is scheduled for spec 018".

## Non-Functional Requirements

- **NFR-001**: Inject must return within 2s worst-case (single HTTP
  call with 2s timeout).
- **NFR-002**: Zero-config fallback: no rows in
  `ent_log_export_connectors` → Inject is a no-op in <10µs.

## Success Criteria

- **SC-001**: With a valid Datadog connector, every chat-completion
  request produces a Datadog log entry within 5 seconds tagged with
  `ddsource:bifrost`.
- **SC-002**: Disabling the connector stops forwarding on the next
  Inject (≤1s because CRUD invalidates the cache).
- **SC-003**: With no connectors, request latency is unchanged (no
  measurable delta in p95 over 1000 synthetic requests).

## Out of Scope (v1)

- BigQuery adapter (tracked as spec 018).
- S3/GCS/Azure Blob/Snowflake/Redshift adapters (docs mention them
  — each needs its own SDK + auth story).
- Filters / transformations on the forwarded payload.
- Batching multiple events into one Datadog POST (v1 is per-event;
  v2 optimisation once volume warrants it).
- Retries on failure.
