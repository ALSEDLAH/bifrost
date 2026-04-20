# Phase 0 Research — User Rankings Dashboard

**Feature**: 003-user-rankings-dashboard
**Date**: 2026-04-20

## Findings

### Pre-existing scaffolding

Someone on an earlier pass authored the type surface on both sides without wiring the middle:

| Layer | Symbol | Location | State |
|---|---|---|---|
| Go | `UserRankingEntry` | `framework/logstore/tables.go:1439-1444` | **declared, unused** |
| Go | `UserRankingTrend` | `framework/logstore/tables.go:1447-1452` | **declared, unused** |
| Go | `UserRankingWithTrend` | `framework/logstore/tables.go:1455-1458` | **declared, unused** |
| Go | `UserRankingResult` | `framework/logstore/tables.go:1461-1463` | **declared, unused** |
| TS | `UserRankingEntry`, `UserRankingTrend`, `UserRankingsResponse` | `ui/lib/types/logs.ts:1113-1130` | **declared, unused** |

All type shapes between Go and TypeScript are already aligned. The only missing pieces are:

1. Go storage method `RDBLogStore.GetUserRankings(ctx, filters) (*UserRankingResult, error)`.
2. Pass-through on `HybridLogStore`.
3. Interface extension on `LogStore` so callers can reach it.
4. Plugin wrapper `LoggerPlugin.GetUserRankings`.
5. HTTP handler `LoggingHandler.getUserRankings` + route registration.
6. RTK Query hook `useGetUserRankingsQuery`.
7. UI consumer replacing the `FeatureStatusPanel` in `userRankingsTab.tsx`.

### Index support

`framework/logstore/tables.go:127` declares:

```go
UserID *string `gorm:"type:varchar(255);index:idx_logs_user_id" json:"user_id"`
```

The index `idx_logs_user_id` exists today. A `GROUP BY organization_id, user_id WHERE created_at BETWEEN ? AND ?` query will be index-supported for the `user_id` leg; the `created_at` predicate uses the existing time-range index. Expected to meet SC-002 (<2s p95 over 30d/10M rows) on the same infrastructure that makes `GetModelRankings` fast.

### PostgreSQL materialized-view option

`getModelRankingsFromMatView` (matviews.go:944) shortcuts model rankings on PG via a pre-aggregated table. Users have no equivalent matview today. Two options:

- **Option A** (chosen): raw-table query for all deployments in v1. Simple, stays below SC-002 for expected loads, no new migration. Matches what `GetModelRankings` does on SQLite deployments today.
- **Option B**: add a `mv_user_rankings` matview. Not in scope for v1; revisit when a customer reports a perf issue.

Decision: Option A. Matview is a follow-up spec if needed.

### Trend calculation

Model rankings compare current period to the immediately-preceding range of equal length (`prevStart = start - duration; prevEnd = start - 1ns`). Users will mirror this exact logic. Trend fields: `requests_trend`, `tokens_trend`, `cost_trend` (all float64, percentage change, `null`-semantics via `HasPreviousPeriod` flag when prior period was empty).

### User ID population

User-ID is written to `bifrost_logs.user_id` by the logging plugin when the request carries one (governance plugin's `UsageUpdate.UserID` path). Unattributed requests get `NULL` and are excluded from the aggregation per FR-003 (and to match how `GetModelRankings` excludes empty-model rows).

## Alternatives considered

- **Client-side aggregation from `/api/logs`**: Loads every matching row, crushes the browser on big ranges. Rejected.
- **Reuse `/api/logs/rankings?group_by=user_id`**: Would require plumbing a `dimension` parameter through the existing model-rankings handler. More invasive than a sibling method with an identical shape. Rejected.
- **Add the matview now**: Premature optimization; SC-002 is achievable without it on typical workloads. Rejected.

## Open questions

None. All symbols align; the shape is known; the perf envelope is bounded.
