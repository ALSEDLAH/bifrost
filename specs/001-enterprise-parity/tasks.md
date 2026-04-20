---
description: "Task list for Bifrost Enterprise Parity — enable, fill, and connect hidden enterprise features"
---

# Tasks: Bifrost Enterprise Parity

**Input**: Design documents from `specs/001-enterprise-parity/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: INCLUDED — Constitution Principle VIII mandates integration tests on real dependencies (PostgreSQL) and Playwright E2E for every new UI page.

**Organization**: Tasks grouped by user story for independent implementation and testing. Within each: backend → UI → test.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1, US2, etc.)
- All paths relative to repository root

---

## Phase 1: Setup (Cleanup & Enable)

**Purpose**: Remove redundant code, re-enable disabled plugins, prepare foundation.

- [X] T001 Remove `plugins/enterprise-gate/` directory entirely (redundant with governance plugin)
- [X] T002 [P] Remove `framework/configstore/tables-enterprise/organization.go` (use governance_customers)
- [X] T003 [P] Remove `framework/configstore/tables-enterprise/workspace.go` (use governance_teams)
- [X] T004 [P] Remove `framework/configstore/tables-enterprise/virtual_key_tenancy.go` (upstream has CustomerID/TeamID FKs)
- [X] T005 [P] Remove `framework/configstore/tables-enterprise/team_tenancy.go`
- [X] T006 [P] Remove `framework/configstore/tables-enterprise/customer_tenancy.go`
- [X] T007 [P] Remove `framework/configstore/tables-enterprise/provider_tenancy.go`
- [X] T008 [P] Remove `framework/configstore/tables-enterprise/provider_key_tenancy.go`
- [X] T009 Remove migrations for dropped tables in `framework/configstore/migrations_enterprise.go` (keep E001 system_defaults, E004 users/roles)
- [X] T010 Remove migrations for dropped sidecar tables in `framework/logstore/migrations_enterprise.go` (keep ent_audit_entries)
- [X] T011 Re-enable governance plugin in enterprise mode — remove `BifrostContextKeyIsEnterprise` guard at `transports/bifrost-http/server/plugins.go:187`
- [X] T012 [P] Re-enable prompts plugin in enterprise mode — remove guard at `transports/bifrost-http/server/plugins.go:167`
- [X] T013 [P] Re-enable API auth middleware in enterprise mode — remove guard at `transports/bifrost-http/server/server.go:1414`
- [X] T014 [P] Re-enable inference auth middleware — remove guard at `transports/bifrost-http/server/server.go:1428`
- [X] T015 Remove our parallel routes: delete `ui/app/workspace/workspaces/` directory and `ui/app/workspace/organization/` directory (upstream routes exist)
- [X] T016 Remove enterprise sidebar section we added in `ui/components/sidebar.tsx` (upstream sidebar already links to the correct enterprise routes)
- [X] T017 Update `transports/go.mod` — remove ALSEDLAH fork module references; remove replace directives for enterprise-gate, audit, license
- [X] T018 Update `transports/Dockerfile.local` — remove enterprise-gate from go work use list
- [X] T019 Verify build: `docker build -f transports/Dockerfile.local` succeeds after cleanup

**Checkpoint**: Clean codebase with governance + prompts re-enabled in enterprise mode.

---

## Phase 2: Foundational (RBAC + Audit Infrastructure)

**Purpose**: Wire real RBAC enforcement and audit query handler — blocks all UI stub work.

**⚠️ CRITICAL**: No user story UI work can begin until RBAC context is real (not always-true fallback).

- [X] T020 Rewrite `framework/tenancy/scopes.go` — 29 RbacResource × 6 RbacOperation matching `ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx` enums
- [X] T021 Update `framework/tenancy/roles.go` — match new 29×6 scope model; update CreateRole/UpdateRole/ParseScopeJSON
- [X] T022 Update built-in role seed data in `framework/configstore/migrations_enterprise.go` — Owner/Admin/Member/Manager scopes use new 29-resource model
- [X] T023 Wire real RBAC context — replaced `ui/app/enterprise/lib/contexts/rbacContext.tsx` with implementation that fetches resolved scopes from `/api/rbac/me` and checks against the 24-resource model. (Completed alongside T032 after audit found the prior stub-only state.)
- [X] T024 [P] Add RTK Query types for RBAC in `ui/lib/types/enterprise.ts` — EnterpriseRole/EnterpriseUser/Assignment types with 24-resource scopes. (Completed alongside T032.)
- [X] T025 [P] Update RTK Query hooks in `ui/lib/store/apis/enterpriseApi.ts` — roles + users + assignments CRUD wired to new `transports/bifrost-http/handlers/rbac.go` handler. (Completed alongside T032.)
- [X] T026 Add audit log query handler at `transports/bifrost-http/handlers/audit_logs.go` — GET `/api/audit-logs` with filters (actor, action, resource, date range, outcome) + CSV/JSON export endpoint

**Checkpoint**: RBAC enforcement is real; audit logs are queryable. UI stub filling can begin.

---

## Phase 3: User Story 1 — Orgs & Workspaces (Priority: P1) 🎯 MVP

**Goal**: Fill the 3 user-groups fallback stubs with real UI wrapping existing governance CRUD.

**Independent Test**: Navigate to `/workspace/organization`; see customer/team management UI; create a team; verify it appears in `/api/governance/teams`.

- [X] T027 [P] [US1] Fill `ui/app/enterprise/components/user-groups/businessUnitsView.tsx` — org/customer management wrapping `/api/governance/customers` CRUD (list, create, edit, delete)
- [X] T028 [P] [US1] Fill `ui/app/enterprise/components/user-groups/teamsView.tsx` — workspace/team management wrapping `/api/governance/teams` CRUD (list, create, edit, delete)
- [X] T029 [P] [US1] Update `ui/app/enterprise/components/orgs-workspaces/organizationSettingsView.tsx` — use governance customer data instead of our custom /admin/organizations endpoint
- [X] T030 [P] [US1] Update `ui/app/enterprise/components/orgs-workspaces/workspacesView.tsx` — use governance teams data instead of our custom /admin/workspaces endpoint
- [X] T031 [US1] Playwright E2E: `ui/tests/e2e/enterprise/orgs-workspaces.spec.ts` — create customer, create team, verify isolation

**Checkpoint**: US1 stubs filled. Org/workspace management works via governance CRUD.

---

## Phase 4: User Story 2 — Granular RBAC (Priority: P1)

**Goal**: Fill RBAC + users stubs with real permission management.

**Independent Test**: Create custom role "ReadOnly Analyst" with metrics:read only; assign to user; verify 403 on write endpoints.

- [X] T032 [P] [US2] Fill `ui/app/enterprise/components/rbac/rbacView.tsx` — roles list + create with 24-resource permission matrix
- [X] T033 [P] [US2] Fill `ui/app/enterprise/components/user-groups/usersView.tsx` — user list + role assignment modal
- [X] T034 [US2] Playwright E2E: `ui/tests/e2e/enterprise/rbac.spec.ts` — create custom role, assign user, verify scope enforcement (spec written; Playwright infra pending per T031 carryover)

**Checkpoint**: US2 stubs filled. RBAC enforcement functional end-to-end.

---

## Phase 5: User Story 4 — Audit Logs (Priority: P1)

**Goal**: Fill audit logs stub with real viewer + export.

**Independent Test**: Perform 5 admin actions; navigate to audit logs; verify all 5 appear with filters; export CSV.

- [X] T035 [P] [US4] Add RTK Query hooks for audit logs in `ui/lib/store/apis/enterpriseApi.ts` — GET `/api/audit-logs` with filter params
- [X] T036 [US4] Fill `ui/app/enterprise/components/audit-logs/auditLogsView.tsx` — table with filters (actor, action, resource, date range, outcome) + CSV/JSON export button
- [X] T037 [US4] Playwright E2E: `ui/tests/e2e/enterprise/audit-logs.spec.ts` — verify audit entries appear after admin actions (spec written; see T031 carryover for Playwright infra)

**Checkpoint**: US4 stub filled. Audit trail viewable and exportable.

---

## Phase 6: User Story 5 — Admin API Keys (Priority: P1) — DESCOPED 2026-04-20

**Status**: DESCOPED. Originally Phase 6 would have added a net-new `ent_admin_api_keys` table + handler + scoped-key management UI. Upstream already ships an admin auth path (`auth_config` basic auth via `admin_username`/`admin_password`) exposed by the existing `apiKeysIndexView` fallback. A parallel multi-key scoped system is net-new and violates the reuse-over-new scope rule.

If ever revived as its own spec: it would be a new feature with its own plan/tasks, not a stub-fill. The fallback UI (basic-auth curl example) stays as the enterprise view.

- [~] T038 [US5] DESCOPED — was: create `ent_admin_api_keys` table
- [~] T039 [US5] DESCOPED — was: `transports/bifrost-http/handlers/admin_api_keys.go`
- [~] T040 [US5] DESCOPED — was: RTK hooks for admin API keys
- [~] T041 [US5] DESCOPED — was: fill `apiKeysIndexView.tsx` with scoped-key UI (now remains an OSS fallback re-export showing the upstream basic-auth credential)
- [~] T042 [US5] DESCOPED — was: fill `accessProfilesIndexView.tsx` (no upstream concept to surface)
- [~] T043 [US5] DESCOPED — was: Playwright E2E for admin API keys

---

## Phase 7: User Story 6 — Central Guardrails (Priority: P2)

**Goal**: Fill guardrails stubs with central guardrail management.

**Independent Test**: Configure org-wide prompt-injection guardrail; send injection from two workspaces; both blocked with HTTP 446.

- [ ] T044 [US6] Create `plugins/guardrails-central/` plugin with `go.mod` — org-wide guardrail engine
- [ ] T045 [US6] Add guardrails handler at `transports/bifrost-http/handlers/guardrails_central.go` — CRUD for guardrail policies
- [ ] T046 [P] [US6] Add RTK Query hooks for guardrails in `ui/lib/store/apis/enterpriseApi.ts`
- [ ] T047 [P] [US6] Fill `ui/app/enterprise/components/guardrails/guardrailsConfigurationView.tsx` — guardrail config UI
- [ ] T048 [P] [US6] Fill `ui/app/enterprise/components/guardrails/guardrailsProviderView.tsx` — guardrail provider management UI
- [ ] T049 [US6] Playwright E2E: `ui/tests/e2e/enterprise/guardrails.spec.ts`

**Checkpoint**: US6 stubs filled. Central guardrails enforceable org-wide.

---

## Phase 8: User Story 7 — PII Redaction (Priority: P2)

**Goal**: Fill PII stubs with redaction management.

**Independent Test**: Enable PII redaction; send request with SSN; verify log entry contains `<REDACTED:SSN>`.

- [ ] T050 [US7] Create `plugins/pii-redactor/` plugin with `go.mod` — PII detection + redaction engine
- [ ] T051 [P] [US7] Add RTK Query hooks for PII redaction in `ui/lib/store/apis/enterpriseApi.ts`
- [ ] T052 [P] [US7] Fill `ui/app/enterprise/components/pii-redactor/piiRedactorRulesView.tsx` — PII rules config UI
- [ ] T053 [P] [US7] Fill `ui/app/enterprise/components/pii-redactor/piiRedactorProviderView.tsx` — PII provider management UI
- [ ] T054 [US7] Playwright E2E: `ui/tests/e2e/enterprise/pii-redactor.spec.ts`

**Checkpoint**: US7 stubs filled.

---

## Phase 9: User Story 8 — Budget Threshold Alerts + US9 Webhooks (Priority: P2)

**Goal**: Add threshold-alert emission to existing governance tracker; add webhook guardrail support.

**Independent Test**: Set $100 budget; generate $50 spend; verify 50% alert fires.

- [ ] T055 [US8] Add threshold-alert emission to `plugins/governance/tracker.go` — emit at 50%/75%/90% of budget to configured alert destinations
- [ ] T056 [US9] Add webhook guardrail type to `plugins/guardrails-central/` — HMAC signing, timeout, fail-open/fail-closed

**Checkpoint**: US8 + US9 complete. Budget alerts and custom guardrail webhooks functional.

---

## Phase 10: User Story 10 — Alerts (Priority: P3)

**Goal**: Fill alert channels stub with real alert management.

**Independent Test**: Configure alert rule for error-rate > 5%; trigger; verify Slack notification.

- [ ] T057 [US10] Create `plugins/alerts/` plugin with `go.mod` — threshold-based alert engine with webhook/Slack destinations
- [ ] T058 [P] [US10] Add RTK Query hooks for alert channels in `ui/lib/store/apis/enterpriseApi.ts`
- [ ] T059 [US10] Fill `ui/app/enterprise/components/alert-channels/alertChannelsView.tsx` — alert rule + destination management UI
- [ ] T060 [US10] Playwright E2E: `ui/tests/e2e/enterprise/alert-channels.spec.ts`

**Checkpoint**: US10 stub filled.

---

## Phase 11: User Story 11 — Log Export (Priority: P3)

**Goal**: Fill data connector stubs with real export configuration.

**Independent Test**: Configure S3 export; generate 100 requests; verify S3 bucket receives records.

- [ ] T061 [US11] Create `plugins/logexport/` plugin with `go.mod` — export framework with S3/Blob/GCS/MongoDB/OTLP sinks
- [ ] T062 [P] [US11] Add RTK Query hooks for log export in `ui/lib/store/apis/enterpriseApi.ts`
- [ ] T063 [P] [US11] Fill `ui/app/enterprise/components/data-connectors/datadog/datadogConnectorView.tsx` — Datadog export config UI
- [ ] T064 [P] [US11] Fill `ui/app/enterprise/components/data-connectors/bigquery/bigqueryConnectorView.tsx` — BigQuery export config UI
- [ ] T065 [US11] Playwright E2E: `ui/tests/e2e/enterprise/log-export.spec.ts`

**Checkpoint**: US11 stubs filled.

---

## Phase 12: User Stories 12+13 — Dashboard + Retention (Priority: P3)

**Goal**: Fill executive dashboard stub; add retention policy support.

- [~] T066 [US12] DESCOPED 2026-04-20 — was: exec dashboard with top users. UI types (`UserRankingEntry`, `UserRankingsResponse` in `lib/types/logs.ts`) are forward-declared but no backend endpoint (`/api/logs/user-rankings`) exists and no existing aggregation provides per-user requests/tokens/cost. Model rankings (`/api/logs/rankings`) exist but that's a different dimension. Per SR-01, user rankings needs its own spec before implementation. Fallback ContactUsView stays.
- [X] T067 [US13] Retention policy — already upstream. `TableClientConfig.LogRetentionDays` (framework/configstore/tables/clientconfig.go:22, default 365) is migrated via `migrationAddLogRetentionDaysColumn` and already surfaced at `/workspace/config/logging` via `loggingView.tsx` as "Log retention days" (min=1). No code change required; this is pure expose-existing that already shipped.

**Checkpoint**: US12 + US13 complete.

---

## Phase 13: User Stories 14-16 — Prompts + Configs (Priority: P4)

**Goal**: Fill prompt deployment stubs; add playground; add config objects.

- [ ] T068 [US14] Extend `plugins/prompts/` for deployment strategies (A/B split, canary, production version)
- [ ] T069 [P] [US14] Fill `ui/app/enterprise/components/prompt-deployments/promptDeploymentView.tsx` — deployment strategy editor UI
- [ ] T070 [P] [US14] Fill `ui/app/enterprise/components/prompt-deployments/promptDeploymentsAccordionItem.tsx` — accordion display component
- [ ] T071 [US15] Add prompt playground handler + UI page — N-model parallel execution with comparison
- [ ] T072 [US16] Add Config object handler at `transports/bifrost-http/handlers/configs.go` — versioned declarative routing configs

**Checkpoint**: US14-16 complete.

---

## Phase 14: User Story 17 — Service Account Keys (Priority: P4)

- [ ] T073 [US17] Create `ent_service_account_keys` table + handler — workspace-scoped non-user credentials with budget/rate-limit

**Checkpoint**: US17 complete.

---

## Phase 15: User Story 30 — Enterprise Platform Features (Priority: P3)

**Goal**: Fill remaining 4 platform stubs.

- [~] T074 [P] [US30] DESCOPED 2026-04-20 — was: MCP tool group management. `TableMCPClient` has no `tool_group_id` / grouping column and no `/api/mcp/tool-groups` endpoint; tool-groups is a net-new concept per SR-01, not a reuse of `/api/mcp/clients` CRUD. Fallback ContactUsView stays.
- [X] T075 [P] [US30] Fill `ui/app/enterprise/components/mcp-auth-config/mcpAuthConfigView.tsx` — surfaces existing `/api/mcp/clients` (filtered to `auth_type=oauth|per_user_oauth`) joined with `/api/oauth/config/:id/status` for status + token expiry + scopes; revoke via DELETE `/api/oauth/config/:id`. Zero new backend.
- [~] T076 [P] [US30] DESCOPED 2026-04-20 — was: large-payload streaming threshold settings. No `/api/large-payload-config` endpoint exists; `largePayloadApi` is a no-op fallback that returns undefined. Filling the fragment would require net-new persistence per SR-01. Fallback null-render stays; thresholds remain server-config-only.
- [X] T077 [US30] Unlock SCIM section in `ui/app/workspace/config/views/proxyView.tsx` — already functional: the `IS_ENTERPRISE=true` flag flipped by `npm run build-enterprise` already toggles the SCIM switch + `enable_for_scim` form field on (and hides the "available in Enterprise" alert). No code change needed; verified in the running enterprise build.

**Checkpoint**: US30 complete. All 4 platform stubs filled.

---

## Phase 16: User Stories 3, 19, 20 — SSO + Cluster + SCIM (Priority: P4-P5)

**Goal**: Fill SSO login, cluster, and SCIM stubs.

- [ ] T078 [US3] Add SSO handler at `transports/bifrost-http/handlers/sso.go` — SAML 2.0 + OIDC discovery, callbacks, user auto-provisioning
- [ ] T079 [US3] Fill `ui/app/enterprise/components/login/loginView.tsx` — enterprise SSO login page
- [ ] T080 [US19] Fill `ui/app/enterprise/components/cluster/clusterView.tsx` — cluster node management UI
- [ ] T081 [P] [US19] Create air-gapped Helm profile at `helm-charts/bifrost/values-airgapped.yaml`
- [ ] T082 [US20] Add SCIM handler at `transports/bifrost-http/handlers/scim.go` — RFC 7644 Users/Groups/Schemas endpoints
- [ ] T083 [US20] Fill `ui/app/enterprise/components/scim/scimView.tsx` — SCIM config UI (bearer token + enable toggle)

**Checkpoint**: US3, US19, US20 stubs filled.

---

## Phase 17: User Stories 18, 21, 22, 23 — BYOK + Terraform + Canary + ETL (Priority: P5-P6)

- [ ] T084 [US18] Create `framework/kms/` with AWS KMS, Azure Key Vault, GCP KMS adapters
- [ ] T085 [US21] Scaffold Terraform provider at `terraform/providers/bifrost/` — workspace, VK, config, guardrail resources
- [ ] T086 [US22] Add canary routing primitive to Config object + fill `ui/app/enterprise/components/adaptive-routing/adaptiveRoutingView.tsx`
- [ ] T087 [US23] Add data lake ETL export to `plugins/logexport/` — curated schema + BigQuery/S3 sink

**Checkpoint**: US18, US21, US22, US23 complete. All 26 stubs filled (SC-020 ✅).

---

## Phase 18: User Stories 24-29 — License + Cloud Commercial (Train E)

- [ ] T088 [US24] Implement license JWT verification in `plugins/license/` — offline Ed25519 signature check, entitlement parsing
- [ ] T089 [US25] Add license expiry state machine — 30/7/1 day warnings, 14-day grace period, graceful degradation
- [ ] T090 [US26] Create metering plugin — per-org request/token/cost counters with daily/monthly rollups
- [ ] T091 [US27] Integrate Stripe — subscription management + Metered Billing API + webhook handler
- [ ] T092 [US28] Build billing portal UI page — usage, invoices, payment method
- [ ] T093 [US29] Implement tier enforcement — Dev/Prod/Enterprise feature matrix

**Checkpoint**: Train E complete.

---

## Phase 19: Polish & Cross-Cutting Concerns

- [ ] T094 [P] Update `config.schema.enterprise.json` — ensure all new features have schema entries
- [ ] T095 [P] Update MDX docs in `docs/enterprise/` for each filled stub
- [ ] T096 [P] Changelog entries per affected module
- [ ] T097 CI check: verify zero "This feature is a part of the Bifrost enterprise license" text in built JS bundle (SC-020)
- [ ] T098 Performance benchmark: verify <1ms p50 overhead at 5k RPS (SC-005)
- [ ] T099 Security scan: verify zero plaintext secrets in logstore/metrics/traces (SC-013)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all UI stub work
- **Phases 3-6 (US1/US2/US4/US5)**: All depend on Phase 2; can run in parallel
- **Phases 7-9 (US6/US7/US8/US9)**: Depend on Phase 2; independent of each other
- **Phases 10-15 (US10-US16/US30)**: Depend on Phase 2; independent of each other
- **Phases 16-17 (US3/US18-US23)**: Depend on Phase 2; independent
- **Phase 18 (Train E)**: Depends on Phase 2; independent of other phases
- **Phase 19 (Polish)**: Depends on all desired phases being complete

### User Story Dependencies

Most stories are independent after Phase 2 (Foundational). Exceptions:
- **US8** (budget alerts) benefits from US10 (alerts) for destination wiring
- **US9** (webhook guardrails) depends on US6 (guardrails-central plugin)
- **US22** (canary) depends on US16 (Config objects)
- **US25** (license expiry) depends on US24 (license activation)
- **US27-US29** (billing) depend on US26 (metering)

### Parallel Opportunities

After Phase 2, these story groups can run simultaneously:
- Group A: US1 + US2 (orgs, RBAC — 6 tasks)
- Group B: US4 + US5 (audit, API keys — 9 tasks)
- Group C: US6 + US7 (guardrails, PII — 11 tasks)
- Group D: US10 + US11 + US12 + US13 (observability — 7 tasks)

---

## Implementation Strategy

### MVP First (Phase 1 + 2 + 3)

1. Phase 1: Cleanup + enable → Clean codebase
2. Phase 2: RBAC + audit → Enforcement real
3. Phase 3: US1 (orgs/workspaces) → **STOP and VALIDATE**
4. Deploy to `mvp.audytia.fr:8088` and demo

### Incremental Delivery

Each phase adds independently testable value:
- After Phase 6: All P1 stories done (US1/US2/US4/US5)
- After Phase 9: All P2 stories done (US6/US7/US8/US9)
- After Phase 15: All P3 stories + US30 done
- After Phase 17: All stubs filled (SC-020 met)
- After Phase 18: Full commercial product

---

## Notes

- [P] tasks = different files, no dependencies — safe to parallelize
- [Story] label maps task to specific user story
- Every stub fill task = replace ContactUsView with real RTK Query-backed component
- Backend handler tasks include audit.Emit() wiring per Constitution Principle VI
- Commit after each task or logical group
- Stop at any checkpoint to validate independently
