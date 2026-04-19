# Implementation Plan: Bifrost Enterprise Parity

**Branch**: `001-enterprise-parity` | **Date**: 2026-04-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/001-enterprise-parity/spec.md`

## Summary

Deliver Portkey-equivalent enterprise capabilities (organizations/workspaces,
granular RBAC, SSO/SCIM, audit logs, admin API keys, central guardrails, PII
redaction, granular budgets/rate limits, custom guardrail webhooks, alerts,
log export, executive dashboard, per-workspace retention, prompt library +
playground, declarative configs, service-account keys, BYOK, air-gapped
deployment, SCIM, Terraform provider, canary routing, curated data-lake
export) across releases `v1.6.0` → `v2.0.0` (Trains A–E), with `v1.6.0`
byte-identical to `v1.5.2` for an unchanged OSS config. v1.6.0 → v1.9.0
are MINOR bumps (Trains A–D) preserving SemVer; v2.0.0 is a MAJOR bump
introducing the cloud commercial layer (Train E) but remains backward-
compatible for self-hosted deployments.

**Technical approach (high level)**:

- Zero changes under `core/**`. Every capability lives in new plugins under
  `plugins/*/`, new framework subsystems under `framework/*/`, new handlers
  under `transports/bifrost-http/handlers/`, new UI pages under `ui/`, and
  new Helm/Terraform assets.
- Tenancy is built once as a framework primitive (`framework/tenancy/`)
  consumed by every feature's persistence and request-scoping logic.
- A new `plugins/enterprise-gate/` plugin resolves tenant context on the
  HTTP pre-hook path and seeds `BifrostContext` keys (`organization_id`,
  `workspace_id`, `role_scopes`) so downstream plugins read them without
  coupling to each other.
- Persistence extends the existing `framework/configstore/` and
  `framework/logstore/` schemas with `organization_id` + `workspace_id`
  columns everywhere; existing single-tenant rows are migrated into a
  synthetic default organization on first boot.
- Audit, guardrails, PII redaction, budgets, alerts, prompts, configs,
  canary, and export each ship as an independent plugin module so they can
  version and deploy separately per Constitution Principle X.

## Technical Context

**Language/Version**: Go 1.26.1+ (multi-module workspace), React 18 +
Vite + TypeScript 5.x (UI), Mintlify MDX (docs), HCL 2 (Terraform).

**Primary Dependencies**:

- Request path: `fasthttp` (transports), `sonic` (hot-path JSON),
  `atomic.Pointer` (hot-reload), `valyala/fastjson` (already in use).
- Storage: `pgx/v5` (PostgreSQL), existing `framework/configstore` and
  `framework/logstore` interfaces, `weaviate-go-client` /
  `qdrant-go-client` / `go-redis` via existing `framework/vectorstore`.
- Auth: `crewjam/saml` (SAML 2.0), `coreos/go-oidc/v3` (OIDC), `scim/v2`
  (SCIM 2.0).
- Crypto / KMS: `aws-sdk-go-v2/service/kms`, `azsdk/azkeys`, GCP
  `cloud.google.com/go/kms`.
- Observability: existing `plugins/otel` + `plugins/telemetry`,
  `prometheus/client_golang`.
- Export: `aws-sdk-go-v2/service/s3`, `mongo-go-driver`, OTLP collector
  via OpenTelemetry SDK.
- Guardrail partners: SDK or REST clients for Aporia, Pillar, Patronus,
  SydeLabs, Pangea.
- UI: `react-router-dom`, `@tanstack/react-query`, `tailwindcss`,
  existing design system; `@playwright/test` for E2E.
- Terraform: `hashicorp/terraform-plugin-framework`.

**Storage**:

- `configstore` (file or PostgreSQL, existing): organizations, workspaces,
  users, roles, scopes, virtual keys, service-account keys, admin API
  keys, guardrail configs, config objects, prompt library (v1), SSO/SCIM
  configs, alert rules, log-export configs, retention policies,
  KMS/BYOK configs.
- `logstore` (file or PostgreSQL, existing): request/response logs,
  audit entries, guardrail events.
- `vectorstore` (existing): unchanged — consumed by the existing
  semantic-cache plugin.

**Testing**:

- Go unit tests (`go test ./...`) per module; plugin tests under
  `plugins/<name>/*_test.go`.
- Integration tests: run against real PostgreSQL (configstore +
  logstore) and real vectorstore; wired through existing
  `make test-plugins` and a new `make test-enterprise` target.
- E2E: `@playwright/test` under `ui/tests/e2e/` using
  `data-testid="<entity>-<element>-<qualifier>"` convention.
- Load / perf regression: existing `make perf` benchmark + new
  enterprise-feature overhead suite.
- Contract tests: OpenAPI-generated conformance tests for the new Admin
  API endpoints.

**Target Platform**: Linux/amd64 and Linux/arm64 container runtimes,
deployed via the published Helm chart (`helm-charts/bifrost/`) with a
new `profile: airgapped` values preset, and Terraform modules under
`terraform/modules/{aws,azure,gcp}/`.

**Project Type**: Web service + UI + IaC assets (existing Bifrost
shape; no new top-level project added).

**Performance Goals**:

- ≤1ms p50 / ≤3ms p99 added latency per enabled enterprise feature on
  the request hot path (SC-005).
- Sustained 5k RPS reference load with full P1+P2 feature stack
  enabled.
- Log export: ≥99.9% delivery under 1k RPS with target unreachable up
  to 10 minutes (SC-010).
- Executive dashboard first-paint P95 <3s at 10M log/month org scale
  (SC-002, US12).
- Audit log filtered-query first-page <2s at 10M entries (SC-011).

**Constraints**:

- **Non-negotiable**: zero modifications under `core/**` (Constitution
  Principle I) — enforced by CI diff.
- Non-breaking v1.5.2 → v1.6.0 upgrade (SC-001); existing configs load
  unchanged; default behavior is byte-identical.
- All features config-gated (Principle IV); no build tags.
- Multi-tenancy from day one in every new table (Principle V).
- Every feature emits OTEL + Prometheus + audit (Principle VI).

**Scale/Scope**:

- v1 deployment: single organization, N workspaces, up to 10M
  log/month, up to 1,000 virtual keys, up to 100k prompts +
  versions.
- Target usage pattern: ~20 enterprise deployments in first year;
  customers typically between 50 and 5,000 monthly active users.
- UI: ~30 new top-level pages/modals across all user stories.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Asserted compliance with each principle in `.specify/memory/constitution.md`.

| # | Principle | Status | Evidence / Notes |
|---|-----------|--------|------------------|
| I | Core Immutability — no edits under `core/**` | PASS | All work lives under `plugins/*`, `framework/*`, `transports/bifrost-http/{handlers,lib}`, `ui/`, `helm-charts/`, `terraform/`, `docs/enterprise/`. CI check (see research.md R-01) diffs `core/` against the v1.5.2 tag baseline and fails the build on any delta. |
| II | Non-Breaking — optional fields, stable hook signatures | PASS | All new `config.schema.json` fields are optional with v1.5.2-preserving defaults. No plugin hook signatures modified; new capability flows through new plugins or new `BifrostContext` keys. FR-039–FR-041 codify this; SC-001 validates via golden-set replay. |
| III | Plugin-First — feature lives under `plugins/<name>/` or `framework/` | PASS | Each feature group maps to a discrete plugin module (see Project Structure below); cross-cutting concerns (tenant resolution, request-ID) go into `transports/bifrost-http/lib/middleware.go`. |
| IV | Config-Driven Gating — no build tags, schema conditionals only | PASS | Every feature has a config-level `enabled: false` default. JSON Schema `if`/`then` conditionals constrain dependent fields only when the feature is enabled. No `//go:build enterprise` tags anywhere. |
| V | Multi-Tenancy First — `workspace_id` / `organization_id` present | PASS | `framework/tenancy/` defines `TenantContext`; every new table carries `organization_id` + `workspace_id` columns indexed jointly. Request scoping resolves tenant at the transport middleware layer (see data-model.md). |
| VI | Observability — OTEL + Prometheus + audit covered | PASS | Each new plugin registers (a) OTEL span via `otel.GetTracerProvider()`, (b) counter + histogram via the existing telemetry registry, (c) audit entries via the `plugins/audit/` sink. Validation is a CI completeness test (research.md R-09). |
| VII | Security by Default — secrets encrypted, TLS, redaction hooked | PASS | `framework/crypto/` unifies at-rest encryption (configstore encryption_key OR BYOK data keys). All external integrations validate TLS 1.2+ at config-load. Logstore write path invokes the PII redactor before persist when enabled. Admin/service/virtual keys hash at rest (argon2id) with prefix-only display. |
| VIII | Test Coverage — real dependencies, Playwright for UI | PASS | Integration tests run against real PostgreSQL and real vectorstore via docker-compose; Playwright E2E covers every new UI page under `data-testid` conventions. No mocks for DB or provider APIs in integration tier. |
| IX | Docs & Schema Sync — `config.schema.json` + MDX + changelog | PASS | Every task in tasks.md that ships a capability bundles (a) schema update, (b) `docs/enterprise/<feature>.mdx`, (c) `changelog.md` entry in affected modules (core→framework→plugins→transport), (d) UI strings. Non-compliant PRs blocked by CI. |
| X | Dependency Hierarchy — no reverse imports; plugin modules independent | PASS | Each new plugin has its own `go.mod`; cross-plugin data flows through `BifrostContext` keys or framework services only. Dependency-direction is verified by `make check-imports` (research.md R-10). |

**No violations requiring Complexity Tracking.**

## Project Structure

### Documentation (this feature)

```text
specs/001-enterprise-parity/
├── plan.md                 # this file
├── spec.md                 # feature specification (with Clarifications)
├── research.md             # Phase 0 output
├── data-model.md           # Phase 1 output
├── quickstart.md           # Phase 1 output — operator onboarding
├── contracts/              # Phase 1 output — API + event contracts
│   ├── admin-api.openapi.yaml
│   ├── events.md
│   └── webhook-payloads.md
├── checklists/
│   └── requirements.md     # from /speckit-specify
└── tasks.md                # created by /speckit-tasks
```

### Source Code (repository root)

```text
# UNCHANGED (Constitution Principle I — core is immutable)
core/                       # UNTOUCHED for this feature
core/schemas/               # UNTOUCHED
core/providers/             # UNTOUCHED
core/mcp/                   # UNTOUCHED

# FRAMEWORK ADDITIONS (framework is a dependency of plugins/transports)
framework/
├── configstore/            # EXTENDED via sibling files (no upstream edits)
│   ├── tables-enterprise/  # NEW — sidecar + new GORM struct tables
│   │   ├── system_defaults.go
│   │   ├── organization.go
│   │   ├── workspace.go
│   │   ├── user.go
│   │   ├── role.go
│   │   ├── user_role_assignment.go
│   │   ├── admin_api_key.go
│   │   ├── service_account_api_key.go
│   │   ├── sso_config.go
│   │   ├── scim_config.go
│   │   ├── virtual_key_tenancy.go    # 1:1 sidecar
│   │   ├── team_tenancy.go           # 1:1 sidecar
│   │   ├── customer_tenancy.go       # 1:1 sidecar
│   │   ├── provider_tenancy.go       # 1:1 sidecar
│   │   ├── provider_key_tenancy.go   # 1:1 sidecar
│   │   ├── guardrail_policy.go
│   │   ├── vk_budget.go
│   │   ├── vk_rate_limit.go
│   │   ├── alert_rule.go
│   │   ├── alert_destination.go
│   │   ├── log_export_config.go
│   │   ├── retention_policy.go
│   │   ├── prompt.go / prompt_version.go / prompt_partial.go / prompt_folder_scope.go
│   │   ├── config.go / config_version.go
│   │   ├── kms_config.go
│   │   ├── data_lake_export.go
│   │   ├── license.go
│   │   └── billing_account.go
│   └── migrations_enterprise.go   # NEW — RegisterEnterpriseMigrations(db)
│                                    #   with E001..E024 *migrator.Migration
├── logstore/               # EXTENDED via sibling files (no upstream edits)
│   ├── tables_enterprise.go        # NEW — TableLogTenancy (sidecar),
│   │                                #   TableAuditEntry, TableGuardrailEvent,
│   │                                #   TableAlertEvent, TableExportDeadLetter,
│   │                                #   TableExecutiveMetricsHourly,
│   │                                #   TableMeterDaily, TableMeterMonthly,
│   │                                #   TableStripeWebhookEvent
│   └── migrations_enterprise.go    # NEW — RegisterEnterpriseMigrations(db)
│                                    #   with E003/E006/E009/E012/E014/E015/E023/E025
├── tenancy/                # NEW — TenantContext resolver + repository helpers
├── crypto/                 # NEW — unified encryption (configstore key + BYOK)
├── kms/                    # NEW — AWS/Azure/GCP KMS adapters + data-key cache
├── idp/                    # NEW — SAML 2.0 + OIDC + SCIM 2.0 infrastructure
├── scim/                   # NEW — SCIM 2.0 server handlers
├── guardrails/             # NEW — guardrail execution engine (shared runtime)
├── redaction/              # NEW — PII detection + redaction rules
├── exportsink/             # NEW — streaming/scheduled export framework
├── alerts/                 # NEW — alert rule evaluator + dispatcher
└── promptengine/           # NEW — Mustache render + partials + versioning

# PLUGIN ADDITIONS (each an independent go.mod; versioned separately)
plugins/
├── governance/             # EXTENDED — tenant-aware budgets, threshold alerts
├── logging/                # EXTENDED — PII hook call, tenant-scoped writes
├── telemetry/              # EXTENDED — tenant labels on metrics
├── otel/                   # EXTENDED — tenant attributes on spans
├── semanticcache/          # EXTENDED — workspace-scoped cache keys
├── prompts/                # EXTENDED — wire to new promptengine + library API
├── enterprise-gate/        # NEW — tenant resolution at HTTP pre-hook
├── audit/                  # NEW — audit log sink (ObservabilityPlugin)
├── guardrails-central/     # NEW — org-wide guardrail enforcement
├── guardrails-partners/    # NEW — Aporia/Pillar/Patronus/SydeLabs/Pangea
├── guardrails-webhook/     # NEW — custom HTTPS webhook guardrail
├── pii-redactor/           # NEW — PII redaction on request/response + logstore hook
├── alerts/                 # NEW — alert rule eval + webhook/Slack dispatch
├── logexport/              # NEW — S3/Azure/GCS/Mongo/OTLP sinks
├── canary/                 # NEW — Config-level canary routing primitive
├── byok/                   # NEW — BYOK envelope encryption + KMS adapters
└── sso/                    # NEW — SAML 2.0 + OIDC auth flows

# TRANSPORT ADDITIONS (handlers + middleware, same transports module)
transports/
├── bifrost-http/
│   ├── handlers/
│   │   ├── organizations.go       # NEW
│   │   ├── workspaces.go          # NEW
│   │   ├── users.go               # NEW
│   │   ├── roles.go               # NEW
│   │   ├── admin_api_keys.go      # NEW
│   │   ├── service_account_keys.go# NEW
│   │   ├── sso.go                 # NEW (SAML + OIDC callbacks)
│   │   ├── scim.go                # NEW (SCIM 2.0 endpoints)
│   │   ├── audit_logs.go          # NEW
│   │   ├── guardrails_central.go  # NEW
│   │   ├── alerts.go              # NEW
│   │   ├── log_exports.go         # NEW
│   │   ├── retention_policies.go  # NEW
│   │   ├── prompt_library.go      # NEW
│   │   ├── prompt_playground.go   # NEW
│   │   ├── configs.go             # NEW
│   │   ├── exec_dashboard.go      # NEW
│   │   └── kms.go                 # NEW
│   ├── lib/
│   │   └── middleware.go          # EXTENDED — tenant-resolve, RBAC-enforce
│   └── config.schema.json         # EXTENDED — new fields + conditionals

# UI ADDITIONS (React pages + components; translations)
ui/
├── src/
│   ├── pages/
│   │   ├── OrgSettings/*.tsx
│   │   ├── Workspaces/*.tsx
│   │   ├── Users/*.tsx
│   │   ├── Roles/*.tsx
│   │   ├── SSOConfig/*.tsx
│   │   ├── AdminApiKeys/*.tsx
│   │   ├── AuditLogs/*.tsx
│   │   ├── Guardrails/*.tsx
│   │   ├── Alerts/*.tsx
│   │   ├── LogExports/*.tsx
│   │   ├── Retention/*.tsx
│   │   ├── PromptLibrary/*.tsx
│   │   ├── PromptPlayground/*.tsx
│   │   ├── Configs/*.tsx
│   │   ├── CanaryReport/*.tsx
│   │   ├── ExecutiveDashboard/*.tsx
│   │   └── KmsSettings/*.tsx
│   └── i18n/
└── tests/e2e/*.spec.ts           # Playwright

# DEPLOYMENT ADDITIONS (Helm + Terraform + docs)
helm-charts/
└── bifrost/
    ├── values.yaml                 # EXTENDED
    ├── values-airgapped.yaml       # NEW — air-gapped MVP profile
    └── templates/                  # EXTENDED (new ConfigMaps, Secrets, Jobs)

terraform/
├── providers/bifrost/              # NEW — Terraform provider
└── modules/
    ├── aws/                        # NEW
    ├── azure/                      # NEW
    └── gcp/                        # NEW

docs/
└── enterprise/
    ├── organizations-workspaces.mdx
    ├── rbac.mdx
    ├── sso.mdx
    ├── scim.mdx
    ├── audit-logs.mdx
    ├── admin-api-keys.mdx
    ├── central-guardrails.mdx
    ├── pii-redaction.mdx
    ├── budgets-rate-limits.mdx
    ├── alerts.mdx
    ├── log-export.mdx
    ├── retention-policies.mdx
    ├── executive-dashboard.mdx
    ├── prompt-library.mdx
    ├── prompt-playground.mdx
    ├── configs.mdx
    ├── canary.mdx
    ├── service-account-keys.mdx
    ├── byok.mdx
    ├── airgapped-deployment.mdx
    ├── terraform-provider.mdx
    └── data-lake-export.mdx
```

**Structure Decision**: Extend existing Bifrost monorepo shape. No new
top-level directory is introduced. Every new capability is placed in
the layer appropriate to its dependency level (core → framework →
plugins → transports → ui), strictly respecting Principle X. Plugin
modules remain independent `go.mod`s so they can version separately.

## Deployment Modes

One binary, three operating modes. The `deployment.mode` config field
selects which plugins load and which defaults apply. The code paths
are 100% shared; modes differ only in configuration and plugin
registration.

| Concern | `cloud` | `selfhosted` | `airgapped` |
|---------|---------|--------------|-------------|
| **Multi-org** | enabled (many orgs, one deployment) | disabled (one org) | disabled |
| **License plugin** | not loaded | **required** (signed file verified offline) | **required** |
| **Phone-home telemetry** | enabled (vendor operates it) | disabled by default; opt-in anonymous version ping | **forbidden** (eBPF-verified zero egress to non-whitelisted endpoints) |
| **Metering (US26)** | enabled | not loaded | not loaded |
| **Stripe billing (US27–US29)** | enabled | not loaded | not loaded |
| **SSO** | OIDC + SAML | OIDC + SAML | **OIDC only** (SAML deferred per FR-037) |
| **BYOK (US18)** | optional | optional | optional (main use case) |
| **Feature surface** | all Trains A–E (subject to tenant tier) | all Trains A–D (subject to license entitlements) | P1 + BYOK subset (FR-037) |
| **Update cadence** | continuous (vendor deploys weekly+) | quarterly stable release | quarterly stable release (offline) |
| **Air-gapped Helm profile** | N/A | available as deployment choice | required |
| **Support model** | vendor-owned SLA | support ticket SLA | support ticket SLA |

**Non-goal for v1**: Portkey-style "hybrid" (customer runs data plane,
vendor runs control plane). All three modes are self-contained. A
future `hybrid` mode can be introduced additively as a fourth enum
value if demand materializes.

### Tier Feature Matrix (cloud mode)

Cloud customers subscribe to one of three tiers. Each enterprise
feature gates by the customer's active tier via the billing
plugin's context key. The matrix below is authoritative; the
billing plugin enforces it, and `docs/enterprise/tiers-and-plans.mdx`
is the customer-facing presentation.

| Feature | Dev (free) | Prod ($49/mo) | Enterprise (custom) |
|---------|:----------:|:-------------:|:-------------------:|
| OSS chat/completions/embeddings | ✅ | ✅ | ✅ |
| Rate limit | 5 req/min | 1000 req/min | custom |
| Log retention | 3 days | 30 days | custom |
| Workspaces (US1) | 1 | unlimited | unlimited |
| Users (US2) | 1 | unlimited | unlimited |
| RBAC — custom roles (US2) | — | ✅ | ✅ |
| SSO (US3) | — | — | ✅ |
| Audit logs (US4) | — | — | ✅ |
| Admin API keys (US5) | — | ✅ | ✅ |
| Service account keys (US17) | — | ✅ | ✅ |
| Deterministic guardrails (US6) | — | ✅ | ✅ |
| LLM-based guardrails (US6) | — | — | ✅ |
| Partner guardrails (US6) | — | — | ✅ |
| Custom guardrail webhooks (US9) | — | ✅ | ✅ |
| PII redaction (US7) | — | — | ✅ |
| Budgets + rate limits (US8) | — | ✅ | ✅ |
| Alerts (US10) | — | ✅ | ✅ |
| Log export — S3 only (US11) | — | ✅ | ✅ |
| Log export — all destinations (US11) | — | — | ✅ |
| Executive dashboard (US12) | — | — | ✅ |
| Custom retention (US13) | — | — | ✅ |
| Prompt library (US14) | 3 prompts | unlimited | unlimited |
| Prompt playground (US15) | — | ✅ | ✅ |
| Declarative configs (US16) | — | ✅ | ✅ |
| BYOK (US18) | — | — | ✅ |
| SCIM (US20) | — | — | ✅ |
| Terraform provider (US21) | ✅ | ✅ | ✅ |
| Canary routing (US22) | — | ✅ | ✅ |
| Data lake ETL (US23) | — | — | ✅ |

Self-hosted / air-gapped deployments bypass this matrix entirely;
their feature availability is driven by the license file's
`entitlements` claim (per R-26) rather than by tier.

**Release-channel policy**: `cloud` follows an "edge" channel
(continuous deployment of `main`). `selfhosted` and `airgapped` pin
to a quarterly `stable` channel that receives security patches only
between feature releases. The CI pipeline tags both channels.

## Phased Delivery Strategy

The user stories span six priority tiers (P1–P6). The plan sequences
delivery as four release trains so each train is independently
deployable and independently valuable.

### Train A — Tenancy + Identity (P1)

Scope: US1 (Orgs/Workspaces), US2 (RBAC), US3 (SSO), US4 (Audit),
US5 (Admin API Keys).

Rationale: Every later capability depends on tenant scoping and audit
sink. No later train can ship first without retrofitting tenancy.

Exit: An existing v1.5.2 deployment upgrades in place, auto-migrates
into a default org + default workspace, and a new org admin can
configure SSO and create scoped admin API keys, with every action
appearing in the audit log.

### Train B — Governance Depth (P2)

Scope: US6 (Central Guardrails), US7 (PII Redactor), US8 (Granular
Budgets/Rate Limits), US9 (Custom Guardrail Webhooks).

Rationale: Tenancy (Train A) is a prerequisite because guardrails and
budgets scope by org/workspace/virtual-key.

Exit: Org-wide PII redaction runs before logstore writes; org-wide
guardrail policy catches prompt injection; budget thresholds alert
and rate limits reject 429; every event audited.

### Train C — Observability + DX (P3, P4)

Scope: US10 (Alerts), US11 (Log Export), US12 (Exec Dashboard), US13
(Retention), US14 (Prompt Library), US15 (Playground), US16 (Configs),
US17 (Service Account Keys).

Rationale: Observability and DX depth are differentiators but not
procurement blockers; they follow governance.

Exit: Alerts fire to Slack and webhook; S3/Azure/GCS/Mongo/OTLP export
is continuous; exec dashboard renders for the org; prompt library +
playground ship; declarative Configs reroute traffic.

### Train D — Security + Ecosystem (P5, P6)

Scope: US18 (BYOK), US19 (Air-gapped), US20 (SCIM), US21 (Terraform),
US22 (Canary), US23 (Data Lake ETL).

Rationale: BYOK + air-gapped lift the sales floor to US-federal /
healthcare; SCIM completes the IdP story; Terraform/canary/ETL are
long-tail DX.

Exit: Air-gapped Helm profile passes smoke tests in a network-
restricted cluster; BYOK encrypts configstore and optionally logstore
payloads; Terraform resources achieve apply → no-op idempotency; ETL
exports land curated datasets in customer BigQuery/S3 on schedule.

Each train is independently releasable as a minor version bump (v1.6,
v1.7, v1.8, v1.9) while maintaining backward compatibility. A single
"enterprise enabled" deployment stacks all four; an OSS deployment
runs on v1.6+ unchanged.

### Train E — Cloud Commercial (cloud mode only)

Scope: US24 (License Activation — ships with every self-hosted train
from v1.6 onward, foundational), US25 (License Expiry Handling),
US26 (Per-Org Metering), US27 (Stripe Billing), US28 (Billing
Portal), US29 (Tier & Plan Management).

Rationale: Commercial distribution requires licensing (self-hosted)
and billing automation (cloud). US24 ships with Train A as part of
the enterprise-gate plugin because it's a prerequisite for ANY
self-hosted enterprise feature to activate. US25 follows in Train B.
US26–US29 form their own train that activates ONLY in cloud mode.

Release: Train E (cloud-only portion) targets `v2.0.0`, a MAJOR
bump because it introduces the cloud commercial layer as a
user-visible product surface. v2.0.0 remains backward-compatible
for self-hosted deployments — they don't load Train E plugins.

Exit: A new cloud customer signs up on the vendor's cloud, enters
a payment method, is billed monthly with a correct invoice,
upgrades Dev → Prod self-service, and sees current-cycle usage in
the billing portal. A self-hosted customer upgrading from v1.9 to
v2.0 observes no behavior change (Train E plugins do not load in
`selfhosted` / `airgapped` mode).

## Complexity Tracking

> Filled only if Constitution Check has violations that must be justified.

**None.** Every principle passes without requiring an exception.

The plan preserves `core/**` untouched (CI-enforced baseline diff),
uses the plugin system and framework-extension patterns already in
Bifrost, gates all features behind config, builds multi-tenancy in
from first commit in every new table, emits all three telemetry
signals per feature, routes secrets through a single unified
encryption layer, mandates integration tests on real dependencies,
binds docs + schema + changelog to every feature-delivering PR, and
keeps plugin modules independently versioned.

The only judgment-call deviation is the deliberate deferral (via
Clarifications Q2) of multi-org activation from v1 even though the
schema carries it. This is an informed simplification for MVP
scope, not a constitutional violation — every new table still has
`organization_id`; the flag gates runtime behavior only.

## Design Phase References

- Phase 0 research: [research.md](./research.md) — tenant-isolation
  enforcement pattern, guardrail composition order, BYOK envelope
  scheme, air-gapped feature matrix verification, PII benchmark
  selection, export sink reliability pattern, alert dispatcher
  backpressure, hot-reload pattern for tenancy, migration rollback
  plan, import-direction enforcement, observability-completeness
  test.
- Phase 1 data model: [data-model.md](./data-model.md) — every entity
  from the spec mapped to configstore/logstore tables with columns,
  indexes, foreign keys, validation rules, and transitions.
- Phase 1 contracts: [contracts/](./contracts/) — Admin API OpenAPI,
  event schemas for audit/guardrail/alert, webhook payload shapes,
  SCIM conformance notes.
- Phase 1 quickstart: [quickstart.md](./quickstart.md) — operator-
  targeted 30-minute path from upgrade through first enterprise login
  (supports SC-002).
