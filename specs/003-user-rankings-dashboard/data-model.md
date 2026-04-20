# Data Model — User Rankings Dashboard

## No new entities

All Go structs and TypeScript types are **already declared** (see research.md). This spec wires them together; it doesn't define new shapes.

## Reused types (authoritative pointers, not re-declarations)

| Symbol | Location | Purpose |
|---|---|---|
| `framework/logstore.UserRankingEntry` | tables.go:1439 | Per-user aggregation row |
| `framework/logstore.UserRankingTrend` | tables.go:1447 | Prior-period comparison |
| `framework/logstore.UserRankingWithTrend` | tables.go:1455 | Row + trend |
| `framework/logstore.UserRankingResult` | tables.go:1461 | API response envelope |
| `framework/logstore.SearchFilters` | tables.go | Reused filter struct (StartTime/EndTime/OrganizationID/UserIDs) |
| `ui/lib/types/logs.UserRankingEntry` + `UserRankingsResponse` | logs.ts:1113-1130 | Frontend mirrors (already field-aligned with Go JSON tags) |

## Query pattern (derived from `GetModelRankings`)

```sql
SELECT
  user_id,
  COUNT(*)                AS total_requests,
  COALESCE(SUM(total_tokens), 0) AS total_tokens,
  COALESCE(SUM(cost), 0)  AS total_cost
FROM bifrost_logs
WHERE status IN ('success', 'error')
  AND organization_id = ?
  AND user_id IS NOT NULL AND user_id != ''
  AND created_at BETWEEN ? AND ?
GROUP BY user_id
ORDER BY total_cost DESC
LIMIT 100;
```

A second query over the immediately-preceding period of equal length populates trend data via a `map[userID]UserRankingEntry` lookup, mirroring the exact flow at `rdb.go:1555-1614` for model rankings.

## Validation rules

- `limit` ≤ 100 (cap matches `defaultMaxRankingsLimit` used by model rankings).
- `from` / `to` must be valid RFC3339 when present; missing → default to last 24 h (matches `parseHistogramFilters` behavior).
- `sort_by` is one of `cost` / `tokens` / `requests`; any other value defaults to `cost`. SQL ordering expression is selected server-side (not user-templated) — no SQL-injection surface.
- `user_id = ''` rows are excluded at the `WHERE` level; no phantom "(unknown)" row in the response.
