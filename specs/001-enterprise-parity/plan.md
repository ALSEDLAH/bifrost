# Implementation Plan: Bifrost Enterprise Parity

**Branch**: `001-enterprise-parity` | **Date**: 2026-04-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/001-enterprise-parity/spec.md`

## Summary

Enable Bifrost's 26 hidden enterprise features by:
1. **Re-enabling** the governance and prompts plugins in enterprise mode
2. **Removing** the redundant enterprise-gate plugin
3. **Filling** all 26 ContactUsView fallback stubs with real implementations
4. **Wiring** real RBAC enforcement using the frontend's 29-resource model
5. **Adding** missing backend handlers only where upstream has none

**Core insight**: Bifrost upstream ships ~40% of the enterprise surface —
governance plugin (budgets, rate-limits, routing, VK/team/customer CRUD),
prompts plugin, 26 pre-wired UI routes, RBAC context, config schema
anchors. These are disabled or stubbed behind `IS_ENTERPRISE`. Our job
is to **enable, fill, and connect** — not rebuild.

**Key decisions** (clarification sessions 2026-04-19):
- US8 DROPPED — governance plugin already ships budgets/rate-limits
- Enterprise-gate plugin REMOVED — redundant with governance's VK→Team→Customer resolution
- `governance_customers` = organizations, `governance_teams` = workspaces — no new tables
- RBAC aligned to frontend's 29 resources × 6 operations
- Fill existing fallback stubs — drop parallel routes
- US30 added for 4 previously uncovered stubs (MCP tool groups, MCP auth, large payload, proxy)
- SC-020 added: zero ContactUsView stubs in enterprise build

## Technical Context

**Language/Version**: Go 1.26.1+ (multi-module workspace), React 19 +
Vite 8 + TypeScript 5.9 (UI), Mintlify MDX (docs), HCL 2 (Terraform).

**Primary Dependencies**:
- Request path: `fasthttp`, `sonic` (hot-path JSON), `valyala/fastjson`
- Storage: `pgx/v5` (PostgreSQL), `gorm`, existing `configstore`/`logstore`
- Auth (new): `crewjam/saml` (SAML 2.0), `coreos/go-oidc/v3` (OIDC)
- Crypto: `framework/encrypt/`, `framework/crypto/` (existing)
- Observability: existing `plugins/otel` + `plugins/telemetry`
- UI: `@tanstack/react-router`, `@reduxjs/toolkit` (RTK Query), `tailwindcss`, Radix UI

**Storage**:
- `configstore` (SQLite or PostgreSQL): governance_customers (=orgs),
  governance_teams (=workspaces), governance_virtual_keys, governance_budgets,
  governance_rate_limits, governance_routing_rules, ent_users, ent_roles,
  ent_user_role_assignments
- `logstore` (SQLite or PostgreSQL): bifrost_logs, ent_audit_entries

**Testing**:
- Go: `make test-all`, `make test-enterprise` (real PostgreSQL)
- UI: Vitest + Playwright E2E (`data-testid` convention)
- Contract: OpenAPI conformance via `specs/001-enterprise-parity/contracts/`

**Target Platform**: Linux/amd64+arm64 containers, Helm chart, Terraform modules.
**Project Type**: Web service + UI + IaC (existing Bifrost shape).
**Performance Goals**: <1ms p50 overhead, <3ms p99 at 5k RPS (SC-005).
**Constraints**: Zero changes to `core/**`. All enterprise config optional.
**Scale/Scope**: 30 user stories, 26 UI stubs to fill, ~6 new backend handlers.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Status | Evidence / Notes |
|---|-----------|--------|------------------|
| I | Core Immutability | ✅ | Zero changes to `core/**`. All work in framework/, transports/, ui/ |
| II | Non-Breaking | ✅ | All new config fields optional. Existing hook signatures unchanged. |
| III | Plugin-First | ✅ | Reuse existing governance + prompts + audit plugins. New plugins only for net-new features (guardrails-central, pii-redactor, alerts, logexport). |
| IV | Config-Driven Gating | ✅ | `IS_ENTERPRISE` env + `deployment.mode` config. No build tags. |
| V | Multi-Tenancy First | ✅ | governance_customers = orgs, governance_teams = workspaces. CustomerID/TeamID FKs provide scoping. |
| VI | Observability | ✅ | `audit.Emit()` on every admin action. OTEL + Prometheus via existing plugins. |
| VII | Security by Default | ✅ | Existing configstore encryption. RBAC on all admin endpoints. |
| VIII | Test Coverage | ✅ | Integration tests against real PostgreSQL. Playwright E2E. |
| IX | Docs & Schema Sync | ✅ | config.schema.enterprise.json overlay. MDX docs. Changelog per module. |
| X | Dependency Hierarchy | ✅ | No reverse imports. framework → plugins → transports → ui. |
| XI | Upstream-Mergeability | ⚠ | Requires modifying `plugins.go` (remove 2 enterprise-mode guards) and `server.go` (remove 2 auth-middleware guards). Minimal, non-breaking. |

## Project Structure

### Documentation (this feature)

```text
specs/001-enterprise-parity/
├── plan.md              # This file
├── spec.md              # Feature specification (30 user stories)
├── research.md          # Phase 0 research (existing, valid)
├── data-model.md        # Phase 1 data model (revised)
├── contracts/           # OpenAPI + event + webhook contracts
│   ├── admin-api.openapi.yaml
│   ├── events.md
│   └── webhook-payloads.md
└── tasks.md             # Phase 2 task list (to be generated via /speckit-tasks)
```

### Source Code (repository root)

```text
# BACKEND — Server wiring (4 files modified)
transports/bifrost-http/server/
├── plugins.go           # Remove IS_ENTERPRISE guards on governance + prompts
└── server.go            # Remove IS_ENTERPRISE guards on auth middleware

# BACKEND — Existing handlers (reuse for org/workspace/VK/routing/budget)
transports/bifrost-http/handlers/
├── governance.go        # REUSE: /api/governance/{customers,teams,virtual-keys,budgets,rate-limits,routing-rules}
├── prompts.go           # REUSE: prompt CRUD (extend for deployments)
├── logging.go           # REUSE: log queries + stats
└── enterprise_helpers.go # KEEP: shared helpers (update)

# BACKEND — New handlers (only where upstream has nothing)
transports/bifrost-http/handlers/
├── audit_logs.go        # NEW: query + export ent_audit_entries (US4)
├── rbac.go              # NEW: roles/users/assignments CRUD (US2)
# DESCOPED 2026-04-20 per SR-01 (each needs its own feature spec):
#   admin_api_keys.go    — US5 — upstream basic-auth already covers admin auth
#   sso.go               — US3 — net-new SAML/OIDC stack
#   scim.go              — US20 — net-new SCIM 2.0 handler
#   guardrails_central.go — US6 — net-new plugin

# BACKEND — RBAC (align to frontend 24-resource model)
framework/tenancy/
├── scopes.go            # REWRITE: 24 resources × 6 operations
├── roles.go             # UPDATE: match new scope model
├── context.go           # KEEP: TenantContext (already has RoleScopes)
└── fromcontext.go       # NEW (sibling): FromContext / FromGoContext helpers

# BACKEND — Existing plugins (reuse)
plugins/
├── governance/          # REUSE: budgets, rate-limits, routing, VK enforcement
│                          +sibling tracker_thresholds.go + main_thresholds.go (US8 T055)
├── prompts/             # REUSE: prompt library (extend for deployments)
├── audit/               # USE: audit.Init() registered at server boot (T026 + C2 remediation)
├── license/             # KEEP (scaffold only); DESCOPED per SR-01 — Train E US24+ needs own spec
├── otel/                # REUSE: OpenTelemetry spans
└── telemetry/           # REUSE: Prometheus metrics

# BACKEND — DESCOPED 2026-04-20 per SR-01 (each needs its own feature spec):
#   plugins/guardrails-central/  — US6 — net-new org-wide guardrail engine
#   plugins/pii-redactor/        — US7 — net-new PII detection/redaction
#   plugins/alerts/              — US10 — net-new threshold alert destinations
#   plugins/logexport/           — US11 — net-new log export to S3/Blob/GCS/etc

# BACKEND — Remove (redundant)
plugins/enterprise-gate/                           # DELETE
framework/configstore/tables-enterprise/organization.go  # DELETE
framework/configstore/tables-enterprise/workspace.go     # DELETE
framework/configstore/tables-enterprise/*_tenancy.go     # DELETE (5 sidecar files)

# FRONTEND — Fill 26 fallback stubs (replace ContactUsView with real UI)
ui/app/enterprise/components/
├── user-groups/businessUnitsView.tsx          # US1: wraps /api/governance/customers
├── user-groups/teamsView.tsx                  # US1: wraps /api/governance/teams
├── user-groups/usersView.tsx                  # US2: user management + role assignment
├── rbac/rbacView.tsx                          # US2: roles & permissions
├── audit-logs/auditLogsView.tsx               # US4: audit log viewer + export
├── api-keys/apiKeysIndexView.tsx              # US5: admin API keys
├── access-profiles/accessProfilesIndexView.tsx # US5: access profiles
├── guardrails/guardrailsConfigurationView.tsx # US6: central guardrails
├── guardrails/guardrailsProviderView.tsx      # US6: guardrail providers
├── pii-redactor/piiRedactorRulesView.tsx      # US7: PII rules
├── pii-redactor/piiRedactorProviderView.tsx   # US7: PII providers
├── alert-channels/alertChannelsView.tsx       # US10: alert channels
├── data-connectors/datadog/*                  # US11: Datadog export
├── data-connectors/bigquery/*                 # US11: BigQuery export
├── user-rankings/userRankingsTab.tsx          # US12: executive dashboard
├── prompt-deployments/promptDeploymentView.tsx # US14: deployment strategies
├── prompt-deployments/promptDeploymentsAccordionItem.tsx # US14
├── login/loginView.tsx                        # US3: enterprise SSO
├── scim/scimView.tsx                          # US20: SCIM config
├── cluster/clusterView.tsx                    # US19: cluster management
├── adaptive-routing/adaptiveRoutingView.tsx   # US22: canary routing
├── mcp-tool-groups/mcpToolGroups.tsx          # US30: MCP tool groups
├── mcp-auth-config/mcpAuthConfigView.tsx      # US30: MCP auth config
├── large-payload/largePayloadSettingsFragment.tsx # US30: large payload
├── orgs-workspaces/organizationSettingsView.tsx   # US1: (already built → update)
└── orgs-workspaces/workspacesView.tsx             # US1: (already built → update)

# FRONTEND — RBAC (wire real enforcement)
ui/app/enterprise/lib/contexts/rbacContext.tsx  # REPLACE fallback with real impl

# FRONTEND — RTK Query APIs
ui/lib/store/apis/enterpriseApi.ts             # UPDATE: add hooks for new handlers
ui/lib/types/enterprise.ts                     # UPDATE: add types for new entities
```

**Structure Decision**: Existing Bifrost multi-module Go workspace +
React/Vite UI. No new top-level directories. All work follows the
existing handler/plugin/framework pattern. Enterprise features are
delivered by filling fallback stubs + adding missing handlers.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| XI: Modify `plugins.go` to re-enable governance+prompts | Governance plugin IS the budget/rate-limit/routing engine; disabling it in enterprise mode breaks all spend enforcement | Wrapping governance in a facade plugin duplicates ~1600 LOC for no benefit |
| XI: Modify `server.go` to re-enable auth middleware | Enterprise mode still needs base auth; enterprise-gate was handling it but is now removed | Separate auth middleware file would still need the same conditional removal |

## Phased Delivery Strategy

### Phase 1: Enable & Wire (v1.6.0 — Train A, P1)

**Goal (updated 2026-04-20 per SR-01)**: Re-enable disabled features, wire RBAC, fill US1/US2/US4 stubs. US5 steps 1.15–1.17 are DESCOPED (upstream `auth_config` basic-auth already covers admin auth) — see SR-01 §Classification gate.

**SC-020 progress (revised)**: every IN-SCOPE stub per SR-01 is filled.

| Step | What | Files | US |
|------|------|-------|----|
| 1.1 | Re-enable governance plugin in enterprise mode | `plugins.go:187` — remove guard | — |
| 1.2 | Re-enable prompts plugin in enterprise mode | `plugins.go:167` — remove guard | — |
| 1.3 | Re-enable auth middleware in enterprise mode | `server.go:1414,1428` — remove guards | — |
| 1.4 | Remove enterprise-gate plugin | Delete `plugins/enterprise-gate/` | — |
| 1.5 | Remove redundant tables | Delete `tables-enterprise/{organization,workspace,*_tenancy}.go` | — |
| 1.6 | Keep useful tables | Retain `ent_users`, `ent_roles`, `ent_user_role_assignments`, `ent_system_defaults`, `ent_audit_entries` | — |
| 1.7 | Align RBAC scopes to frontend | Rewrite `framework/tenancy/scopes.go`: 24 resources × 6 ops | US2 |
| 1.8 | Wire real RBAC context | Replace `rbacContext.tsx` fallback with real implementation | US2 |
| 1.9 | Fill `businessUnitsView` stub | Wrap `/api/governance/customers` CRUD | US1 |
| 1.10 | Fill `teamsView` stub | Wrap `/api/governance/teams` CRUD | US1 |
| 1.11 | Fill `usersView` stub | User list + role assignment UI | US2 |
| 1.12 | Update `rbacView` stub | Align to 24-resource scope model | US2 |
| 1.13 | Add audit log query handler | `handlers/audit_logs.go` — query + filter + CSV/JSON export | US4 |
| 1.14 | Fill `auditLogsView` stub | Audit log viewer + export UI | US4 |
| ~~1.15~~ | ~~Add admin API keys handler + table~~ | DESCOPED 2026-04-20 per SR-01 (US5) | US5 |
| ~~1.16~~ | ~~Fill `apiKeysIndexView` stub~~ | DESCOPED 2026-04-20 per SR-01 (US5) — OSS fallback now serves as the admin-auth surface | US5 |
| ~~1.17~~ | ~~Fill `accessProfilesIndexView` stub~~ | DESCOPED 2026-04-20 per SR-01 (US5) | US5 |

### Phase 2: Governance Depth (v1.7.0 — Train B, P2)

**Goal**: Central guardrails, PII redaction, budget alerts.
**SC-020 progress**: 14 of 26 stubs filled.

| Step | What | Files | US |
|------|------|-------|----|
| 2.1 | New guardrails-central plugin | `plugins/guardrails-central/` | US6 |
| 2.2 | Guardrails handler | `handlers/guardrails_central.go` | US6 |
| 2.3 | Fill `guardrailsConfigurationView` stub | Central guardrails config UI | US6 |
| 2.4 | Fill `guardrailsProviderView` stub | Guardrail provider management UI | US6 |
| 2.5 | New pii-redactor plugin | `plugins/pii-redactor/` | US7 |
| 2.6 | Fill `piiRedactorRulesView` stub | PII rules UI | US7 |
| 2.7 | Fill `piiRedactorProviderView` stub | PII provider UI | US7 |
| 2.8 | Add threshold-alert emission | Extend `plugins/governance/tracker.go` | US8 |
| 2.9 | Custom guardrail webhooks | Extend guardrails-central plugin | US9 |

### Phase 3: Observability + DX (v1.8.0 — Train C, P3)

**Goal**: Alerts, log export, dashboard, retention, prompts, configs, platform features.
**SC-020 progress**: 24 of 26 stubs filled.

| Step | What | Files | US |
|------|------|-------|----|
| 3.1 | New alerts plugin | `plugins/alerts/` | US10 |
| 3.2 | Fill `alertChannelsView` stub | Alert channel config UI | US10 |
| 3.3 | New logexport plugin | `plugins/logexport/` | US11 |
| 3.4 | Fill `datadogConnectorView` stub | Datadog export UI | US11 |
| 3.5 | Fill `bigqueryConnectorView` stub | BigQuery export UI | US11 |
| 3.6 | Fill `userRankingsTab` stub | Executive dashboard extension | US12 |
| 3.7 | Retention policy handler | Config extension | US13 |
| 3.8 | Extend prompts plugin for deployments | Deployment strategies | US14 |
| 3.9 | Fill `promptDeploymentView` stub | Prompt deployment UI | US14 |
| 3.10 | Prompt playground handler + UI | New page | US15 |
| 3.11 | Config object handler | Declarative configs | US16 |
| 3.12 | Fill `mcpToolGroups` stub | MCP tool group management UI | US30 |
| 3.13 | Fill `mcpAuthConfigView` stub | MCP auth config UI | US30 |
| 3.14 | Fill `largePayloadSettingsFragment` stub | Large payload settings UI | US30 |
| 3.15 | Fill proxy/SCIM section | Proxy config SCIM section | US30 |

### Phase 4: Security + Ecosystem (v1.9.0 — Train D, P4-P5)

**Goal**: SSO, SCIM, BYOK, air-gapped, service accounts, Terraform, canary.
**SC-020 progress**: 26 of 26 stubs filled. ✅

| Step | What | Files | US |
|------|------|-------|----|
| 4.1 | SSO handler (SAML + OIDC) | `handlers/sso.go` | US3 |
| 4.2 | Fill `loginView` stub | Enterprise SSO login UI | US3 |
| 4.3 | Service account keys handler | New handler + table | US17 |
| 4.4 | BYOK KMS adapters | `framework/kms/` | US18 |
| 4.5 | Air-gapped Helm profile | `helm-charts/bifrost/values-airgapped.yaml` | US19 |
| 4.6 | Fill `clusterView` stub | Cluster management UI | US19 |
| 4.7 | SCIM handler | `handlers/scim.go` | US20 |
| 4.8 | Fill `scimView` stub | SCIM config UI | US20 |
| 4.9 | Terraform provider | `terraform/providers/bifrost/` | US21 |
| 4.10 | Canary routing | Config-level primitive | US22 |
| 4.11 | Fill `adaptiveRoutingView` stub | Canary/adaptive routing UI | US22 |
| 4.12 | Data lake ETL | Export extension | US23 |

### Phase 5: Cloud Commercial (v2.0.0 — Train E)

**Goal**: License, metering, billing, portal, tiers.

| Step | What | US |
|------|------|----|
| 5.1 | License verification (self-hosted) | US24, US25 |
| 5.2 | Per-org metering | US26 |
| 5.3 | Stripe billing | US27 |
| 5.4 | Billing portal | US28 |
| 5.5 | Tier management | US29 |

## Upstream Feature Reuse Map

| Feature | Upstream | Action |
|---------|----------|--------|
| Budget enforcement | ✅ `plugins/governance/` | Reuse; add threshold alerts only |
| Rate-limit enforcement | ✅ `plugins/governance/` | Reuse as-is |
| Routing rules (CEL) | ✅ `plugins/governance/` | Reuse as-is |
| VK CRUD | ✅ `/api/governance/virtual-keys` | Reuse; UI exists |
| Team CRUD | ✅ `/api/governance/teams` | Reuse as workspace management |
| Customer CRUD | ✅ `/api/governance/customers` | Reuse as org management |
| Model configs | ✅ `/api/governance/model-configs` | Reuse as-is |
| Provider governance | ✅ `/api/governance/providers` | Reuse as-is |
| Prompts plugin | ✅ `plugins/prompts/` | Re-enable; extend for deployments |
| Audit plugin | ✅ `plugins/audit/` | Keep; add query handler |
| License scaffold | ✅ `plugins/license/` | Keep; implement in Phase 5 |
| IS_ENTERPRISE flag | ✅ UI + backend | Use as-is |
| 26 UI fallback stubs | ✅ ContactUsView | Fill with real implementations |
| RBAC context (29 resources) | ✅ Fallback always-true | Wire real enforcement |
| Config schema anchors | ✅ guardrails, audit, cluster, scim | Extend as needed |
| Deployment mode enum | ✅ `framework/deploymentmode/` | Use as-is |
| Encryption infrastructure | ✅ `framework/encrypt/` | Use; extend for BYOK |
| MCP handler | ✅ `handlers/mcp.go` | Reuse for tool groups + auth |
| Large payload streaming | ✅ `governance/main.go` IsLargePayloadMode | Reuse; fill UI stub |
| Proxy config | ✅ `handlers/config.go` proxy-config | Reuse; fill SCIM section |
