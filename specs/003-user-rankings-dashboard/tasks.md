---
description: "Task list for User Rankings Dashboard — wire the pre-declared types into a working feature"
---

# Tasks: User Rankings Dashboard

**Input**: Design documents from `specs/003-user-rankings-dashboard/`

**Tests**: INCLUDED — Go unit test for the storage method; Playwright smoke for the UI.

## Format: `[ID] [P?] [Story] Description`

---

## Phase 1: Setup

*None — all type surface is pre-existing.*

## Phase 2: Foundational

*None.*

## Phase 3: User Story 1 — Exec views top users (Priority: P1)

### Backend

- [ ] T001 [US1] Add `GetUserRankings(ctx, filters) (*UserRankingResult, error)` to the `LogStore` interface in `framework/logstore/logstore.go` (alongside `GetModelRankings`).
- [ ] T002 [US1] Implement `RDBLogStore.GetUserRankings` in `framework/logstore/rdb.go` — mirrors `GetModelRankings` structure: current-period GROUP BY `user_id`, then previous-period query over the same (user_id) set for trend computation, excludes empty user_id rows.
- [ ] T003 [US1] Add `HybridLogStore.GetUserRankings` pass-through in `framework/logstore/hybrid.go` (one-line delegation to `h.inner`).
- [ ] T004 [US1] Add `LoggerPlugin.GetUserRankings` wrapper in `plugins/logging/operations.go` (mirror of the `GetModelRankings` wrapper at operations.go:979).
- [ ] T005 [US1] Add `LoggingHandler.getUserRankings` + route registration in `transports/bifrost-http/handlers/logging.go`: route `r.GET("/api/logs/user-rankings", ...)` alongside the existing `/api/logs/rankings` registration at logging.go:88; handler body mirrors `getModelRankings`.
- [ ] T006 [US1] Go unit test `framework/logstore/rdb_test.go` (or sibling `rdb_user_rankings_test.go`) — seed 3 users across a known time range; assert total_requests / total_tokens / total_cost per user matches a hand-calculated expectation. Assumes in-memory SQLite like existing logstore tests.

### UI

- [ ] T007 [US1] Add `useGetUserRankingsQuery` to `ui/lib/store/apis/logsApi.ts` — same pattern as `useGetModelRankingsQuery` at logsApi.ts:266; takes `{ filters: LogFilters }`; maps to `GET /logs/user-rankings`.
- [ ] T008 [US1] Rewrite `ui/app/enterprise/components/user-rankings/userRankingsTab.tsx` — flip from `FeatureStatusPanel` to a real table with columns: user_id, total_requests, total_tokens, total_cost, trend arrows. Clicking a row links to `/workspace/logs?user_ids=<id>`. Read the time range from the parent dashboard URL state (nuqs) so the tab honors the dashboard's existing time picker.
- [ ] T009 [US1] Playwright smoke at `ui/tests/e2e/enterprise/user-rankings.spec.ts` — navigate to `/workspace/dashboard` → User Rankings tab; assert the `user-rankings-view` testid, assert no `feature-status-panel` present; assert at least one row renders when seeded data exists.

## Phase 4: User Story 2 — Drilldown (Priority: P2)

- [ ] T010 [US2] In the same UI file as T008, make each row clickable; navigate to `/workspace/logs?user_ids=<id>` (preserving current from/to query params). Covered by the same component edit; listed as a separate task for traceability to US2.

## Phase 5: Polish

- [ ] T011 [P] Update CI check allowlist in `scripts/check-sc020-enterprise-stubs.sh`: no change required — the file under `user-rankings/` will no longer match the body-grep once it renders real content.
- [ ] T012 [P] Changelog entry in `ui/changelog.md` (create if needed) + `framework/changelog.md` + `transports/changelog.md`.
- [ ] T013 [P] Update spec 002's `research.md` row 20 decision from `descope → FeatureStatusPanel` to `shipped in spec 003`.
- [ ] T014 Final verify: docker build green, page loads, SC-020 script still reports 0 violations.

---

## Dependencies

- Backend chain: T001 → T002 → T003 → T004 → T005. T006 (test) can go after T002.
- UI chain: T007 (hook) → T008 (component) → T009 (Playwright). T010 rolled into T008.
- Polish tasks [P] after T009 + T005 done.

## Implementation Strategy

- MVP = T001-T008 (backend + UI working). T009-T014 are verification/polish.
- Single commit acceptable for the backend chain (they're tightly coupled).
- UI + tests in a separate commit for reviewability.
