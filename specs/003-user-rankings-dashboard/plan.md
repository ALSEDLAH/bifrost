# Implementation Plan: User Rankings Dashboard

**Branch**: `003-user-rankings-dashboard` | **Date**: 2026-04-20 | **Spec**: [spec.md](./spec.md)

## Summary

Smallest SR-01-out-of-scope item from spec 002's audit. The frontend types (`UserRankingsResponse`) were forward-declared in spec 001; the Go types (`UserRankingEntry`, `UserRankingTrend`, `UserRankingWithTrend`, `UserRankingResult`) are already in `framework/logstore/tables.go`. Neither has a matching storage method or handler. This spec ships the missing connective tissue.

Mirror `GetModelRankings` for users: one new storage method on `RDBLogStore`, one pass-through on `HybridLogStore`, one plugin method, one handler method, one route registration, one RTK hook, one UI flip.

## Technical Context

**Language/Version**: Go 1.26.1 (backend), TypeScript 5.9 (UI).
**Primary Dependencies**: existing `framework/logstore` + `plugins/logging` + `transports/bifrost-http/handlers`. No new modules.
**Storage**: `bifrost_logs` (already exists, already indexed on `user_id`).
**Testing**: Go unit test against in-memory SQLite (mirror the model-rankings test pattern); Playwright E2E for the UI tab.
**Target Platform**: Enterprise Bifrost build.
**Project Type**: Web service + UI.
**Performance Goals**: p95 <2s for 30-day window over 10M rows (SC-002).
**Constraints**: zero `core/**` changes; sibling-method extension only (no new files in upstream storage).
**Scale/Scope**: 1 new endpoint, ~150 LOC of new Go, ~120 LOC of new UI.

## Constitution Check

| # | Principle | Status | Notes |
|---|-----------|--------|-------|
| I | Core Immutability | ✅ | No `core/**` touches. |
| II | Non-Breaking | ✅ | New endpoint; no existing signatures change. |
| III | Plugin-First | ✅ | Method added to existing `LoggerPlugin`; no new plugin. |
| IV | Config-Driven Gating | ✅ | No new gates. |
| V | Multi-Tenancy First | ✅ | Storage method filters on `organization_id` like every other query. |
| VI | Observability | ✅ | Emits on existing governance audit path when the handler is invoked (via shared logging middleware). |
| VII | Security by Default | ✅ | No secrets handled; purely aggregating already-persisted rows. |
| VIII | Test Coverage | ✅ | Go unit test + Playwright E2E. |
| IX | Docs & Schema Sync | ✅ | No schema change. Docs update for dashboard page. |
| X | Dependency Hierarchy | ✅ | No reverse imports. |
| XI | Upstream-Mergeability | ✅ | All new code lives in sibling methods/files next to the model-rankings equivalents. Pattern already precedented. |

All pass. No complexity-tracking entries.

## Project Structure

```text
specs/003-user-rankings-dashboard/
├── plan.md
├── spec.md
├── research.md
├── data-model.md
├── contracts/
│   └── user-rankings-api.md
└── checklists/
    └── requirements.md
```

### Source code touched

```text
# Backend (Go)
framework/logstore/rdb.go                 # ADD GetUserRankings(ctx, filters) — mirrors GetModelRankings
framework/logstore/hybrid.go              # ADD GetUserRankings pass-through
framework/logstore/logstore.go            # ADD GetUserRankings to the interface
plugins/logging/operations.go             # ADD LoggerPlugin.GetUserRankings wrapper
transports/bifrost-http/handlers/logging.go   # ADD getUserRankings handler + route registration

# UI (TypeScript)
ui/lib/store/apis/logsApi.ts              # ADD useGetUserRankingsQuery hook
ui/app/enterprise/components/user-rankings/userRankingsTab.tsx   # FLIP from FeatureStatusPanel → real table
ui/tests/e2e/enterprise/user-rankings.spec.ts   # NEW — Playwright smoke
```

**Structure Decision**: sibling-method + sibling-handler pattern already used for model rankings. Zero net-new files in upstream-owned code paths.

## Phased Delivery

### Phase 0 — Research (see [research.md](./research.md))

Confirmed all Go types pre-exist. Confirmed `bifrost_logs.user_id` is indexed. No open questions.

### Phase 1 — Design & Contracts

- [data-model.md](./data-model.md) documents reuse of pre-existing types.
- [contracts/user-rankings-api.md](./contracts/user-rankings-api.md) defines the endpoint contract.

### Phase 2 — Tasks

See [tasks.md](./tasks.md) (generated next).

## Complexity Tracking

Empty.
