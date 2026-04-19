# Data Model — Bifrost Enterprise Parity (Phase 1)

**Branch**: `001-enterprise-parity` | **Date**: 2026-04-19
**Plan**: [plan.md](./plan.md) | **Research**: [research.md](./research.md)

All new tables live in existing framework stores:

- **configstore** (SQLite or PostgreSQL via GORM): tenant data, auth,
  policy, prompt library, configs, routing rules, KMS bindings.
- **logstore** (SQLite or PostgreSQL via GORM): request/response logs,
  audit entries, guardrail events, alert events, executive rollups.

Conventions:

- All entries below describe **GORM struct fields**, not raw SQL
  columns. The actual implementation lives in
  `framework/configstore/tables-enterprise/*.go` and
  `framework/logstore/tables_enterprise.go` per Constitution
  Principle XI rule 2. See [research.md §R-03](./research.md) for the
  GORM struct pattern and sidecar-table convention.
- Migrations are registered as `*migrator.Migration` entries in
  `framework/<store>/migrations_enterprise.go` with IDs prefixed
  `E###_<descriptive_name>` (e.g., `E001_seed_default_org`,
  `E002_orgs_workspaces`). Each migration supplies a `Migrate` func
  and a `Rollback` func.
- All IDs are UUID v7 strings (`gorm:"primaryKey;type:varchar(255)"`).
- Every tenant-scoped table carries `organization_id` and (where
  applicable) `workspace_id` string fields, indexed compositely with
  the primary access key via `gorm:"index:<name>"` tags.
- Timestamps are `time.Time` with `gorm:"index;not null"`.
- Columns marked 🔐 are encrypted via `framework/crypto` (either
  configstore encryption_key or BYOK envelope per R-05).
- Columns marked 🔸 are never logged, exported, or emitted in metrics.
- A synthetic `SYSTEM_DEFAULT_ORG_UUID` is persisted in the
  `TableSystemDefaults` row and seeds the single-org v1 deployment.
- **Existing upstream tables that need tenancy** (e.g.,
  `TableVirtualKey`, `TableTeam`, `TableCustomer`) are NOT modified.
  Tenancy attaches via 1:1 sidecar tables (e.g.,
  `TableVirtualKeyTenancy`) per Principle XI rule 1.

---

## 1. Tenancy & Identity

### `organizations`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT NOT NULL | |
| is_default | BOOLEAN NOT NULL DEFAULT FALSE | exactly one row with TRUE in v1 |
| sso_required | BOOLEAN NOT NULL DEFAULT FALSE | |
| break_glass_enabled | BOOLEAN NOT NULL DEFAULT FALSE | |
| default_retention_days | INT NOT NULL DEFAULT 90 | |
| data_residency_region | TEXT | metadata, see Assumptions |
| created_at | TIMESTAMPTZ NOT NULL | |
| updated_at | TIMESTAMPTZ NOT NULL | |

### `workspaces`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL FK → organizations(id) | |
| name | TEXT NOT NULL | UNIQUE per org |
| slug | TEXT NOT NULL | URL-safe; unique per org |
| description | TEXT | |
| log_retention_days | INT | overrides org default |
| metric_retention_days | INT | overrides org default |
| payload_encryption_enabled | BOOLEAN NOT NULL DEFAULT FALSE | FR-035a |
| created_at | TIMESTAMPTZ NOT NULL | |
| updated_at | TIMESTAMPTZ NOT NULL | |
| deleted_at | TIMESTAMPTZ | soft delete, 30-day grace (edge case) |

Index: `(organization_id, slug)` unique; `(organization_id, deleted_at)`.

### `users`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL FK | |
| email | TEXT NOT NULL | |
| display_name | TEXT | |
| idp_subject | TEXT | SSO subject; NULL for local |
| local_password_hash | TEXT 🔸 | argon2id; break-glass only |
| mfa_secret | TEXT 🔐 | TOTP seed for break-glass |
| status | ENUM(active, suspended, pending) | |
| last_login_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ NOT NULL | |
| updated_at | TIMESTAMPTZ NOT NULL | |

Index: `(organization_id, email)` unique; `(organization_id, idp_subject)` unique.

### `roles`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL FK | |
| name | TEXT NOT NULL | |
| scope_bitmask | BIGINT NOT NULL | compact scope rep; see R-14 |
| scope_json | JSONB NOT NULL | forward-compat canonical |
| is_builtin | BOOLEAN NOT NULL DEFAULT FALSE | Owner/Admin/Member/Manager |
| created_at | TIMESTAMPTZ NOT NULL | |

Index: `(organization_id, name)` unique.

### `user_role_assignments`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID NOT NULL FK | |
| role_id | UUID NOT NULL FK | |
| workspace_id | UUID NULL | NULL = org-level role |
| assigned_at | TIMESTAMPTZ NOT NULL | |
| assigned_by | UUID NOT NULL FK users(id) | |

Index: `(user_id, workspace_id)` unique per scope.

### `admin_api_keys`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL FK | |
| prefix | TEXT NOT NULL | `bf-admin-XXXXXXXX` |
| key_hash | TEXT NOT NULL 🔸 | argon2id |
| name | TEXT NOT NULL | |
| scope_bitmask | BIGINT NOT NULL | |
| scope_json | JSONB NOT NULL | |
| status | ENUM(active, revoking, revoked) | |
| revoking_until | TIMESTAMPTZ | grace period for rotation |
| created_by | UUID NOT NULL FK | |
| created_at | TIMESTAMPTZ NOT NULL | |
| expires_at | TIMESTAMPTZ NOT NULL | |
| last_used_at | TIMESTAMPTZ | |

### `service_account_api_keys`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL FK | |
| workspace_id | UUID NOT NULL FK | |
| prefix | TEXT NOT NULL | `bf-svc-XXXXXXXX` |
| key_hash | TEXT NOT NULL 🔸 | |
| name | TEXT NOT NULL | |
| scope_json | JSONB NOT NULL | workspace-scoped subset of admin scopes |
| budget_config_id | UUID FK → virtual_key_budgets(id) | optional |
| rate_limit_config_id | UUID FK → virtual_key_rate_limits(id) | optional |
| status | ENUM(active, revoked) | |
| created_at | TIMESTAMPTZ NOT NULL | |
| expires_at | TIMESTAMPTZ | optional |

### `sso_configs`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL FK | |
| provider | ENUM(saml, oidc) | |
| issuer_url | TEXT | OIDC |
| client_id | TEXT | OIDC |
| client_secret | TEXT 🔐 | OIDC |
| metadata_xml | TEXT 🔐 | SAML |
| signing_certificate | TEXT 🔐 | SAML |
| group_role_map | JSONB | idp-group → bifrost-role |
| default_role_id | UUID FK | assigned on first login |
| auto_accept_invites | BOOLEAN NOT NULL DEFAULT TRUE | |
| is_required | BOOLEAN NOT NULL DEFAULT FALSE | |

### `scim_configs`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL FK | |
| bearer_token_hash | TEXT NOT NULL 🔸 | |
| bearer_token_prefix | TEXT NOT NULL | display |
| enabled | BOOLEAN NOT NULL | |
| last_sync_at | TIMESTAMPTZ | |

### Lifecycle: User

```
pending ─ SSO first login or SCIM create ─▶ active
active ─ SCIM suspend / admin suspend ─▶ suspended
suspended ─ SCIM reactivate / admin reactivate ─▶ active
any ─ SCIM delete ─▶ (hard delete, with audit; keys revoked)
```

---

## 2. Virtual Keys, Budgets & Rate Limits

### `virtual_keys` (EXTENDED — existing table gains tenancy columns)

| Added Column | Type | Notes |
|--------------|------|-------|
| organization_id | UUID NOT NULL FK | migration backfill: DEFAULT_ORG |
| workspace_id | UUID NOT NULL FK | migration backfill: DEFAULT_WORKSPACE |

Existing columns (id, prefix, key_hash, etc.) unchanged in shape.

### `virtual_key_budgets`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| virtual_key_id | UUID NOT NULL FK | |
| organization_id | UUID NOT NULL FK | |
| workspace_id | UUID NOT NULL FK | |
| period | ENUM(monthly, weekly, custom) | |
| period_days | INT | when custom |
| cost_cap_usd_cents | BIGINT | either this or token_cap_* must be set |
| token_cap_input | BIGINT | |
| token_cap_output | BIGINT | |
| alert_thresholds_pct | INT[] NOT NULL DEFAULT '{50,75,90}' | |
| alert_destinations_id | UUID FK → alert_destinations(id) | |
| current_period_start | TIMESTAMPTZ NOT NULL | |
| current_spend_usd_cents | BIGINT NOT NULL DEFAULT 0 | |

### `virtual_key_rate_limits`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| virtual_key_id | UUID NOT NULL FK | |
| window | ENUM(minute, hour, day) | |
| request_limit | BIGINT | |
| token_limit_input | BIGINT | |
| token_limit_output | BIGINT | |

Index: `(virtual_key_id, window)` unique.

A virtual key may have multiple rate_limits rows (one per window).

### Counter store (Redis or PostgreSQL hot table)

Not a configstore table — R-13. Keys:
`rl:{vk_id}:{window}:{bucket_index}` for rate-limit buckets;
`budget:{vk_id}:{period_start}` for spend.

---

## 3. Guardrails

### `guardrail_policies`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL FK | |
| workspace_id | UUID | NULL = org-wide |
| virtual_key_id | UUID | NULL = not key-scoped |
| name | TEXT NOT NULL | |
| type | ENUM(deterministic, llm_based, partner, custom_webhook) | |
| subtype | TEXT | regex / json_schema / prompt_injection / aporia / etc. |
| config_json | JSONB 🔐 | guardrail-specific config (patterns, endpoint, secret) |
| applies_to | ENUM(input, output, both) | |
| execution_mode | ENUM(sync_parallel, sync_sequential, async) | |
| composition_policy | ENUM(any_deny, all_pass, majority) | only at scope level |
| action_on_fail | ENUM(deny, warn, retry, fallback, log_only) | |
| fallback_target_json | JSONB | when action=fallback |
| on_timeout_policy | ENUM(fail_open, fail_closed) | |
| timeout_ms | INT NOT NULL DEFAULT 500 | |
| enabled | BOOLEAN NOT NULL DEFAULT TRUE | |
| execution_order | INT NOT NULL DEFAULT 0 | within scope |

Index: `(organization_id, workspace_id, execution_order)`.

### `guardrail_events` (logstore)

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID | |
| virtual_key_id | UUID | |
| request_id | UUID NOT NULL | |
| policy_id | UUID NOT NULL | |
| phase | ENUM(input, output) | |
| outcome | ENUM(allow, deny, warn, retry, fallback, error) | |
| reason | TEXT | |
| latency_ms | INT | |
| metadata_json | JSONB | |
| created_at | TIMESTAMPTZ NOT NULL | |

Index: `(organization_id, created_at DESC)`; `(request_id)`.

---

## 4. Prompt Library

### `prompts`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID NOT NULL | |
| folder_path | TEXT NOT NULL DEFAULT '/' | |
| name | TEXT NOT NULL | |
| production_version_id | UUID FK → prompt_versions(id) | |
| metadata_json | JSONB | |
| created_at | TIMESTAMPTZ NOT NULL | |
| updated_at | TIMESTAMPTZ NOT NULL | |

Index: `(organization_id, workspace_id, folder_path, name)` unique.

### `prompt_versions`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| prompt_id | UUID NOT NULL FK | |
| version_number | INT NOT NULL | monotonic per prompt |
| content_json | JSONB NOT NULL 🔐 | multimodal: text + image refs |
| variables_schema_json | JSONB | JSON schema |
| partials_refs | UUID[] | → prompt_partials(id) |
| created_by | UUID NOT NULL | |
| created_at | TIMESTAMPTZ NOT NULL | |
| commit_message | TEXT | |

Index: `(prompt_id, version_number)` unique.

### `prompt_partials`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID NOT NULL | |
| name | TEXT NOT NULL | |
| content_json | JSONB NOT NULL 🔐 | |
| updated_at | TIMESTAMPTZ NOT NULL | |

### `prompt_folder_scopes`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID NOT NULL | |
| folder_path | TEXT NOT NULL | |
| role_id | UUID NOT NULL | |
| permissions | JSONB NOT NULL | read / write / delete |

---

## 5. Declarative Configs & Canary

### `configs`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID NOT NULL | |
| name | TEXT NOT NULL | |
| production_version_id | UUID FK → config_versions(id) | |
| updated_at | TIMESTAMPTZ NOT NULL | |

### `config_versions`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| config_id | UUID NOT NULL FK | |
| version_number | INT NOT NULL | |
| document_json | JSONB NOT NULL | canonical Config JSON incl. canary |
| created_by | UUID NOT NULL | |
| created_at | TIMESTAMPTZ NOT NULL | |

Canary report is computed live from logstore; no separate table.

---

## 6. Alerts, Exports, Retention

### `alert_rules`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID | NULL = org-scope |
| name | TEXT NOT NULL | |
| metric | ENUM(error_rate, latency_p50, latency_p95, latency_p99, cost_per_hour, feedback_score, budget_threshold, guardrail_block_rate) | |
| operator | ENUM(gt, gte, lt, lte, eq) | |
| threshold | DOUBLE PRECISION NOT NULL | |
| window_seconds | INT NOT NULL | |
| cooldown_seconds | INT NOT NULL DEFAULT 300 | |
| destinations | UUID[] FK → alert_destinations(id) | |
| enabled | BOOLEAN NOT NULL DEFAULT TRUE | |

### `alert_destinations`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| type | ENUM(webhook, slack, email) | |
| config_json | JSONB 🔐 | URL + secret for webhook; channel+token for slack |

### `alert_events` (logstore)

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| rule_id | UUID NOT NULL | |
| severity | ENUM(info, warning, critical) | |
| observed_value | DOUBLE PRECISION NOT NULL | |
| threshold | DOUBLE PRECISION NOT NULL | |
| fired_at | TIMESTAMPTZ NOT NULL | |
| resolved_at | TIMESTAMPTZ | |

### `log_export_configs`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID | NULL = org-wide |
| destination_type | ENUM(s3, azure_blob, gcs, mongodb, otlp) | |
| destination_config_json | JSONB 🔐 | bucket/ARN/connstring/endpoint + creds |
| mode | ENUM(streaming, scheduled) | |
| stream_interval_seconds | INT | streaming |
| cron_expression | TEXT | scheduled |
| partition_scheme | TEXT | e.g., `year=YYYY/month=MM/day=DD/hour=HH` |
| record_schema_json | JSONB | subset of log columns |
| enabled | BOOLEAN NOT NULL DEFAULT TRUE | |
| last_success_at | TIMESTAMPTZ | |
| last_error | TEXT | |

### `export_dead_letters` (logstore)

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| export_config_id | UUID NOT NULL | |
| record_json | JSONB NOT NULL | |
| attempts | INT NOT NULL | |
| last_error | TEXT | |
| created_at | TIMESTAMPTZ NOT NULL | |

### `retention_policies`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID NOT NULL | |
| log_retention_days | INT NOT NULL | |
| metric_retention_days | INT NOT NULL | |
| applies_at | TIMESTAMPTZ NOT NULL | when policy became effective |

### `data_lake_exports` (US23)

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID | |
| name | TEXT NOT NULL | |
| source_entity | ENUM(logs, metrics, audit) | |
| column_schema_json | JSONB NOT NULL | |
| filter_json | JSONB | SQL-safe WHERE subset |
| format | ENUM(parquet, avro, jsonl) | |
| destination_config_json | JSONB 🔐 | s3/gcs/bigquery/wasabi |
| cron_expression | TEXT NOT NULL | |
| partition_scheme | TEXT | |
| enabled | BOOLEAN NOT NULL DEFAULT TRUE | |
| last_success_at | TIMESTAMPTZ | |

---

## 7. Audit & Observability

### `audit_entries` (logstore)

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| workspace_id | UUID | NULL = org-level action |
| actor_type | ENUM(user, admin_api_key, service_account, system) | |
| actor_id | UUID | |
| actor_display | TEXT | email or key prefix |
| actor_ip | INET | |
| action | TEXT NOT NULL | canonical verb: `virtual_key.create`, `role.assign`, ... |
| resource_type | TEXT NOT NULL | |
| resource_id | UUID | |
| outcome | ENUM(allowed, denied, error) | |
| reason | TEXT | |
| before_json | JSONB 🔐 (if PII) | prior state |
| after_json | JSONB 🔐 (if PII) | new state |
| request_id | UUID | |
| created_at | TIMESTAMPTZ NOT NULL | |

Indexes: `(organization_id, created_at DESC)`, `(actor_id)`, `(resource_type, resource_id)`.

### `executive_metrics_hourly` (logstore rollup)

| Column | Type | Notes |
|--------|------|-------|
| organization_id | UUID NOT NULL | |
| hour_bucket | TIMESTAMPTZ NOT NULL | |
| metric_name | TEXT NOT NULL | |
| dimension_key | TEXT | e.g., workspace_id or model |
| value | DOUBLE PRECISION NOT NULL | |
| PRIMARY KEY | (organization_id, hour_bucket, metric_name, dimension_key) | |

Partitioned by month.

---

## 7.5 Licensing & Cloud Billing (Train E)

### `licenses` (configstore — selfhosted / airgapped only)

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| raw_jwt | TEXT NOT NULL 🔐 | signed license file content |
| customer_name | TEXT NOT NULL | parsed from `sub` claim |
| issued_at | TIMESTAMPTZ NOT NULL | |
| expires_at | TIMESTAMPTZ NOT NULL | |
| grace_period_days | INT NOT NULL DEFAULT 14 | |
| entitlements_json | JSONB NOT NULL | full claim subtree |
| limits_json | JSONB NOT NULL | max_workspaces/users/keys |
| contact_email | TEXT | |
| uploaded_at | TIMESTAMPTZ NOT NULL | |
| uploaded_by | UUID FK → users(id) | |

Only one row active at a time per deployment; replacement rows are
retained for audit.

### `billing_accounts` (configstore — cloud only)

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL FK | |
| stripe_customer_id | TEXT NOT NULL | |
| stripe_subscription_id | TEXT | null for Dev tier |
| tier | ENUM(dev, prod, enterprise) | |
| billing_cycle_start | TIMESTAMPTZ NOT NULL | |
| billing_cycle_end | TIMESTAMPTZ NOT NULL | |
| dunning | BOOLEAN NOT NULL DEFAULT FALSE | |
| dunning_until | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ NOT NULL | |
| updated_at | TIMESTAMPTZ NOT NULL | |

### `meter_daily` (logstore — cloud only)

| Column | Type | Notes |
|--------|------|-------|
| organization_id | UUID NOT NULL | |
| day_bucket | DATE NOT NULL | |
| requests | BIGINT NOT NULL | |
| input_tokens | BIGINT NOT NULL | |
| output_tokens | BIGINT NOT NULL | |
| cost_usd_micros | BIGINT NOT NULL | cost × 1e6 to avoid FP |
| cache_hits | BIGINT NOT NULL | |
| PRIMARY KEY | (organization_id, day_bucket) | |

Partitioned by month.

### `meter_monthly` (logstore — cloud only)

| Column | Type | Notes |
|--------|------|-------|
| organization_id | UUID NOT NULL | |
| month_bucket | DATE NOT NULL | first day of month |
| requests | BIGINT NOT NULL | |
| input_tokens | BIGINT NOT NULL | |
| output_tokens | BIGINT NOT NULL | |
| cost_usd_micros | BIGINT NOT NULL | |
| billed_at | TIMESTAMPTZ | set when usage pushed to Stripe |
| stripe_invoice_id | TEXT | |
| PRIMARY KEY | (organization_id, month_bucket) | |

### `stripe_webhook_events` (logstore — cloud only, idempotency log)

| Column | Type | Notes |
|--------|------|-------|
| event_id | TEXT PK | Stripe `evt_*` id (idempotency key) |
| event_type | TEXT NOT NULL | |
| payload_json | JSONB NOT NULL | |
| received_at | TIMESTAMPTZ NOT NULL | |
| processed_at | TIMESTAMPTZ | |
| outcome | ENUM(success, error, skipped_duplicate) | |

---

## 8. KMS / BYOK

### `kms_configs`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| organization_id | UUID NOT NULL | |
| provider | ENUM(aws_kms, azure_key_vault, gcp_kms, configstore_key) | |
| key_ref | TEXT NOT NULL 🔐 | ARN / keyvault URI / resource name |
| auth_json | JSONB 🔐 | IAM role ARN / managed-identity / service-account |
| dek_cache_ttl_seconds | INT NOT NULL DEFAULT 900 | |
| applies_to | JSONB NOT NULL | list of `{entity, enabled}` rows |
| enabled | BOOLEAN NOT NULL DEFAULT TRUE | |

### `encrypted_record_metadata` (sidecar per encrypted record)

Stored inline in the ciphertext layout (R-05):

```
version(1B) | kek_ref_hash(32B) | dek_wrapped(N) | nonce(12B) | ct+tag(N)
```

No separate table; ciphertext carries its own envelope.

---

## 9. Existing v1.5.2 Tables — Sidecar Tenancy Plan

Upstream GORM struct files in `framework/configstore/tables/` and
`framework/logstore/tables.go` MUST NOT be modified (Principle XI
rule 1). Tenancy is attached via 1:1 sidecar tables defined in
`framework/<store>/tables-enterprise/`:

| Upstream table | Sidecar table | Sidecar PK | Sidecar tenancy fields |
|----------------|--------------|-----------|------------------------|
| `governance_virtual_keys` (TableVirtualKey) | `ent_virtual_key_tenancy` | `virtual_key_id` (FK) | `organization_id`, `workspace_id` |
| `governance_teams` (TableTeam) | `ent_team_tenancy` | `team_id` (FK) | `organization_id`, `workspace_id` |
| `governance_customers` (TableCustomer) | `ent_customer_tenancy` | `customer_id` (FK) | `organization_id`, `workspace_id` |
| `governance_providers` (TableProvider) | `ent_provider_tenancy` | `provider_id` (FK) | `organization_id` (workspace optional) |
| `governance_keys` (TableKey) | `ent_provider_key_tenancy` | `key_id` (FK) | `organization_id` |
| `bifrost_logs` (logstore) | `ent_log_tenancy` | `log_id` (FK) | `organization_id`, `workspace_id` |

Sidecar reads use a `LEFT JOIN` so upstream rows without sidecar
attribution fall into the synthetic default organization (which has
a sidecar row written at first boot by `E001_seed_default_org`).

Sidecar writes happen alongside upstream writes: when the
enterprise-gate plugin's `HTTPTransportPreHook` resolves the
tenant context for an incoming request and that request creates a
new virtual key, the create operation writes to BOTH
`governance_virtual_keys` (via existing governance plugin) AND
`ent_virtual_key_tenancy` (via the enterprise-gate's post-hook).

**Upstreaming bias** (Principle XI rule 7): if upstream accepts a
PR adding `organization_id`/`workspace_id` columns to the
respective upstream `TableX` structs, the sidecar collapses into
the main struct in a v1.7+ release with a one-time
`E0XX_collapse_<table>_tenancy` migration that copies sidecar
values into the upstream columns and drops the sidecar table.

Non-affected upstream tables (intrinsically global, no tenancy
needed): `bifrost_config_hash`, `bifrost_distributed_lock`, model
catalog master rows, MCP catalog master rows.

---

## 10. Relationships Map

```
organization 1—N workspace
workspace    1—N virtual_key (via existing table)
workspace    1—N service_account_api_key
organization 1—N admin_api_key
organization 1—N user  N—N role (via user_role_assignments)
workspace    1—N prompt 1—N prompt_version
workspace    1—N prompt_partial
workspace    1—N config 1—N config_version
org|workspace 1—N guardrail_policy
org|workspace 1—N alert_rule
org|workspace 1—N log_export_config
workspace    1—1 retention_policy
organization 1—N kms_config
organization 1—1 sso_config
organization 1—1 scim_config
```

Every row in every tenant-scoped table points back to an organization;
a tenant's entire data set can be located (for DSR export, for
deletion, for migration to a different instance) by a single
`organization_id` filter.

Cloud-only tables (`billing_accounts`, `meter_daily`,
`meter_monthly`, `stripe_webhook_events`) carry `organization_id`
as well and follow the same per-tenant scoping rules; they simply
do not populate in `selfhosted` / `airgapped` deployments.

Self-hosted-only tables (`licenses`) are deployment-global and
carry no `organization_id` — the license covers the entire
single-org deployment.
