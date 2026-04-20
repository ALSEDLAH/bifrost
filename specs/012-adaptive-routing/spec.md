# Feature Specification: Adaptive Routing (Phase 1 — observe)

**Feature Branch**: `012-adaptive-routing`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Public docs promise "adaptive routing with circuit breakers and
multi-factor scoring". Our existing `/workspace/adaptive-routing`
view only surfaces static weighted routing rules. This spec ships
the **observation half** of adaptive routing — a live per-provider
health tracker backed by `sony/gobreaker` circuit breakers and an
EWMA latency signal, derived from the already-recorded log stream
(no hot-path surgery). The UI renders each provider's circuit
state + latency + success rate alongside the existing static
routing rules.

Phase 2 (separate spec) will wire the health signal into the
runtime target selector so circuit-open / high-error providers are
automatically downweighted.

## User Scenarios

### US1 — SRE sees a degrading provider in real time (P1)

**As an** SRE
**I want** to open `/workspace/adaptive-routing` and see each
provider's recent error rate, p95 latency, and circuit-breaker
state
**So that** I can diagnose a flapping upstream before it affects
more traffic.

### US2 — Admin reviews routing decisions (P2)

**As an** admin
**I want** the same page to show the static routing rules with
multiple targets alongside the adaptive health signals
**So that** I can see which targets are candidates for adaptive
downweighting once phase 2 ships.

## Functional Requirements

- **FR-001**: Every 10 seconds, a background goroutine reads the
  last 5 minutes of logs from `logstore.GetModelRankings` and
  updates an in-process per-(provider, model) state map.
- **FR-002**: State per key contains: a `sony/gobreaker` circuit
  breaker fed by success/error counts, an EWMA of `avg_latency`,
  success rate, total requests in the window.
- **FR-003**: Circuit breaker thresholds: trip on `>50% errors in
  ≥20 requests/minute`; half-open after 30s; reset on 3 consecutive
  successes. Configurable constants in the package.
- **FR-004**: `GET /api/adaptive-routing/status` returns JSON
  `{providers: [{provider, model, circuit_state, ewma_latency_ms,
  success_rate, total_requests, window_started_at}]}`. Cached
  response is <10ms (no DB hit on request path).
- **FR-005**: UI `/workspace/adaptive-routing` renders a Health
  Matrix above the existing routing rules table — one row per
  (provider, model) with its circuit-state badge (closed / open /
  half-open), EWMA latency, and success rate. Polls every 10s.
- **FR-006**: If `logstore` is absent (OSS dev mode), the handler
  returns `{providers: []}` without error.

## Non-Functional Requirements

- **NFR-001**: Refresh goroutine must not block on logstore; any
  error is logged at Warn and the state map is left unchanged.
- **NFR-002**: Memory overhead ≤ 1 KB per (provider, model).

## Success Criteria

- **SC-001**: Within 15 seconds of a provider starting to error,
  the Health Matrix badge flips to "open".
- **SC-002**: `/workspace/adaptive-routing` no longer displays
  only static routing rules — it surfaces real runtime health.
- **SC-003**: SC-020 scanner clean (no change needed — the view
  already is real, we're enriching it).

## Out of Scope (v1)

- Feeding the circuit state into target selection (phase 2).
- Per-API-key health (keyed on provider+model, not key).
- Predictive scaling, gossip sync, <10µs hot path (marketing
  terms; the observe-only phase is the honest floor).
- Persistent state across restarts (in-memory only).
