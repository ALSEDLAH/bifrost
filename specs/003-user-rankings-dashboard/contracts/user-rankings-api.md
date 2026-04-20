# Contract — `GET /api/logs/user-rankings`

## Request

```
GET /api/logs/user-rankings?from=<RFC3339>&to=<RFC3339>&limit=<N>&sort_by=<cost|tokens|requests>
```

All params optional. Defaults match `parseHistogramFilters`:

- `from`: 24 h ago if absent
- `to`: now if absent
- `limit`: 25 (max 100)
- `sort_by`: `cost`

Authentication: same middleware chain as other `/api/logs/**` endpoints (basic-auth or session Bearer).
Tenant scoping: `organization_id` is read from the resolved `TenantContext`; not a query param.

## Response

`200 OK`

```json
{
  "rankings": [
    {
      "user_id": "alice@example.com",
      "total_requests": 1247,
      "total_tokens": 983412,
      "total_cost": 42.88,
      "trend": {
        "has_previous_period": true,
        "requests_trend": 12.4,
        "tokens_trend": 8.1,
        "cost_trend": 15.2
      }
    }
  ]
}
```

Trend fields are **percentage change** vs. the immediately-preceding period of equal length. When the preceding period has no rows for the user, `has_previous_period` is `false` and the trend numbers are ignored by the UI.

## Errors

- `500` — storage error. Body: `BifrostError` shape.
- No `404`, no `400` — empty results return `{rankings: []}` with 200.

## Performance

- p95 <2 s for 30-day window over 10M log rows (SC-002).
- Backed by existing `idx_logs_user_id` + `idx_logs_created_at`; no new indexes required.

## Observability

- Request path logged through existing logging middleware.
- No new metrics required.
