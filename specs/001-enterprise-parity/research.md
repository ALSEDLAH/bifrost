# Phase 0 Research — Bifrost Enterprise Parity

**Branch**: `001-enterprise-parity` | **Date**: 2026-04-19
**Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

Each section records a **Decision**, **Rationale**, and **Alternatives
considered**. Research items are labeled `R-NN` and referenced from
plan.md and data-model.md.

---

## R-01 — Enforcing Core Immutability in CI

**Decision**: Add a CI job `check-core-unchanged` that runs
`git diff --stat v1.5.2..HEAD -- core/ core/schemas/ core/providers/
core/mcp/` and fails the build on any non-empty diff. The baseline
ref is a repo variable `CORE_BASELINE_REF` set to `v1.5.2` at
enterprise-parity kickoff and bumped only on deliberate MAJOR bumps
of the core module.

**Rationale**: Constitution Principle I is non-negotiable. A
human-reviewer check is insufficient because enterprise-parity work
touches many files; a drift is easy to miss. The diff-against-
baseline pattern is O(ms) and is unambiguous.

**Alternatives considered**:

- `go-mod-check` that blocks imports of `core/` from unexpected
  places — rejected because the import direction is already
  correct; the risk is direct edits, not unauthorized imports.
- Git pre-receive hook — rejected because it blocks local
  development and is brittle across contributor environments.

---

## R-02 — Tenant Context Propagation Pattern

**Decision**: Introduce `framework/tenancy.TenantContext` carrying
`OrganizationID`, `WorkspaceID`, `UserID`, `RoleScopes`, and
`ResolvedVia` (e.g., `sso-session`, `admin-api-key`,
`service-account-key`, `virtual-key`). Populate it in the HTTP
transport middleware layer (`transports/bifrost-http/lib/
middleware.go`) using a deterministic resolution order:

1. `Authorization: Bearer <admin-api-key>` → look up org + scopes.
2. `Authorization: Bearer <service-account-key>` → look up
   workspace + scopes.
3. Session cookie → look up user → org/workspace memberships.
4. `x-api-key: <virtual-key>` → look up workspace.

On resolution, set `BifrostContextKeyOrganizationID`,
`BifrostContextKeyWorkspaceID`, `BifrostContextKeyRoleScopes` in
the existing `BifrostContext`. These context keys are **new**
(additive), match the reserved-keys pattern in
`core/schemas/context.go`, and are written ONLY by the middleware
+ `plugins/enterprise-gate/` — plugin writes are blocked via the
existing `BlockRestrictedWrites()` mechanism.

**Rationale**: `BifrostContext` is already the canonical
cross-plugin data carrier; reusing it (a) keeps plugins decoupled
from each other, (b) requires zero changes to plugin hook
signatures (Principle II), and (c) matches the governance plugin's
existing pattern for setting selected-key IDs.

**Alternatives considered**:

- Passing tenant in every plugin-config call — rejected: creates
  cross-plugin coupling and violates Principle X.
- New framework-level goroutine-local storage — rejected: Go's
  idiom is context propagation, and `BifrostContext` already
  exists for this purpose.

---

## R-03 — Multi-Tenancy Schema Pattern

**Decision**: Every new enterprise table is a GORM struct in
`framework/<store>/tables-enterprise/` carrying tenancy fields:

```go
package tables_enterprise

import "time"

type TableWorkspace struct {
    ID             string `gorm:"primaryKey;type:varchar(255)" json:"id"`
    OrganizationID string `gorm:"type:varchar(255);not null;index:idx_workspaces_org_slug,unique" json:"organization_id"`
    Slug           string `gorm:"type:varchar(255);not null;index:idx_workspaces_org_slug,unique" json:"slug"`
    // ...
    CreatedAt time.Time `gorm:"index;not null" json:"created_at"`
    UpdatedAt time.Time `gorm:"index;not null" json:"updated_at"`
}
func (TableWorkspace) TableName() string { return "ent_workspaces" }
```

Composite indexes use GORM's `index:<name>,unique` tag idiom;
`(organization_id, workspace_id, <feature_key>)` is the standard
read-path shape. Row-Level Security is NOT used; every repository
function accepts a `TenantContext` and filters explicitly with
GORM's `.Where("organization_id = ? AND workspace_id = ?", ...)`.

**Existing upstream tables that need tenancy** (e.g.,
`TableVirtualKey`, `TableTeam`, `TableCustomer`) MUST NOT be
modified — that would violate Principle XI rule 1. Instead, create
a 1:1 sidecar in `tables-enterprise/`:

```go
type TableVirtualKeyTenancy struct {
    VirtualKeyID   string `gorm:"primaryKey;type:varchar(255)" json:"virtual_key_id"`
    OrganizationID string `gorm:"type:varchar(255);not null;index" json:"organization_id"`
    WorkspaceID    string `gorm:"type:varchar(255);not null;index" json:"workspace_id"`
}
func (TableVirtualKeyTenancy) TableName() string { return "ent_virtual_key_tenancy" }
```

Repositories joining tenancy onto upstream rows do an explicit
`LEFT JOIN ent_virtual_key_tenancy ON ent_virtual_key_tenancy.virtual_key_id = virtual_keys.id`
— rows without tenancy assignment fall into the synthetic default
organization (which always has a sidecar row written at first-
boot migration time). Upstreaming the columns later (Principle XI
rule 7) collapses the sidecar into the main struct.

**First-boot tenancy backfill** (in
`migrations_enterprise.go`): the `E000_seed_default_org` migration
inserts the synthetic default org + workspace + a sidecar row for
every pre-existing virtual key/team/customer. Idempotent:
re-running on a migrated store is a no-op.

**Rationale**:

- Explicit filters in Go code are grep-able and reviewable; RLS
  hides the filter in a DB policy that's easy to forget when
  adding a new backend.
- Composite indexes match the query pattern (99% of reads scope to
  one tenant).
- Sidecar tables avoid editing upstream's `tables/` directory,
  preserving Principle XI rule 1 (additive-by-sibling-file).
- GORM AutoMigrate on the sidecar is idempotent and reversible
  via `migrator.Rollback`.

**Alternatives considered**:

- PostgreSQL Row-Level Security policies — rejected: SQLite-backed
  configstore can't use them; inconsistent enforcement surface.
- Modifying upstream GORM structs to add tenancy columns —
  rejected: violates Principle XI rule 1; merge conflicts on every
  upstream change to those structs; only acceptable if upstream
  accepts the change (R-26 upstreaming bias).
- Schema-per-org — rejected: operationally prohibitive at N>100
  orgs and breaks the single-org v1 contract.

---

## R-04 — Guardrail Composition & Execution Order

**Decision**:

- **Order**: Organization-scoped guardrails run before
  workspace-scoped, which run before virtual-key-scoped. Deny
  at any level is authoritative.
- **Mode**: Sync-parallel by default (all guardrails launch on
  goroutines, the request blocks on the earliest deny); sync-
  sequential available per-guardrail config; async supported for
  log-only guardrails (fire-and-forget).
- **Composition policy**: Default is any-deny. Configurable to
  all-must-pass or majority via `composition_policy` per
  guardrail scope.
- **Actions**: `deny` → HTTP 446 with reason; `warn` → HTTP 246
  with reason header; `retry` → re-execute with modified prompt;
  `fallback` → route to configured alternate model/provider;
  `log_only` → record without affecting response.
- **Partner guardrail failure**: `fail_closed` (default) rejects
  the request; `fail_open` allows; alert fires in either case.

**Rationale**: Matches Portkey's guardrail semantics precisely for
parity (SC-003), and the any-deny-wins pattern matches every
regulated customer's stated policy.

**Alternatives considered**:

- All-must-pass as default — rejected: minor guardrail flakiness
  would reject production traffic.
- Single scope level — rejected: customers universally want
  org-wide overlays on top of team policies.

---

## R-05 — BYOK Envelope Encryption Scheme

**Decision**: Two-tier envelope encryption.

- **KEK (Key Encryption Key)**: Customer-managed symmetric key in
  AWS KMS / Azure Key Vault / GCP KMS, addressed by ARN/ID in
  config.
- **DEK (Data Encryption Key)**: Per-record AES-256-GCM key
  generated locally via `crypto/rand`.
- **Ciphertext layout**: `version(1B) | kek_ref(32B hash) |
  dek_wrapped(varies) | nonce(12B) | ciphertext+tag(varies)`.
- **DEK caching**: In-memory LRU with configurable TTL (default
  15 min, range 1–60 min). Cache keyed by record-type or
  record-class for logstore; per-record for high-sensitivity
  configstore items (keys). Cache is process-local; no cross-
  instance sharing.
- **KEK rotation**: Re-wrap DEK on next write; reads try current
  KEK version first, fall back to `kek_ref` in ciphertext.
- **KEK revocation**: Cached DEKs survive until TTL; new writes
  fail with clear KMS error.

**Rationale**: Envelope encryption is the standard pattern that
decouples data-key generation from KMS round-trip cost (meeting
SC-009 performance target with a 15-min cache → ≤4 KMS calls/hour
per instance for a workspace with 10k writes/hour).

**Alternatives considered**:

- Direct KMS encryption per record — rejected: 5k RPS × KMS
  round-trip (8–40ms) is operationally prohibitive and costly.
- Single long-lived DEK per tenant — rejected: key-compromise
  blast radius is every record.

---

## R-06 — Air-Gapped Feature Matrix Verification

**Decision**: The `helm-charts/bifrost/values-airgapped.yaml`
profile explicitly lists in comments the supported user stories
(US1, US2, US3 OIDC-only, US4, US5, US18) and forcibly disables
the rest via `enabled: false`. A CI smoke test runs the profile
in a network-restricted Kubernetes-in-Docker cluster with egress
blocked except to the AWS/Azure/GCP KMS endpoints and validates:

1. The pod starts successfully.
2. A golden request set completes end-to-end.
3. Zero outbound connections to non-whitelisted endpoints
   (captured via an eBPF-based connection monitor running in the
   pod).

The smoke test is gated by a `NETWORK_EGRESS_RESTRICTED=true` env
var and runs on a scheduled matrix (AWS, Azure, GCP) weekly
plus on every Helm-chart PR.

**Rationale**: Air-gapped is a verifiable claim; a CI check is the
only durable guarantee. Matches Principle VIII (real dependencies,
not mocks).

**Alternatives considered**:

- Manual quarterly validation — rejected: regression surface is
  large; once drifted, the claim rots silently.
- Static Helm-chart manifest inspection — rejected: egress can
  occur from runtime code paths that a manifest doesn't reveal.

---

## R-07 — PII Detection Engine + Benchmark

**Decision**:

- **Engine**: Two-pass detection. Pass 1 is deterministic regex +
  rule patterns (SSN, credit-card Luhn-valid, IBAN, phone,
  email, IPv4/v6, MAC, common PHI identifiers). Pass 2 is an
  optional NER model (default: `microsoft/Presidio-analyzer`
  inline via a small Go shim over Python; or, for air-gapped,
  a compiled ONNX model running with `onnxruntime-go`).
- **Benchmark**: Use the open-source
  [PII-NER eval corpus](https://huggingface.co/datasets/
  Isotonic/pii-masking-200k) + internal synthetic set; target
  ≥98% recall and ≤2% false-positive rate (SC-007).
- **Replacement**: `<REDACTED:<TYPE>>` placeholders preserve
  position; deterministic hash placeholders
  `<REDACTED:SSN:ab12ef>` when configurable consistent-
  redaction mode is on.
- **Mode**: `redact-in-logs-only` (default) or
  `redact-before-provider` (more protective, may degrade LLM
  output quality).

**Rationale**: Deterministic pass catches 80%+ at zero ML cost;
NER pass catches names/addresses that regex can't. Presidio is
industry-standard and Apache-licensed. ONNX path keeps air-gapped
viable.

**Alternatives considered**:

- AWS Comprehend / Azure Cognitive Services PII — rejected: egress
  dependency kills air-gapped viability.
- Regex-only — rejected: misses names and free-text addresses.

---

## R-08 — Log Export Reliability Pattern

**Decision**:

- Each export destination runs in its own goroutine with a
  bounded buffered channel (default capacity 10k records).
- Export loop: collect batch → compress (gzip) → upload with
  retries (exponential backoff, max 5 attempts, 5-min ceiling) →
  on persistent failure, append to a dead-letter local store
  (`framework/logstore` table `export_dead_letters`) and fire
  an alert.
- Streaming mode: flush every `stream_interval_seconds` (default
  60s) or when buffer is 80% full.
- Scheduled mode: flush on cron expression (typical: hourly or
  daily).
- Backpressure: when buffer is full, drop newest and increment
  `export_dropped_total` metric. Alerting on dropped → 0 is the
  primary health signal.

**Rationale**: The bounded-buffer + dead-letter pattern is the
standard reliability design for log pipelines (see Fluentd,
Vector). Drop-newest rather than block-request keeps the request
hot path unaffected.

**Alternatives considered**:

- Kafka-based export — rejected: adds operational burden that
  small enterprise deployments don't want.
- Unbounded buffers — rejected: memory leak under sustained
  destination outage.

---

## R-09 — Observability Completeness CI Check

**Decision**: Add a CI job `check-obs-completeness` that:

1. Enumerates every plugin under `plugins/` whose name is in the
   enterprise-feature registry (`plugins/enterprise-gate/
   features.go`).
2. Static-analyzes each plugin to verify it contains at least one
   call to (a) `otel.Tracer(...).Start(...)`, (b) a
   telemetry-registered counter, (c) an audit-sink emit.
3. Reports missing signals as build failures unless the plugin
   is explicitly allow-listed with a `// obs-exempt: <reason>`
   comment (these appear in `plan.md` "Deferred Observability").

**Rationale**: Principle VI requires all three signals; a human
reviewer often catches two of three. Automated enumeration is
cheap and permanent.

**Alternatives considered**:

- Runtime check at startup — rejected: only catches wiring
  errors, not missing instrumentation.
- Manual review checkbox in PR template — rejected: drifts.

---

## R-10 — Import-Direction Enforcement

**Decision**: A `make check-imports` target runs `go list` across
every module and asserts:

- `core/` imports: only stdlib + `core/`-internal.
- `framework/` imports: core + stdlib + framework-internal.
- `plugins/X/` imports: core + framework + stdlib + external
  third-party. Plugin X MUST NOT import plugin Y.
- `transports/` imports: core + framework + plugins (via
  `plugin.Open` only) + stdlib + external.
- `ui/` is TypeScript and doesn't participate.

Violations fail the build with a human-readable message.

**Rationale**: The workspace (`go.work`) silently resolves
reverse imports at dev time. Only release-tagging surfaces
them, often far from the change that caused them. An early
check prevents surprise release blockers.

**Alternatives considered**:

- Go's built-in package cycle detection — insufficient; cycles
  within a legal import direction still pass.
- `depguard` linter — acceptable alternative; kept as a
  compatible choice but `make check-imports` is lower-friction
  in this repo.

---

## R-11 — Migration Rollback Plan

**Decision**:

- Every enterprise migration registered with `migrator.New(...)`
  supplies a `Rollback func(*gorm.DB) error` paired with the
  `Migrate` function (see `framework/migrator/migrator.go`). This
  is gormigrate-native; rollback is a first-class concept.
- First-boot enterprise migrations run via a
  `RegisterEnterpriseMigrations(db *gorm.DB)` function called
  from the enterprise plugin's `Init()`. Sidecar tables are
  created only when the relevant feature is enabled in config.
- The synthetic-default-org UUID is fixed per deployment and
  persisted in a `TableSystemDefaults` row, never regenerated.
  Idempotency: re-running migrations on an already-migrated
  store is a no-op (migrator tracks applied IDs in a tracking
  table).
- Rollback to v1.5.2: invoke each enterprise migration's
  `Rollback` in reverse order via `migrator.RollbackMigration` /
  `migrator.RollbackTo`. This drops sidecar tables and any
  enterprise tables. Upstream's `tables/` is unchanged
  throughout, so v1.5.2 data is preserved by default.

**Rationale**: Rollback discipline is non-negotiable for
enterprise customers; "upgrades only" is rejected by most
procurement teams.

**Alternatives considered**:

- Irreversible-only migrations — rejected: violates enterprise
  change-management norms.

---

## R-12 — SSO Session Model & Break-Glass Local Login

**Decision**:

- After SSO callback, issue a signed session cookie (HS256, 12h
  default) bound to org + user + active-workspace + role-scopes.
- IdP group → Bifrost role mapping configurable per org
  (defaults: "bifrost-owners" → Owner, "bifrost-admins" → Admin,
  everyone else → Member).
- Local username/password login is **disabled by default** in
  enterprise profile; admin can enable a "break-glass" list of
  specific emails with mandatory MFA and auditing. Every
  break-glass login fires a high-severity alert.

**Rationale**: Matches Portkey's enterprise pattern and SOC 2
/ ISO 27001 CC6.1 requirements.

**Alternatives considered**:

- Mandatory MFA for local even without break-glass list —
  rejected: adds MFA infra cost for a feature that's off by
  default.

---

## R-13 — Budget Counter Coordination Across Instances

**Decision**: Budget counters (spend-to-date, request-count,
token-count within windows) are kept in a shared counter store
that any Bifrost instance can atomically increment:

- **Primary backend**: Redis (already listed in the vectorstore
  backend list — reuse the connection pool where configured).
- **Fallback backend**: PostgreSQL row-level `UPDATE` with
  `FOR UPDATE SKIP LOCKED` for hot rows, acceptable up to
  ~200 RPS per key.
- Rate-limit windows use a sliding-window algorithm with 10
  buckets per window for smoothness.
- Clock skew mitigation: use the shared store's monotonic
  server-side `INCRBY` + TTL; avoid client-clock-based windows.

**Rationale**: A single Bifrost instance cannot accurately
enforce limits for a clustered deployment without shared
counters. Redis is the standard pick; the PostgreSQL fallback
covers deployments that don't run Redis.

**Alternatives considered**:

- Eventually-consistent counters via gossip — rejected:
  allows over-spend windows.
- Per-instance partitioning by virtual-key hash — rejected:
  unbalanced traffic per VK.

---

## R-14 — Admin API Key Storage & Rotation

**Decision**:

- Stored as `argon2id(hash)` with prefix `bf-admin-<8-char-id>`.
- UI displays the full key exactly once at creation; after that
  only the prefix.
- Scopes serialized as a compact bitmask + JSON for forward
  compatibility.
- Expiry stored as `expires_at TIMESTAMP`; middleware rejects
  expired keys.
- Rotation creates a new key and marks the old one `revoking`
  for a configurable grace period (default 60s) before hard
  revocation.
- Key creator + revoker + active-sessions-observed are tracked
  per key for audit.

**Rationale**: Matches admin-key hygiene expectations in
enterprise security reviews.

---

## R-15 — Alert Dispatcher Backpressure

**Decision**:

- Alert rules evaluated at a fixed cadence (configurable, default
  30s) by a single goroutine per rule.
- Notifications dispatched to a bounded queue per destination;
  on full queue, coalesce by (rule_id, severity) so one
  notification represents N evaluations.
- Deduplication: within a configured window (default 15 min),
  the same rule + destination fires once with a notified-count
  in the payload.
- Recovery notifications fire when metric stays below threshold
  for the cooldown window (default 5 min).

**Rationale**: Prevents pager storms during incidents. Standard
pattern from PagerDuty/Alertmanager designs.

---

## R-16 — Prompt Library Storage in Configstore (v1)

**Decision**:

- Table `prompts` with columns: `id`, `organization_id`,
  `workspace_id`, `folder_path`, `name`, `production_version_id`,
  `metadata JSONB`, `created_at`, `updated_at`.
- Table `prompt_versions` with columns: `id`, `prompt_id`,
  `version_number`, `content JSONB` (multimodal: text + image
  refs), `variables_schema JSONB`, `partials_refs UUID[]`,
  `created_by`, `created_at`, `commit_message`.
- Table `prompt_partials` with same tenancy + content, reusable
  across prompts.
- Folder access: folder-level scopes stored in
  `prompt_folder_scopes` with role-ID refs.
- `render` endpoint: Mustache engine expands variables + partials
  (depth limit 5 to prevent recursion bombs); returns rendered
  prompt.
- `completions` endpoint: render + execute against selected
  model.

**Rationale**: Configstore reuse (Clarifications Q5) keeps
infrastructure debt low. Separate tables rather than a single
JSONB blob for per-version query performance.

---

## R-17 — Canary Routing Primitive in Config Objects

**Decision**:

- Config object schema gains a `canary` block:
  ```json
  {
    "canary": {
      "enabled": true,
      "percentage": 10,
      "target": {
        "provider": "openai",
        "model": "gpt-5-preview"
      },
      "metric_collection_window_hours": 72
    }
  }
  ```
- Routing decision uses a stable hash of `(virtual_key_id,
  request_id)` modulo 100 to assign to canary vs primary, so a
  single user's experience is consistent within a window.
- Canary comparison report joins the per-leg
  (latency_p50/p95/p99, cost_per_request, error_rate,
  feedback_score) metrics over the collection window.

**Rationale**: Orthogonal to governance routing-chains (per
Clarifications Q3); keeps canary as a declarative, user-facing
concern while governance keeps its budget/VK focus.

---

## R-18 — Terraform Provider Approach

**Decision**:

- Framework: `hashicorp/terraform-plugin-framework` (v2, not the
  legacy SDK).
- Resources (v1): `bifrost_workspace`, `bifrost_virtual_key`,
  `bifrost_admin_api_key`, `bifrost_config`, `bifrost_guardrail`,
  `bifrost_alert_rule`, `bifrost_log_export`, `bifrost_sso_config`.
- Data sources: organization, workspaces list, users list.
- Backend: Admin API with bearer-token auth.
- Published to Terraform Registry.

**Rationale**: Framework v2 is the current standard; legacy SDK
is deprecated for new providers.

---

## R-19 — SCIM 2.0 Conformance Scope

**Decision**:

- Implement RFC 7644 endpoints: `/Users`, `/Groups`, `/Schemas`,
  `/ServiceProviderConfig`, `/ResourceTypes`.
- Filter support: `eq`, `ne`, `co`, `sw`, `ew`, `pr`, `and`,
  `or` (v1); `gt/lt/ge/le` in v1.1.
- PATCH operations: `add`, `replace`, `remove`.
- Bulk operations: deferred to v2.
- Conformance: pass RFC-7644 conformance suite (e.g.,
  `scim-for-developers/scim-2-compliance`).

**Rationale**: Okta, Azure AD, Google all require RFC-7644
conformance; bulk ops are rare in user-lifecycle use.

---

## R-20 — Executive Dashboard Query Strategy

**Decision**:

- Aggregate metrics are precomputed hourly into a rollup table
  `executive_metrics_hourly` keyed by
  `(organization_id, hour_bucket, metric_name, dimension_key)`.
- Dashboard queries read from rollup tables with a ≤2s target;
  when the selected range spans < 1 hour, the UI falls back to a
  live read from `logstore` for freshness.
- Rollup job runs every 15 minutes with an overlapping window to
  handle late-arriving data.

**Rationale**: 10M log entries per org would cost seconds per
ad-hoc aggregation. A rollup table is the standard approach.
Meets SC-011 (first-page <2s).

---

## R-21 — Retention Policy Enforcement

**Decision**:

- Daily cron job walks each workspace's retention policy and
  issues bounded `DELETE FROM logs WHERE workspace_id=$1 AND
  created_at < NOW() - $2 INTERVAL LIMIT $3` statements until the
  horizon is clear.
- LIMIT per batch (default 10k rows) avoids long-running locks.
- Audit-entry retention is the MAX of all workspace retention
  policies, capped at a deployment-level ceiling (default 7
  years) to satisfy "audit outlives feature data" requirement.
- Cold-storage tier is not implemented in v1 (per Assumption);
  customers needing longer storage use the Log Export feature.

---

## R-22 — Hot-Reload Pattern for Tenancy Config

**Decision**:

- SSO, SCIM, KMS, guardrails, alert rules, log exports, and
  retention policies are stored in configstore and cached
  in-memory behind `atomic.Pointer` slices following the
  existing governance-plugin pattern.
- A configstore change notification fans out via the existing
  pub/sub to all Bifrost instances; each instance atomically
  swaps its cached slice.
- SLO for propagation: 30 seconds p95 (FR-006, FR-022).

**Rationale**: Matches the existing Principle-compliant
hot-reload pattern; no new infrastructure required.

---

## R-23 — Data Lake ETL Pipeline (US23)

**Decision**:

- Curated export definitions live in configstore as
  `data_lake_exports` with fields: destination, schedule
  (cron), schema definition (column list + types), source
  filter (SQL WHERE), partitioning scheme.
- A per-export goroutine runs on schedule, executes the source
  filter against logstore/metrics, emits files to the
  destination in Parquet (S3/BigQuery) or Avro (Mongo).
- Failure path: failed run emits alert; next scheduled run
  includes the missed window (bounded to 7 days lookback).

**Rationale**: Curated exports differ from raw log export (US11)
in that they apply SQL projection and schema stability; keeping
them separate avoids forcing raw-export customers into the
overhead of schema validation.

---

## R-24 — Upstream Sync Strategy

**Decision**: Adopt a weekly-merge discipline against
`upstream/main` = `https://github.com/maximhq/bifrost.git`, enforced
by a combination of Constitution Principle XI rules, CI drift
watcher, and a dedicated operator runbook at `UPSTREAM-SYNC.md`.

**Concrete tactics** (all encoded as Principle XI sub-rules):

1. **File-touch minimization.** For every file we share with
   upstream, our fork's diff is ideally zero (new sibling file) and
   at worst a single-line hook call. The six "extended" plugins
   (governance, logging, telemetry, otel, semanticcache, prompts)
   receive ONLY sibling files (`budgets.go`, `pii_hook.go`, etc.);
   we never edit their `main.go`. Same for
   `transports/bifrost-http/lib/middleware.go`: a single
   `RegisterEnterpriseMiddleware(mw)` hook call is the only line we
   add upstream-side; the rest lives in
   `middleware_enterprise.go`.

2. **Migration namespace separation.** Enterprise migrations use
   `E###_<name>.sql` in
   `framework/<store>/migrations-enterprise/`. Upstream's `NNN_`
   sequential names and our `E###_` prefix sort disjoint on every
   runner and never collide on any filesystem. The migration runner
   (T019) discovers both dirs at boot.

3. **`config.schema.json` overlay.** Enterprise fields live in a
   sibling `config.schema.enterprise.json` pulled in via a single
   `allOf: [{ $ref: "./config.schema.enterprise.json" }]` entry in
   upstream's file. The overlay file is 100% ours; conflicts on the
   main schema are reduced to that one anchor line. Load-time
   validator composes the two.

4. **UI router hook.** Enterprise pages register through an
   `enterpriseRoutes.ts` array imported by upstream's router via a
   single import + spread: `[...upstreamRoutes, ...enterpriseRoutes]`.
   Upstream router code stays authoritative on the shared shape; our
   list grows freely.

5. **Weekly merge cadence.** A scheduled GitHub Action runs
   `git fetch upstream && git merge upstream/main --no-ff` every
   Monday at 09:00 UTC into a `sync/upstream-YYYY-MM-DD` branch,
   runs the full test matrix + golden-set replay, and opens a PR
   if green. Merge is NOT rebase: preserves the fork history so we
   can trace every upstream commit's arrival.

6. **CI drift watcher.** A GitHub Action on every PR computes
   `git diff --stat upstream/main -- <watch-list>` where the watch
   list is hard-coded in
   `.github/drift-watchlist.txt`. Any PR that increases the
   cumulative diff against a previous-week baseline by more than
   N lines (default 50) fails unless the commit message begins
   with `drift:` (explicit ack). The watch list includes: the six
   extended plugins' directories, `middleware.go`,
   `config.schema.json`, `AGENTS.md`, `CLAUDE.md`, top-level
   `Makefile`, `go.work`, `helm-charts/bifrost/values.yaml`.

7. **Upstream-carried patches registry.** `UPSTREAM-SYNC.md`
   carries a "Patches we maintain pending upstream acceptance"
   table. Every entry has a referenced upstream issue / PR and a
   removal plan. Entries with no movement for 90 days trigger a
   quarterly review.

8. **Upstreaming bias.** When a primitive is clearly generic
   (multi-tenancy schema, audit sink, crypto envelope), the
   maintainer files a PR against `maximhq/bifrost` before merging
   locally, so our fork's carry delta shrinks on acceptance.

**Rationale**: Forks that do not enforce discipline silently
accumulate file-level diffs that turn weekly merges into week-long
projects. Every rule above converts a would-be conflict into a
no-op or a one-line merge.

**Alternatives considered**:

- **Full rebase (`git rebase upstream/main`)** — rejected for
  long-lived fork: rewrites history, breaks already-pushed feature
  branches, hostile to reviewers following the fork.
- **Cherry-pick upstream commits individually** — rejected: does
  not scale past ~10 upstream PRs/week; operator burden high; loses
  causality between commits.
- **Quarterly merges** — rejected: the conflict surface at 3-month
  cadence is an order of magnitude worse than weekly. Mergelocks
  are the fork-killer.
- **Shared-file override via Go build tags** — rejected: violates
  Principle IV (no build tags for feature gating) and doesn't
  address UI/schema/docs files anyway.

**Impact on tasks.md**: Adds Phase 1.5 "Upstream Sync Tooling" with
tasks T320–T326 (upstream remote + weekly action, drift watcher CI,
schema overlay loader, migration namespace discovery, hook-point
refactors for middleware + UI router, UPSTREAM-SYNC.md authoring).

---

## R-25 — Deployment Mode Strategy

**Decision**: Introduce a top-level `deployment.mode` configuration
enum with values `cloud | selfhosted | airgapped`. The mode is read
at boot, set as an immutable global, and consumed by the plugin
loader + middleware chain to select which plugins register and
which defaults apply. Per the Deployment Modes table in plan.md,
each mode produces opinionated behavior for multi-org, telemetry,
licensing, metering, SSO surface, and air-gap posture. All three
modes share 100% of code paths; they differ only in configuration
and plugin registration.

**Rationale**: Operators should not have to flip five flags (multi-
org, telemetry, metering, license, air-gap) to reach a coherent
deployment shape. One mode flag drives a table of opinionated
defaults that each sub-flag can still individually override for
edge cases.

**Alternatives considered**:

- Separate binaries per mode — rejected: violates Principle IV
  (config-driven gating, no forked binaries).
- Separate Helm charts per mode — rejected: duplicates upgrade
  runbooks and confuses customers choosing between SKUs.
- Per-feature flags only (no mode aggregator) — rejected: cognitive
  load on operators, inconsistent defaults across deployments.

**Hybrid non-goal**: A Portkey-style "data plane in customer VPC,
control plane in vendor cloud" split is explicitly deferred. If
demand materializes, it adds a fourth mode value without changing
the three existing.

---

## R-26 — License Key Design (Self-Hosted)

**Decision**: Offline-verifiable signed license file,
JWT-formatted (RFC 7519), signed with Ed25519 (or RSA-4096
fallback). The vendor's public key is embedded in the Bifrost
binary at release-build time. License claims:

```json
{
  "iss": "bifrost-license-authority",
  "sub": "customer-id-or-name",
  "iat": 1713542400,
  "exp": 1745078400,
  "nbf": 1713542400,
  "entitlements": {
    "workspaces": true,
    "rbac": true,
    "sso": true,
    "audit_logs": true,
    "admin_api_keys": true,
    "central_guardrails": true,
    "pii_redaction": true,
    "budgets_rate_limits": true,
    "custom_guardrail_webhooks": true,
    "alerts": true,
    "log_export": true,
    "executive_dashboard": true,
    "retention_policies": true,
    "prompt_library": true,
    "prompt_playground": true,
    "configs": true,
    "service_account_keys": true,
    "byok": true,
    "airgapped": true,
    "scim": true,
    "terraform": true,
    "canary": true,
    "data_lake_export": true
  },
  "limits": {
    "max_workspaces": 50,
    "max_users": 500,
    "max_virtual_keys": 1000
  },
  "grace_period_days": 14,
  "customer_contact_email": "ops@customer.example"
}
```

**Verification flow** (offline):

1. Read license file (filesystem path OR UI upload).
2. Decode JWT header, confirm `alg: EdDSA` (or `RS256` fallback).
3. Verify signature against embedded public key.
4. Validate `iat` ≤ now, `nbf` ≤ now. (Expiry checked separately
   for grace-period logic.)
5. Parse claims; populate license plugin's in-memory state.
6. Set `BifrostContextKeyLicense` on every subsequent request so
   gated features can call `IsEntitled(feature)`.

**Expiry behavior**:

- `now < exp`: full operation; warning banners at 30/7/1 days
  remaining.
- `exp ≤ now < exp + grace_period_days`: full operation; critical
  banner visible to admins; daily audit entry
  `license.grace_period_active`.
- `now ≥ exp + grace_period_days`: enterprise features return
  HTTP 402 with renewal link; OSS core unchanged; existing
  enterprise data remains READABLE (read-only gate on write
  endpoints in the audit + governance + workspace middleware).
  A renewal via fresh license upload re-enables within 60 seconds.

**Revocation**: Not via online CRL (would violate air-gapped
Principle XI rule 6). Instead, vendor can reissue a replacement
license with a shorter `exp` and rely on customer upload. A
`license.revoked_keys` embedded block with key-id wildcards is a
future extension; v1 relies on expiry-driven rotation.

**Signing key rotation (clarified 2026-04-19)**: Binaries embed
an ORDERED ARRAY of valid vendor public keys (not a single key):

```go
var VendorPublicKeys = [][]byte{
    PubKey_v1_2026,  // current primary
    PubKey_v2_2027,  // next — activates when rotation begins
}
```

On verification, the license plugin tries each key in order and
accepts the license if any key validates the signature. Rotation
flow:

1. **Generate** new keypair in the password-managed vault
   (never in the repo).
2. **Append** new public key to the `VendorPublicKeys` array in
   the next binary release (both keys valid in parallel).
3. **Begin issuing** new licenses signed with the new private key
   while continuing to honor licenses signed with the old key.
4. **After** all customers are known to be on a binary version
   that embeds the new key (for air-gapped, this may take the
   longest-deployed customer's upgrade cadence — 6-12 months),
   drop the old public key from the array in a subsequent binary
   release.

Cadence: 12-month rotations as standard hygiene; emergency
rotation on suspected compromise follows the same flow compressed
to days if needed. The air-gapped-support guarantee is the hard
constraint — no rotation drops a key before the longest-deployed
air-gapped binary has been replaced.

**Issuance tooling (clarified 2026-04-19)**: Licenses are issued
by a CLI tool `tools/license-authority/` run manually by vendor
ops (v1). The tool reads the Ed25519 private key from a locally-
mounted file (sourced from password-manager vault at issuance
time) and outputs a signed JWT license file. The tool is NOT a
production service; it runs only on a vendor-ops workstation and
is never exposed to the network. v2 may replace it with a
Stripe-webhook-driven auto-issuance service for cloud-to-self-
hosted hybrid customers; that path is out of scope for v1.

**Limits enforcement (clarified 2026-04-19)**: When a license's
declared limits are exceeded, the license plugin applies a
"block-new, allow-existing" policy. `IsEntitled(feature)` adds a
`CheckLimit(dimension, current_count)` companion that the
resource-creation endpoints call. On limit breach,
`max_workspaces` and `max_virtual_keys` return HTTP 402 on new
creation (hard cutoff); `max_users` allows up to 110% of declared
count with escalating warnings before becoming hard at 110%. This
prevents production outages while forcing renewal conversations.

**Rationale**: JWT + Ed25519 is industry standard (GitLab, Sentry
self-hosted use this pattern). Offline verification is
non-negotiable for air-gapped. Grace period avoids the "license
expired at 2am Saturday and took prod down" incident.

**Alternatives considered**:

- Custom binary format — rejected: reinvents JWT without the
  tooling / library support.
- Online license check (periodic call-home) — rejected: violates
  air-gapped requirement (Principle XI rule 6 / FR-037).
- Hardware dongle — rejected: impractical at enterprise scale and
  hostile to container deployment.
- No expiry (perpetual license) — rejected: commercial model
  incompatible; customer-support for renewals becomes impossible
  to track.

**Implementation shape**:

- New `plugins/license/` module with `go.mod`.
- Implements `LLMPlugin` for request-time entitlement context keys
  + a custom `LicensePlugin` interface with `IsEntitled(feature)`,
  `DaysUntilExpiry()`, `InGracePeriod()`.
- Load order: `pre_builtin` with `order: -100` so it runs before
  the enterprise-gate plugin that reads its context keys.
- Not loaded in `deployment.mode: cloud` — cloud deployments
  operated by the vendor don't need a license file.

---

## R-27 — Cloud Billing Architecture (Stripe)

**Decision**: Stripe as the sole billing backend for Train E.
Integration uses (a) **Subscriptions API** for the base monthly
charge per tier, (b) **Metered Billing API** (usage records pushed
per billing cycle) for request-count overage, (c) **Invoices API**
for PDF generation, (d) **Customer Portal** for self-service
payment-method updates, (e) **webhook endpoint** on Bifrost for
subscription events (created, updated, canceled, payment_failed).

**Architecture**:

```
  [Request path]
      │
      │ (every request)
      ▼
  plugins/metering ─── increments per-org counters in Redis
      │                (request, input_tokens, output_tokens, cost_micros)
      │
      │ (every 15m)
      ▼
  framework/metering/rollup ─── flushes to configstore daily rollups
                                (meter_daily, meter_monthly tables)
      │
      │ (monthly billing cycle close)
      ▼
  plugins/billing/stripe ─── pushes usage records to Stripe
                             Metered Billing API; Stripe generates
                             invoice and charges payment method
      │
      │ (webhook)
      ▼
  transports/.../handlers/stripe_webhook.go ─── updates
                             subscription state, tier, dunning flag
```

**Tier definitions** (indicative; confirmed with stakeholders):

| Tier | Base | Overage | Rate limit | Retention | Guardrails | SSO |
|------|------|---------|------------|-----------|------------|-----|
| Dev (free) | $0 | not possible | 5 req/min | 3 days | no | no |
| Prod | $49/mo | $9 / 100k req | 1000 req/min | 30 days | yes | no |
| Enterprise | custom | contract | custom | custom | yes | yes |

**Billing cycle**: Monthly, aligned to subscription start date.
Usage accumulates in Redis counters, flushes to daily rollups every
15 minutes, and to a monthly roll-up at cycle close. Stripe usage
records pushed at cycle close with idempotency keys.

**Dunning**: On `invoice.payment_failed` Stripe webhook, Bifrost
flags the org as `dunning=true`; a 14-day grace period follows
during which the org is read-only on admin endpoints (completions
continue so the customer's product doesn't break). On
`invoice.payment_succeeded`, flag clears.

**Tier transitions**:

- Dev → Prod: Stripe Checkout flow → payment success webhook →
  Bifrost updates org tier record → new limits apply within 60s
  via the existing hot-reload pattern (R-22).
- Downgrades: apply at period end (not mid-cycle) to avoid
  mid-cycle feature loss surprise.
- Enterprise: UI captures "request enterprise" form → routes to
  vendor sales workflow (no self-service upgrade).

**Rationale**: Stripe is the standard for SaaS billing; their
Metered Billing API (launched 2023) natively supports usage-based
components and avoids reinventing invoice rendering + tax + PCI.

**Alternatives considered**:

- Chargebee / Paddle / Orb — rejected for v1: Stripe's ecosystem
  + docs + Go SDK + customer recognition is unmatched.
- Custom invoicing — rejected: PCI compliance + tax computation
  (US sales tax, EU VAT, etc.) alone would dwarf the rest of
  Train E.
- Self-hosted billing subsystem (for cloud re-sell customers) —
  deferred: can layer on top later by making the billing plugin
  pluggable.

**Data boundary**: Stripe holds PII (name, email, payment method).
Bifrost stores Stripe customer IDs + subscription IDs only; no
card numbers or CVCs ever touch Bifrost's storage (Stripe hosted
payment method UI handles that).

---

## Summary of Cross-Cutting Research Outcomes

| Area | Decision Anchor | Impacts |
|------|-----------------|---------|
| Tenancy enforcement | R-02, R-03 | US1, US2, every feature |
| Encryption | R-05 | US18, FR-035, FR-035a |
| CI guarantees | R-01, R-06, R-09, R-10 | Constitution Principles I, VI, X |
| Reliability patterns | R-08, R-13, R-15 | US10, US11, US8 |
| Storage shapes | R-03, R-16, R-20, R-21 | data-model.md |
| Auth model | R-12, R-14, R-19 | US3, US5, US20 |
| Upstream sync | R-24 | Constitution Principle XI, Phase 1.5 tasks |
| Deployment modes | R-25 | Plan "Deployment Modes" section, Phase 1.5 T327 |
| Licensing (self-hosted) | R-26 | US24, US25, plugins/license/, Phase 8 Train E |
| Cloud billing | R-27 | US26-US29, plugins/billing/, Phase 8 Train E |

All items marked NEEDS CLARIFICATION in the Technical Context
have been resolved.
