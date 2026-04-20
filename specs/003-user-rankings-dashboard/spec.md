# Feature Specification: User Rankings Dashboard

**Feature Branch**: `003-user-rankings-dashboard`
**Created**: 2026-04-20
**Status**: Draft
**Input**: User description: "make user rankings tab work — operator needs top-users analytics (requests, tokens, cost) derived from existing logs"

## Context

Spec 002 audit (`research.md` row 20) flipped `user-rankings/userRankingsTab.tsx` to a `FeatureStatusPanel` because the forward-declared `UserRankingsResponse` type in `ui/lib/types/logs.ts` had no backing endpoint. Spec 001's SR-01 descoped the whole story as "needs own spec". This is that spec.

Upstream already has:

- `bifrost_logs` table with `user_id`, `tokens_used`, `cost`, `status`, `created_at` columns (populated on every request by the logging plugin).
- `GET /api/logs` list endpoint with filter params including `user_ids`.
- `GET /api/logs/rankings` returning **model** rankings (different dimension).
- `GET /api/logs/filterdata` returning an `users` array of `KeyPair` entries.
- Frontend type `UserRankingsResponse` + `UserRankingEntry` already declared.

**What's missing:** a single aggregation endpoint that GROUPs by `user_id` over a time range and returns per-user totals (requests, tokens, cost) + a small trend. Adding it is a sibling file on the existing logging handler (no new plugin, no schema change) — the smallest-cost new backend work on the whole SR-01 out-of-scope set.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Executive views top users by cost (Priority: P1)

As a **platform lead** presenting usage to leadership, I open the dashboard's "User Rankings" tab over a configurable time range (last 7d / 30d / custom) and see a ranked table of the top users by total request cost in that range, including request count and total tokens. I can sort by cost, tokens, or requests.

**Why this priority**: Top-users reporting is the most-requested exec dashboard slice. One table unblocks a weekly business ritual.

**Independent Test**: Generate 20+ requests across 3 distinct `user_id` values with different costs over the last 7 days. Navigate to `/workspace/dashboard` → "User Rankings" tab. Table shows those 3 users sorted by cost descending, with per-user totals matching a manual SQL check against `bifrost_logs`.

**Acceptance Scenarios**:

1. **Given** logs exist for N users in the selected time range, **when** the tab loads, **then** the top-K users (default 25) render in a table with columns: user, requests, tokens, cost, trend vs. prior period.
2. **Given** an empty log range, **when** the tab loads, **then** an empty state renders with a helpful explainer ("No user-attributed requests in this range") and the page does NOT show a `FeatureStatusPanel`.
3. **Given** the user changes the time range or sort column, **when** the request completes, **then** the table updates without a full-page reload.

---

### User Story 2 — Drilldown to a single user's activity (Priority: P2)

As a **platform lead**, I click a user's row and land on the existing logs page pre-filtered to that user so I can see their actual requests.

**Why this priority**: Aggregations are useful but leadership often wants "what did Alice actually run?". Reusing the existing logs page's `user_ids` filter gives us this for free.

**Independent Test**: Click any user row; verify the URL becomes `/workspace/logs?user_ids=<id>` and the logs table filters correctly.

**Acceptance Scenarios**:

1. **Given** a user row on the rankings table, **when** the operator clicks it, **then** they navigate to `/workspace/logs?user_ids=<id>` with the logs filter pre-applied.

---

### Edge Cases

- **Logs with no `user_id`**: unattributed requests are excluded from the ranking (not a phantom "(unknown)" row). The ranking is deliberately about identifiable users.
- **Same `user_id` across orgs**: ranking is scoped to the caller's organization via existing tenant context; cross-org rows never appear.
- **Very large time ranges**: aggregation query is index-supported (`(organization_id, user_id, created_at)`); 30-day ranges over 10M rows must return within 2 s (matches SC-011 from spec 001).
- **Trend calculation when prior period has zero usage**: trend is reported as `null`, UI renders a "—" (no divide-by-zero or inflated +∞%).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose `GET /api/logs/user-rankings` accepting filter params `from`, `to`, `limit` (default 25, max 100), `sort_by` (one of `cost` / `tokens` / `requests`, default `cost`), `organization_id` (implicit from tenant).
- **FR-002**: Response MUST match the existing frontend type `UserRankingsResponse` shape: `{ rankings: UserRankingEntry[] }` where `UserRankingEntry = { user_id, total_requests, total_tokens, total_cost, trend }` and `trend = { has_previous_period, requests_trend, tokens_trend, cost_trend }`.
- **FR-003**: Aggregation MUST exclude log rows where `user_id` is empty / NULL.
- **FR-004**: Aggregation MUST filter by the caller's `organization_id` — no cross-tenant leakage.
- **FR-005**: Trend values MUST compare the selected range to the immediately-preceding range of equal length (e.g. last 7d vs. the 7d before that).
- **FR-006**: UI at `ui/app/enterprise/components/user-rankings/userRankingsTab.tsx` MUST render a real table (not a `FeatureStatusPanel`) with columns: user, requests, tokens, cost, trend arrows.
- **FR-007**: Clicking a user row MUST navigate to `/workspace/logs?user_ids=<id>` preserving the existing time-range query params.
- **FR-008**: The SC-020 check-script MUST still report zero violations after this feature ships (the stub flips from panel to real UI; no marketing copy anywhere).

### Key Entities

- **UserRankingRow** (runtime-only, not persisted): `{ user_id, total_requests, total_tokens, total_cost, trend }`. Derived from a SQL GROUP BY over `bifrost_logs`.
- No new tables, no new migrations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `/api/logs/user-rankings?from=…&to=…` returns the correct per-user aggregation for at least 3 distinct users over a 7-day range in a seeded test fixture. Values match a manual SQL aggregation over the same range.
- **SC-002**: p95 of the endpoint over a 30-day window spanning 10M log rows returns within 2 seconds (matches spec 001 SC-011 audit budget — same class of aggregation).
- **SC-003**: Zero occurrences of `"This feature is a part of the Bifrost enterprise license"` in the rendered DOM of `/workspace/dashboard` (User Rankings tab) after this feature ships. SC-020 continues to pass.
- **SC-004**: The existing dashboard page loads the rankings tab with no more than one HTTP round-trip; the previously-stubbed `FeatureStatusPanel` is gone.

## Assumptions

- `bifrost_logs` table has `user_id`, `tokens_used`, `cost`, `created_at`, `organization_id` columns already. Confirmed by inspection of `framework/logstore/tables.go` and related query code in `handlers/logging.go`.
- Writes to `bifrost_logs` populate `user_id` when the request carries one (via the existing governance plugin's `UserID` field on `UsageUpdate`). The ranking will therefore only be meaningful on requests that identify a user — expected per the feature's intent.
- The new endpoint piggybacks on the existing `LoggingHandler` — no new handler struct, just a new `getUserRankings` method alongside `getModelRankings`. Follows the upstream-mergeability discipline (sibling-method extension, not a new file).
- No UI navigation changes needed to reach the tab — `/workspace/dashboard` already exposes the User Rankings tab; only its content component flips from `FeatureStatusPanel` to the real view.
