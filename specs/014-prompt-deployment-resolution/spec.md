# Feature Specification: Prompt Deployment Runtime Resolution

**Feature Branch**: `014-prompt-deployment-resolution`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Spec 011 shipped label storage (`production`, `staging`) on prompt
versions. This spec makes those labels **resolve at request time** —
when a request references a prompt without pinning an explicit
version, the prompts plugin now picks the version carrying the
`production` label (if any) instead of `is_latest`. Pinning via the
`x-bf-prompt-version` header still overrides the label.

## User Scenarios

### US1 — Author's latest draft doesn't hit prod (P1)

**As a** prompt author working on v8
**I want** production traffic to keep using v7 (which I promoted)
until I explicitly promote v8
**So that** I can iterate on `is_latest` without affecting live
inference.

### US2 — Operator rolls back with one click (P2)

**As an** operator
**I want** promoting v5 to production (from v7) to immediately cause
every subsequent inference for that prompt to use v5
**So that** rollback is instantaneous, not a redeploy.

## Functional Requirements

- **FR-001**: When `PromptResolver.Resolve` returns versionNumber==0
  (no explicit pin), the plugin consults a deployment lookup before
  falling back to `prompt.LatestVersion`.
- **FR-002**: The deployment lookup returns the version **id** (not
  number) carrying the `production` label for the given prompt, or 0
  if none.
- **FR-003**: If the lookup returns a non-zero version id that maps
  to an active version in the in-memory cache, that version is used.
  If the id doesn't map (e.g. stale label), fall back to latest and
  log a Warn.
- **FR-004**: Explicit version pins (`x-bf-prompt-version` header or
  a custom resolver) **take precedence** over deployment labels. FR-001
  only applies when the resolver did not pick a version.
- **FR-005**: Deployment labels are cached in-process with a 30s TTL
  (mirrors the large-payload middleware pattern); the admin PUT/DELETE
  handlers invalidate the cache so rollbacks are visible in <1s.

## Non-Functional Requirements

- **NFR-001**: The hot path cost is a single `sync.Map` lookup per
  inference request (~50ns); no DB hit.

## Success Criteria

- **SC-001**: After promoting v5 as production, the next inference
  for the prompt routes through v5 — observable via
  `BifrostContextKeySelectedPromptVersion` and in logs.
- **SC-002**: With no production label set, behaviour is unchanged
  from today (falls back to `prompt.LatestVersion`).

## Out of Scope (v1)

- `staging` label resolution (only `production` is live-routed; other
  labels remain admin-metadata).
- Per-VK or per-customer deployment labels.
- Canary percentages — a production label picks one version
  deterministically.
