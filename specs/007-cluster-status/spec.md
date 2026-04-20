# Feature Specification: Cluster Status (single-node)

**Feature Branch**: `007-cluster-status`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Bifrost v1 runs as a single-node deployment. The `/workspace/cluster`
stub currently says "cluster management needs its own spec" — which
is true for multi-node coordination, but we can still surface useful
single-node metadata today (hostname, version, uptime, pid, build
info) so operators have a real page to visit instead of a placeholder.
Multi-node coordination remains a future spec.

## User Scenarios

### US1 — SRE confirms the pod is healthy and on the expected version (P1)

**As an** SRE
**I want** a `/workspace/cluster` page that shows hostname + version +
uptime
**So that** I can confirm the enterprise build deployed to production
without shelling into the pod.

## Functional Requirements

- **FR-001**: New `GET /api/cluster/status` endpoint returning
  `{node_role, hostname, version, started_at, uptime_seconds,
  pid, goroutines, process_memory_bytes}`.
- **FR-002**: `node_role` is always `"standalone"` in v1; reserved
  for `"leader"|"follower"` in future multi-node spec.
- **FR-003**: UI: flip `/workspace/cluster` from
  FeatureStatusPanel to a real card grid. "Add node" and peer
  discovery are explicitly deferred.

## Success Criteria

- **SC-001**: The page renders hostname, uptime, and version of the
  running node within 200 ms.
- **SC-002**: SC-020 scanner no longer matches
  `clusterView.tsx`.
