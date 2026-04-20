# Feature Specification: Large Payload Settings

**Feature Branch**: `006-large-payload-settings`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Bifrost's transport layer has **five** context-keyed config overrides
for handling large requests/responses
(`StreamingDecompressThreshold`, `BifrostContextKeyLargeResponseThreshold`,
`BifrostContextKeyLargePayloadPrefetchSize`, `MaxPayloadBytes`,
`TruncatedLogBytes`) that are already consumed by provider clients and
log pipelines but never **set** from admin config — they currently fall
back to defaults forever. This spec ships a persistent `LargePayloadConfig`
singleton and wires it into the request path so admins can tune the
thresholds from the UI.

## User Scenarios

### US1 — SRE raises request threshold for a beta customer (P1)

**As an** SRE supporting a customer sending 50 MB chat payloads
**I want** to raise the request-decompression streaming threshold from
10 MB to 75 MB
**So that** the customer's inference requests stop spiking memory
without touching a config file + restart cycle.

**Acceptance**:
- Navigate to `/workspace/config/client-settings`
- The "Large Payload Optimization" section renders a real form (not a
  FeatureStatusPanel)
- Change `request_threshold_bytes` → 75000000 → Save
- Subsequent inference requests with 50 MB bodies stream-decompress
  without requiring a server restart

### US2 — Dev reduces response threshold to see truncated logs for debugging (P2)

**As a** developer
**I want** to lower `truncated_log_bytes` temporarily to 64 KB
**So that** I can see large responses clipped in audit logs and
diagnose a storage bloat issue.

## Functional Requirements

- **FR-001**: A new singleton table `ent_large_payload_config` persists
  `{enabled, request_threshold_bytes, response_threshold_bytes,
  prefetch_size_bytes, max_payload_bytes, truncated_log_bytes}`. Only
  one row exists (ID = `default`).
- **FR-002**: API endpoints:
  - `GET /api/config/large-payload` → current config (returns defaults
    if no row yet).
  - `PUT /api/config/large-payload` → upsert.
- **FR-003**: On save, the in-process `lib.Config.StreamingDecompressThreshold`
  is updated to the new `request_threshold_bytes` value so the next
  request picks it up without a restart.
- **FR-004**: A request middleware reads the stored config on every
  inference request (cached with a 30s TTL) and sets the
  `BifrostContextKeyLargeResponseThreshold` +
  `BifrostContextKeyLargePayloadPrefetchSize` context keys so the
  provider clients respect the configured values.
- **FR-005**: The existing enterprise stub fragment at
  `ui/app/enterprise/components/large-payload/largePayloadSettingsFragment.tsx`
  is rewritten to render real number inputs for each of the six fields,
  honouring `controlsDisabled` and `onConfigChange` from the parent
  clientSettingsView.
- **FR-006**: RBAC: writes gated by `Settings.Update`. Reads
  gated by `Settings.View`. No new RBAC resource needed.

## Non-Functional Requirements

- **NFR-001**: The middleware cache refreshes within 30 seconds of a
  save, so SAVE → next-request-honors-new-value happens within 30s.
- **NFR-002**: Zero-config fallback: if the table is empty or the
  middleware cache errors, the system falls back to existing defaults
  (no regression for non-enterprise deploys).

## Success Criteria

- **SC-001**: `/workspace/config/client-settings` renders the six
  threshold inputs; save round-trips through a real API.
- **SC-002**: A deliberately-set high threshold (1 GB) is reflected in
  `shouldStreamDecompress` within 30 seconds.
- **SC-003**: SC-020 scanner no longer matches
  `largePayloadSettingsFragment.tsx`.

## Out of Scope (v1)

- Per-VK or per-workspace overrides (single global config).
- Real-time broadcast of config changes (30s TTL is acceptable).
- UI tooltips with unit conversion helpers (the form accepts raw bytes).

## Assumptions

- Defaults from
  `ui/app/_fallbacks/enterprise/lib/types/largePayload.ts`'s
  `DefaultLargePayloadConfig` are the canonical defaults.
