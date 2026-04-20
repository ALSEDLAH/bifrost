# Feature Specification: Adaptive Routing Phase 2 — feed signal into selection

**Feature Branch**: `013-adaptive-routing-phase-2`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Spec 012 shipped an observe-only health tracker with circuit breakers
and EWMA latency. This spec makes the signal **act** — the governance
target selector for multi-target routing rules now skips
circuit-open providers and down-weights half-open ones, with a
fail-safe fallback to the original weighted selection if every target
is unhealthy.

## User Scenarios

### US1 — Bad provider auto-avoided (P1)

**As an** SRE
**I want** a routing rule that splits traffic 50/50 between provider
A and B to automatically route 100% to A when B's circuit opens
**So that** a flapping upstream stops affecting live traffic within
a minute without me touching the rule.

## Functional Requirements

- **FR-001**: `GovernancePlugin` exposes a new setter
  `SetAdaptiveHealth(healthFn)` where `healthFn(provider, model) float64`
  returns a health factor in `[0, 1]`. Called at server startup with
  the Tracker's lookup.
- **FR-002**: The `selectWeightedTarget` path evaluates effective
  weight = `static_weight × health_factor(target.provider, target.model)`.
- **FR-003**: Health factor mapping:
  - `closed` → 1.0
  - `half-open` → 0.1
  - `open` → 0.0
  - unknown (no data yet) → 1.0 (optimistic)
- **FR-004**: Fail-safe: if every candidate's effective weight is 0
  (all circuits open), fall back to the original `selectWeightedTarget`
  behaviour so the request is not dropped — the caller will still get
  the error from the provider if it is truly down, but the gateway
  will not withhold its own routing.
- **FR-005**: Selection is visible in the routing-engine log:
  `"target <p>/<m> adaptive_weight=<w> (static=<s>, health=<h>)"`.

## Out of Scope (v1)

- Per-API-key health (key granularity still (provider, model)).
- Persisted history across restarts.
- Gossip sync of weights across cluster peers.

## Success Criteria

- **SC-001**: When a provider's circuit trips, a subsequent multi-target
  rule evaluation picks the non-open target in ≥99% of trials (empirical
  measurement under synthetic load).
- **SC-002**: When ALL candidates are circuit-open, the selector still
  returns a target rather than 500-ing the request.
