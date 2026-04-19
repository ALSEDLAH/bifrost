---
description: "Task list for Bifrost Enterprise Parity feature implementation"
---

# Tasks: Bifrost Enterprise Parity

**Input**: Design documents from [specs/001-enterprise-parity/](.)
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: INCLUDED — the Constitution (Principle VIII) mandates integration tests on real dependencies (PostgreSQL, vectorstore) and Playwright E2E for every new UI page.

**Organization**: Tasks are grouped first by the four release trains (A–D) from plan.md, then by user story (US1–US23) inside each train, so each story is independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks).
- **[Story]**: Traceability tag; one of US1–US23.
- Every task includes an exact file path or a concrete scope.

## Path Conventions

- Go modules: `core/` (UNTOUCHED), `framework/*/`, `plugins/*/`, `transports/bifrost-http/{handlers,lib}/`.
- UI: `ui/src/pages/<Area>/*.tsx`, `ui/tests/e2e/*.spec.ts`.
- Docs: `docs/enterprise/<feature>.mdx`.
- Schema: `transports/config.schema.json`.
- Deployment: `helm-charts/bifrost/`, `terraform/{providers,modules}/`.
- Migrations: GORM struct files in `framework/<store>/tables-enterprise/` (configstore) or `framework/<store>/tables_enterprise.go` (logstore) + migration registrations in `migrations_enterprise.go` per Constitution Principle XI rule 2.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Global scaffolding that must exist before any Train A work begins. No user-facing capability is delivered in this phase.

- [x] T001 Add CI job `check-core-unchanged` that diffs `core/` against `CORE_BASELINE_REF` (set to `v1.5.2`) in `.github/workflows/core-immutability.yml` (research R-01).
- [x] T002 [P] Add CI job `check-imports` enforcing core→framework→plugins→transport direction via new `make check-imports` target in `Makefile` (research R-10).
- [x] T003 [P] Add CI job `check-obs-completeness` that scans enterprise plugins for OTEL span + Prometheus metric + audit emit calls in `.github/workflows/obs-completeness.yml` (research R-09).
- [x] T004 [P] Add `make test-enterprise` target that spins up docker-compose with PostgreSQL + Redis + Weaviate + minio (S3-compat) for integration tests, in `Makefile` and `docker-compose.enterprise.yml`.
- [x] T005 [P] Add `.github/workflows/airgapped-smoke.yml` weekly matrix job running Kubernetes-in-Docker with egress restricted; gated by `NETWORK_EGRESS_RESTRICTED=true` (research R-06).
- [x] T006 [P] Update `.github/pull_request_template.md` with Constitution Principles I–X checklist block.
- [x] T007 [P] Extend `transports/config.schema.json` with the top-level `enterprise` object (default `{enabled: false}`) and JSON Schema conditionals that gate all downstream enterprise fields.
- [x] T008 [P] Add baseline `docs/enterprise/README.mdx` index page linking to the per-feature docs that later tasks will author.
- [x] T009 [P] Add `framework/configstore/tables-enterprise/system_defaults.go` defining `TableSystemDefaults` GORM struct (singleton row holding `SYSTEM_DEFAULT_ORG_UUID` and `SYSTEM_DEFAULT_WORKSPACE_UUID`) + register migration `E001_seed_default_org` in new `framework/configstore/migrations_enterprise.go` (research R-11).
- [x] T010 Wire golden-set replay test harness in `tests/golden/` with a 100-request corpus and baseline responses captured from `v1.5.2` (validates SC-001).

**Checkpoint**: CI gates are active, schema is extensible, replay harness exists. Train A work can start.

---

## Phase 1.5: Upstream Sync Tooling (Constitution Principle XI)

**Purpose**: Establish the merge-discipline tooling BEFORE any enterprise code lands, so the first week of enterprise PRs already runs under upstream-sync guardrails. Without this phase, the sixth week of enterprise work is where the fork becomes unmergeable. Ties directly to research §R-24 and Principle XI.

- [ ] T320 Add `upstream` git remote pointing at `https://github.com/maximhq/bifrost.git` and document the one-time setup in the top-level `README.md` (maintainer section).
- [ ] T321 [P] Author `UPSTREAM-SYNC.md` at repo root: weekly merge flow diagram, conflict-resolution playbook, upstream-carried-patches registry template, emergency "2-weeks-missed" recovery steps. (Content mirrors research R-24; this is the operator-facing runbook.)
- [ ] T322 [P] Create `.github/workflows/upstream-sync.yml` — Monday 09:00 UTC cron that fetches `upstream/main`, creates a `sync/upstream-YYYY-MM-DD` branch, merges with `--no-ff`, runs `make test-core` + `make test-plugins` + `make test-enterprise` + golden-set replay, and opens a PR when green.
- [ ] T323 [P] Create `.github/workflows/drift-watcher.yml` + `.github/drift-watchlist.txt` — on every PR, compute `git diff --stat upstream/main -- $(cat .github/drift-watchlist.txt)` and fail if the cumulative diff grows >50 lines above the previous-week baseline (stored in `.github/drift-baseline.json`) unless the commit begins with `drift:`. The watchlist includes the six extended plugin directories, `transports/bifrost-http/lib/middleware.go`, `transports/config.schema.json`, `AGENTS.md`, `CLAUDE.md`, `Makefile`, `go.work`, `helm-charts/bifrost/values.yaml`.
- [ ] T324 [P] Schema overlay loader: add `transports/config.schema.enterprise.json` (empty skeleton) and a single `allOf: [{ $ref: "./config.schema.enterprise.json" }]` anchor line in `transports/config.schema.json`. Implement the load-time composer in `transports/bifrost-http/lib/schema_loader.go` so subsequent enterprise fields are added ONLY to the overlay file (Principle XI rule 3).
- [ ] T325 [P] Migration-namespace separation via migrator IDs (no directory split needed since Bifrost uses GORM/migrator, not SQL files). Enterprise migrations registered via `RegisterEnterpriseMigrations(db)` (T019) use IDs prefixed `E###_<name>` so they sort disjoint from upstream's descriptive IDs in the migration tracking table. Document the convention in the AGENTS.md addendum (T316). NOTE: Spec/research/data-model/Constitution Principle XI rule 2 were updated 2026-04-19 to reflect Bifrost's actual GORM-based architecture (raw SQL files in `migrations-enterprise/` directory was a documentation error; corrected after pre-implementation reconnaissance).
- [ ] T326 [P] Hook-point refactors: add `RegisterEnterpriseMiddleware(chain *MiddlewareChain)` function signature + single call site in `transports/bifrost-http/lib/middleware.go` (one-line touch); full implementation lives in `transports/bifrost-http/lib/middleware_enterprise.go` (Principle XI rule 4). Same pattern for UI router: add an `enterpriseRoutes` import + spread in `ui/src/routes.tsx`; implementation in `ui/src/enterpriseRoutes.ts`.
- [ ] T327 [P] Add `deployment.mode` enum (`cloud` | `selfhosted` | `airgapped`) to `transports/config.schema.enterprise.json` with opinionated defaults per plan.md Deployment Modes table. Implement the mode resolver in `framework/deploymentmode/mode.go` consumed by the plugin loader to decide which plugins to register.
- [ ] T328 [P] Scaffold `plugins/license/` with `go.mod` as a no-op default implementation. Define `LicensePlugin` interface with `IsEntitled(feature string) bool`, `DaysUntilExpiry() int`, `InGracePeriod() bool`. Register at `pre_builtin` with `order: -100`. In `cloud` mode, plugin returns "always entitled"; in `selfhosted`/`airgapped`, plugin refuses to start without a valid license file (implementation lands in Phase 8 T400-series).
- [ ] T329 [P] Phone-home telemetry gate: add `framework/telemetry/phonehome.go` with a deployment-mode-aware sender. Default OFF for `selfhosted` (opt-in anonymous version ping via `telemetry.phone_home: true`), permanently OFF for `airgapped` (enforced by a boot-time assertion), default ON for `cloud`. Payload includes version + feature-enable summary only, never per-request data.
- [ ] T330 [P] Release-channel documentation: author `docs/enterprise/deployment-modes.mdx` consolidating the plan.md Deployment Modes table into customer-facing docs, and add a GitHub Action in `.github/workflows/release-channels.yml` that tags stable (quarterly) vs edge (continuous) from the same commit base.

**Checkpoint**: Any enterprise PR from T011 onward touches zero upstream-owned lines except the four hook anchors (T324 schema ref, T325 migration dir, T326 middleware hook, T326 router spread). Deployment mode + license plugin + phone-home policy are all wired in before any feature code. Merging upstream/main is a routine weekly task, not an engineering event.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that every user story below depends on. **No US-tagged work may begin until this phase is complete.**

### 2.1 Tenancy primitive

- [ ] T011 Create module `framework/tenancy/` with `go.mod`; define `TenantContext` struct (fields: `OrganizationID`, `WorkspaceID`, `UserID`, `RoleScopes`, `ResolvedVia`) in `framework/tenancy/context.go` (research R-02).
- [ ] T012 [P] Add tenancy context keys (`BifrostContextKeyOrganizationID`, `BifrostContextKeyWorkspaceID`, `BifrostContextKeyRoleScopes`) to the new `framework/tenancy/keys.go` — these are the authoritative names used by middleware and plugins (research R-02).
- [ ] T013 Add `framework/tenancy/repository.go` with helpers `ScopedQuery(ctx, baseSQL, args...)` that automatically injects `WHERE organization_id = $1 AND workspace_id = $2` into queries (research R-03).
- [ ] T014 Integration test: `framework/tenancy/tenancy_test.go` spins up PostgreSQL and verifies cross-tenant reads are blocked.

### 2.2 Unified encryption primitive

- [ ] T015 [P] Create `framework/crypto/` module with `Encrypt`/`Decrypt` using configstore `encryption_key` as default backend in `framework/crypto/configkey.go`.
- [ ] T016 [P] Define ciphertext envelope layout (`version | kek_ref_hash | dek_wrapped | nonce | ct+tag`) and helpers in `framework/crypto/envelope.go` (research R-05).

### 2.3 Tenancy migration for existing tables

- [ ] T017 Add sidecar GORM structs in `framework/configstore/tables-enterprise/`: `TableVirtualKeyTenancy`, `TableTeamTenancy`, `TableCustomerTenancy`, `TableProviderTenancy`, `TableProviderKeyTenancy` (one file per struct). Each is a 1:1 sidecar with `<entity>_id` PK + `organization_id` + `workspace_id` indexed fields. Register migration `E002_create_tenancy_sidecars` in `framework/configstore/migrations_enterprise.go` that creates the sidecar tables AND backfills one row per existing upstream row pointing at the synthetic default org/workspace (research R-03, data-model §9).
- [ ] T018 Add `framework/logstore/tables_enterprise.go` containing `TableLogTenancy` sidecar struct + register migration `E003_create_log_tenancy_sidecar` in new `framework/logstore/migrations_enterprise.go` that creates the sidecar and backfills existing log rows.
- [ ] T019 Add `RegisterEnterpriseMigrations(db *gorm.DB)` entry-point in each of `framework/configstore/migrations_enterprise.go` and `framework/logstore/migrations_enterprise.go`. Idempotency is provided by the existing `framework/migrator` package's tracking-table behavior — re-runs are no-ops (research R-11). The enterprise plugin's `Init()` calls these once at boot.
- [ ] T020 Integration test: `framework/configstore/migrations_enterprise_test.go` seeds a v1.5.2 fixture (existing virtual_keys / teams / customers rows without tenancy), runs `RegisterEnterpriseMigrations` then re-runs it, asserts idempotency, asserts every fixture row has a sidecar pointing at the default org, asserts no upstream rows were modified.

### 2.4 Audit sink plugin

- [ ] T021 Scaffold `plugins/audit/` with `go.mod`, `main.go` implementing `ObservabilityPlugin.Inject()` that writes `audit_entries` rows (research data-model §7).
- [ ] T022 [P] Add `plugins/audit/emit.go` exposing `audit.Emit(ctx, entry)` helper consumed by all other enterprise plugins.
- [ ] T023 [P] Add Playwright-invisible unit test for the Emit helper's serialization in `plugins/audit/emit_test.go`.

### 2.5 Enterprise-gate plugin (tenant resolution)

- [ ] T024 Scaffold `plugins/enterprise-gate/` with `go.mod`; implement `HTTPTransportPreHook` that resolves tenant per R-02 resolution order and writes `BifrostContext` keys.
- [ ] T025 [P] Add `plugins/enterprise-gate/features.go` registering the enterprise-feature manifest read by the obs-completeness CI job (research R-09).
- [ ] T026 [P] Unit test: `plugins/enterprise-gate/main_test.go` validates resolution-order determinism for all 4 auth-token types.
- [ ] T027 Integration test: `plugins/enterprise-gate/integration_test.go` sends requests with each auth type against a test HTTP server and asserts `BifrostContext` keys.

### 2.6 Tenancy middleware in transports

- [ ] T028 Extend `transports/bifrost-http/lib/middleware.go` with `TenantResolveMiddleware` and `RBACEnforceMiddleware` that consume `BifrostContext` keys and short-circuit with 401/403 when unresolved / scope-insufficient.
- [ ] T029 [P] Contract test: `transports/bifrost-http/lib/middleware_test.go` verifies 401 on missing auth, 403 on missing scope, happy path on valid admin API key.

**Checkpoint**: Tenancy, encryption, audit sink, enterprise-gate, and middleware are operational. All 23 user-story phases below can begin in parallel (subject to their own intra-story ordering).

---

## Phase 3: Train A — Tenancy + Identity (P1)

Scope: US1 Orgs/Workspaces, US2 RBAC, US3 SSO, US4 Audit Logs UI, US5 Admin API Keys.

### Phase 3.1 — User Story 1: Organizations & Workspaces (Priority: P1) 🎯 MVP

**Goal**: Deliver workspace-scoped isolation for configs, prompts, model catalog, virtual keys, logs, analytics.

**Independent Test**: Create two workspaces, scope a user to one, verify the user cannot list or access any resource in the other.

#### Tests

- [ ] T030 [P] [US1] Integration test: `framework/configstore/orgs_workspaces_test.go` exercises CRUD on organizations/workspaces tables with real PostgreSQL.
- [ ] T031 [P] [US1] Integration test: `transports/bifrost-http/handlers/workspaces_test.go` verifies 403 cross-workspace access.
- [ ] T032 [P] [US1] Playwright E2E: `ui/tests/e2e/workspaces-isolation.spec.ts` — two-workspace isolation scenario (spec US1 acceptance 3).

#### Schema & Migrations

- [ ] T033 [US1] Add GORM structs in `framework/configstore/tables-enterprise/`: `TableOrganization` (organization.go), `TableWorkspace` (workspace.go), `TableUser` (user.go), `TableRole` (role.go), `TableUserRoleAssignment` (user_role_assignment.go) per data-model §1. Register migration `E004_orgs_workspaces_users_roles` in `migrations_enterprise.go`.
- [ ] T034 [US1] Seed migration: insert the synthetic default organization + default workspace rows and back-stamp existing v1.5.2 rows.

#### Framework

- [ ] T035 [P] [US1] Add `framework/tenancy/orgs.go` with `CreateOrganization`, `UpdateOrganization`, `GetOrganization` repository functions.
- [ ] T036 [P] [US1] Add `framework/tenancy/workspaces.go` with CRUD + soft-delete with 30-day grace per spec edge case.

#### Transports (Admin API)

- [ ] T037 [US1] Implement `transports/bifrost-http/handlers/organizations.go` for the `/v1/admin/organizations/current` endpoint per [admin-api.openapi.yaml](./contracts/admin-api.openapi.yaml).
- [ ] T038 [US1] Implement `transports/bifrost-http/handlers/workspaces.go` for `/v1/admin/workspaces` list/create/get/patch/delete per contracts.
- [ ] T039 [US1] Extend `transports/config.schema.json` with `enterprise.orgs_workspaces` block (no required fields).

#### UI

- [ ] T040 [P] [US1] Build `ui/src/pages/Workspaces/List.tsx` with `data-testid="workspace-list-*"`.
- [ ] T041 [P] [US1] Build `ui/src/pages/Workspaces/Create.tsx` and `Detail.tsx` with `data-testid` conventions.
- [ ] T042 [P] [US1] Build `ui/src/pages/OrgSettings/General.tsx` (org name, SSO required toggle, default retention).

#### Audit emits

- [ ] T043 [US1] Wire `audit.Emit` calls for `organization.update`, `workspace.create`, `workspace.update`, `workspace.delete` actions.

#### Docs & Changelog

- [ ] T044 [P] [US1] Author `docs/enterprise/organizations-workspaces.mdx`.
- [ ] T045 [P] [US1] Add changelog entries in `framework/changelog.md`, `transports/changelog.md` per Principle IX core→framework→plugins→transport hierarchy.

**Checkpoint (US1)**: Workspaces can be created, scoped, and isolation is E2E-verified.

### Phase 3.2 — User Story 2: Granular RBAC (Priority: P1)

**Goal**: Built-in roles (Owner/Admin/Member/Manager) + custom roles with per-resource scopes.

**Independent Test**: Custom role "ReadOnly Analyst" with `metrics:read, completions:read` only — assigned user can view dashboards, all writes return 403.

#### Tests

- [ ] T046 [P] [US2] Integration test: `framework/tenancy/roles_test.go` validates scope bitmask encoding/decoding and built-in role seeding.
- [ ] T047 [P] [US2] Integration test: `transports/bifrost-http/handlers/roles_test.go` verifies custom role enforcement on write endpoints.
- [ ] T048 [P] [US2] Playwright E2E: `ui/tests/e2e/rbac-custom-role.spec.ts` — exercise the ReadOnly Analyst acceptance scenario end-to-end.

#### Framework

- [ ] T049 [P] [US2] Add scope definitions in `framework/tenancy/scopes.go`: resources (`metrics`, `completions`, `prompts`, `configs`, `guardrails`, `integrations`, `providers`, `models`, `team_mgmt`, `virtual_keys`, `admin_api_keys`, `service_accounts`, `audit_logs`) × verbs (`read`, `write`, `delete`).
- [ ] T050 [P] [US2] Add `framework/tenancy/roles.go` with `CreateRole`, `AssignRole`, `CheckScope(ctx, resource, verb)` helpers consuming `BifrostContextKeyRoleScopes`.
- [ ] T051 [US2] Seed built-in roles (Owner, Admin, Member, Manager) in the migration from T033.

#### Transports

- [ ] T052 [US2] Implement `transports/bifrost-http/handlers/roles.go` for `/v1/admin/roles` list/create + `/v1/admin/users/{id}/roles` assign.
- [ ] T053 [US2] Enforce `RBACEnforceMiddleware` (T028) on every existing admin endpoint with the appropriate `{resource, verb}` scope tag.

#### UI

- [ ] T054 [P] [US2] Build `ui/src/pages/Roles/List.tsx` + `Create.tsx` (built-in roles non-editable; custom roles fully editable).
- [ ] T055 [P] [US2] Build `ui/src/pages/Users/List.tsx` + `AssignRole.tsx` modal.

#### Audit emits

- [ ] T056 [US2] Wire `role.create`, `role.update`, `role.delete`, `user_role_assignment.create`, `user_role_assignment.delete`.

#### Docs & Changelog

- [ ] T057 [P] [US2] Author `docs/enterprise/rbac.mdx`.
- [ ] T058 [P] [US2] Changelog entries in affected modules.

**Checkpoint (US2)**: Roles can be assigned, scopes are enforced in the middleware chain, audit trail populated.

### Phase 3.3 — User Story 3: SSO (SAML 2.0 + OIDC) (Priority: P1)

**Goal**: Corporate IdP integration with auto-provisioning and group-to-role mapping.

**Independent Test**: Configure Okta OIDC; a fresh Okta user authenticates and is auto-provisioned with default role.

#### Tests

- [ ] T059 [P] [US3] Integration test: `framework/idp/oidc_test.go` against a mock OIDC issuer.
- [ ] T060 [P] [US3] Integration test: `framework/idp/saml_test.go` validates signature + clock-skew handling.
- [ ] T061 [P] [US3] Playwright E2E: `ui/tests/e2e/sso-login.spec.ts` — configure OIDC, login, verify session cookie issued.

#### Framework

- [ ] T062 [P] [US3] Create `framework/idp/` module with `go.mod`; OIDC provider via `coreos/go-oidc/v3` in `framework/idp/oidc.go`.
- [ ] T063 [P] [US3] SAML 2.0 provider via `crewjam/saml` in `framework/idp/saml.go`; validate issuer + signature + NotBefore/NotOnOrAfter (R-12).
- [ ] T064 [P] [US3] Session-cookie signer in `framework/idp/session.go` (HS256, 12h TTL, bound to org+user+role-scopes).
- [ ] T065 [US3] Add GORM structs `TableSSOConfig` (sso_config.go) and `TableSCIMConfig` (scim_config.go) in `framework/configstore/tables-enterprise/`. Register migration `E005_sso_scim_configs` in `migrations_enterprise.go`.

#### Plugin

- [ ] T066 [US3] Scaffold `plugins/sso/` with `go.mod`; wire OIDC + SAML callback handling through the enterprise-gate plugin; auto-accept invites (spec FR-006).

#### Transports

- [ ] T067 [US3] Implement `transports/bifrost-http/handlers/sso.go` for `/admin/sso/configs`, `/admin/sso/saml/callback`, `/admin/sso/oidc/callback` per contracts.
- [ ] T068 [US3] Schema: add `enterprise.sso` block to `transports/config.schema.json` with conditional required fields per provider type.

#### UI

- [ ] T069 [P] [US3] Build `ui/src/pages/SSOConfig/Setup.tsx` for OIDC and SAML config entry.
- [ ] T070 [P] [US3] Add SSO login button + redirect handling on `ui/src/pages/Login.tsx`.

#### Audit emits

- [ ] T071 [US3] Wire `sso.config_update`, `sso.login`, `sso.login_failed`, `break_glass.login` (high-severity alert per R-12).

#### Docs & Changelog

- [ ] T072 [P] [US3] Author `docs/enterprise/sso.mdx` with Okta and Entra ID configuration examples.
- [ ] T073 [P] [US3] Changelog entries.

**Checkpoint (US3)**: Okta OIDC + Entra SAML work end-to-end; break-glass path available but disabled by default.

### Phase 3.4 — User Story 4: Audit Logs (Priority: P1)

**Goal**: Filterable, exportable system-wide audit trail.

**Independent Test**: Perform 5 admin actions, filter by actor, verify all 5 appear with before/after; export to CSV.

#### Tests

- [ ] T074 [P] [US4] Integration test: `framework/logstore/audit_test.go` — 10k audit rows, filtered query <2s (SC-011).
- [ ] T075 [P] [US4] Playwright E2E: `ui/tests/e2e/audit-logs.spec.ts` covers filter + export flows per US4 acceptance scenarios.

#### Logstore

- [ ] T076 [US4] Add `TableAuditEntry` GORM struct in `framework/logstore/tables_enterprise.go` with composite indexes per data-model §7. Register migration `E006_audit_entries` in `framework/logstore/migrations_enterprise.go`.
- [ ] T077 [P] [US4] Add `framework/logstore/audit.go` repository with cursor-paginated filtered reads.

#### Transports

- [ ] T078 [US4] Implement `transports/bifrost-http/handlers/audit_logs.go` for `/admin/audit-logs` list + `/export`.

#### UI

- [ ] T079 [P] [US4] Build `ui/src/pages/AuditLogs/List.tsx` with filters (actor, action, resource, time range, outcome).
- [ ] T080 [P] [US4] Add CSV/JSON export button + full-response streamer in `ui/src/pages/AuditLogs/Export.tsx`.

#### Docs & Changelog

- [ ] T081 [P] [US4] Author `docs/enterprise/audit-logs.mdx` with the canonical action-verb list from `contracts/events.md`.
- [ ] T082 [P] [US4] Changelog entries.

**Checkpoint (US4)**: Audit UI is usable; exports work at 10M-row scale.

### Phase 3.5 — User Story 5: Admin API Keys (Priority: P1)

**Goal**: Scoped, expiring, rotatable org-level admin keys for automation.

**Independent Test**: Create admin key with `virtual_keys:write`, create a VK via API → 201; attempt workspace delete → 403.

#### Tests

- [ ] T083 [P] [US5] Integration test: `framework/tenancy/admin_api_keys_test.go` validates argon2id hashing + scope bitmask.
- [ ] T084 [P] [US5] Integration test: `transports/bifrost-http/handlers/admin_api_keys_test.go` — rotation grace period behavior per R-14.
- [ ] T085 [P] [US5] Playwright E2E: `ui/tests/e2e/admin-api-keys.spec.ts`.

#### Migrations

- [ ] T086 [US5] Add `TableAdminAPIKey` GORM struct in `framework/configstore/tables-enterprise/admin_api_key.go` per data-model §1. Register migration `E007_admin_api_keys` in `migrations_enterprise.go`.

#### Framework

- [ ] T087 [P] [US5] Add `framework/tenancy/admin_keys.go`: create, rotate (60s grace per R-14), revoke, list.
- [ ] T088 [P] [US5] Add expiration-alert job in `framework/tenancy/admin_keys_alerts.go` — email at 7d and 1d remaining.

#### Transports

- [ ] T089 [US5] Implement `transports/bifrost-http/handlers/admin_api_keys.go` (`POST` returns full key one time; subsequent reads return prefix only).

#### UI

- [ ] T090 [P] [US5] Build `ui/src/pages/AdminApiKeys/List.tsx` + `Create.tsx` (one-time display modal).
- [ ] T091 [P] [US5] Rotate + revoke buttons with confirmation modals.

#### Audit emits

- [ ] T092 [US5] Wire `admin_api_key.create`, `.rotate`, `.revoke`, expiration alerts.

#### Docs & Changelog

- [ ] T093 [P] [US5] Author `docs/enterprise/admin-api-keys.mdx`.
- [ ] T094 [P] [US5] Changelog entries.

**Checkpoint (Train A complete)**: End-to-end enterprise onboarding (quickstart Part 2) runs against a fresh v1.6.0 deployment. An OSS v1.5.2 deployment upgraded to v1.6.0 with no config changes passes the golden-set replay (SC-001).

---

## Phase 4: Train B — Governance Depth (P2)

Scope: US6 Central Guardrails, US7 PII Redactor, US8 Granular Budgets/Rate Limits, US9 Custom Guardrail Webhooks.

### Phase 4.1 — Foundational: Guardrail Engine

- [ ] T095 Create `framework/guardrails/` module with `go.mod`; define `Engine`, `Policy`, `Verdict` types in `framework/guardrails/engine.go` (research R-04).
- [ ] T096 [P] Implement composition policy evaluator (any_deny / all_pass / majority) in `framework/guardrails/composition.go`.
- [ ] T097 [P] Implement execution-mode runner (sync_parallel / sync_sequential / async) in `framework/guardrails/runner.go`.
- [ ] T098 [US6] Add `TableGuardrailPolicy` GORM struct in `framework/configstore/tables-enterprise/guardrail_policy.go` per data-model §3. Register migration `E008_guardrail_policies` in `migrations_enterprise.go`.
- [ ] T099 [US6] Add `TableGuardrailEvent` GORM struct in `framework/logstore/tables_enterprise.go`. Register migration `E009_guardrail_events` in `framework/logstore/migrations_enterprise.go`.

### Phase 4.2 — User Story 6: Central Guardrails (Priority: P2)

**Goal**: Org/workspace/key-scoped guardrails with deterministic, LLM-based, and partner types.

**Independent Test**: Org-wide prompt-injection guardrail with action=deny; two requests from two workspaces with a known payload both blocked with 446.

#### Tests

- [ ] T100 [P] [US6] Integration test: `plugins/guardrails-central/engine_test.go` — execution order (org → workspace → key) and any-deny-wins.
- [ ] T101 [P] [US6] Integration test: `plugins/guardrails-partners/aporia_test.go` — fail-closed on partner unreachable (mocked at network boundary).
- [ ] T102 [P] [US6] Playwright E2E: `ui/tests/e2e/central-guardrails.spec.ts`.

#### Plugins

- [ ] T103 [US6] Scaffold `plugins/guardrails-central/` with `go.mod`; implement `LLMPlugin.PreLLMHook` + `PostLLMHook` driving `framework/guardrails.Engine`.
- [ ] T104 [P] [US6] Scaffold `plugins/guardrails-partners/` with adapters for Aporia, Pillar, Patronus, SydeLabs, Pangea (one file per partner: `aporia.go`, `pillar.go`, …).
- [ ] T105 [P] [US6] Add deterministic guardrail types (regex, JSON schema, code-language, word/char/sentence count) in `framework/guardrails/deterministic/`.
- [ ] T106 [P] [US6] Add LLM-based guardrail types (prompt_injection, gibberish) in `framework/guardrails/llm/` with configurable model provider.

#### Transports

- [ ] T107 [US6] Implement `transports/bifrost-http/handlers/guardrails_central.go` CRUD per contracts.
- [ ] T108 [US6] Schema: `enterprise.guardrails.central` array with conditionals per `type`.

#### UI

- [ ] T109 [P] [US6] Build `ui/src/pages/Guardrails/List.tsx` + `Create.tsx` + `Edit.tsx` with per-type config forms.

#### Audit emits

- [ ] T110 [US6] Wire `guardrail_policy.create`, `.update`, `.delete`; `guardrail.{allow,deny,warn,retry,fallback,timeout,error}` per execution.

#### Docs & Changelog

- [ ] T111 [P] [US6] Author `docs/enterprise/central-guardrails.mdx`.
- [ ] T112 [P] [US6] Changelog entries.

**Checkpoint (US6)**: Central guardrails enforce, partner integrations function, hot-reload propagates within 30s (R-22).

### Phase 4.3 — User Story 7: PII Anonymizer (Priority: P2)

**Goal**: Org-wide PII redaction before logstore writes; optional redact-before-provider mode.

**Independent Test**: Submit request with SSN/CC/email; stored log carries `<REDACTED:*>` placeholders; downstream provider sees redacted or original per mode; PII-redacted metric incremented.

#### Tests

- [ ] T113 [P] [US7] Integration test: `framework/redaction/engine_test.go` — runs the open-source PII-NER benchmark corpus; asserts ≥98% recall / ≤2% FP rate (SC-007).
- [ ] T114 [P] [US7] Integration test: `plugins/pii-redactor/logstore_hook_test.go` — redacted log payloads.
- [ ] T115 [P] [US7] Playwright E2E: `ui/tests/e2e/pii-redaction.spec.ts`.

#### Framework

- [ ] T116 [US7] Create `framework/redaction/` module with `go.mod`; Pass-1 regex engine in `framework/redaction/patterns.go` (SSN, CC-Luhn, IBAN, phone, email, IPv4/v6, MAC, PHI identifiers) (research R-07).
- [ ] T117 [P] [US7] Pass-2 NER via Presidio inline subprocess in `framework/redaction/ner_presidio.go`.
- [ ] T118 [P] [US7] Pass-2 NER via ONNX runtime in `framework/redaction/ner_onnx.go` (for air-gapped support in US19).
- [ ] T119 [P] [US7] Redaction placeholder formatter + consistent-hash mode in `framework/redaction/replace.go`.

#### Plugin

- [ ] T120 [US7] Scaffold `plugins/pii-redactor/` with `go.mod`; implement hook chain: `HTTPTransportPreHook` for redact-before-provider, `ObservabilityPlugin.Inject` hook into logstore write path for redact-in-logs mode.

#### Transports & Schema

- [ ] T121 [US7] Schema: `enterprise.pii_redaction` block (mode, patterns, custom_regexes, fail_closed) in `config.schema.json`.

#### UI

- [ ] T122 [P] [US7] Build `ui/src/pages/Guardrails/PIISettings.tsx` with pattern toggles + custom regex editor.

#### Audit emits

- [ ] T123 [US7] Wire `pii_redaction.applied` (with count + type), `pii_redaction.fail_closed`.

#### Docs & Changelog

- [ ] T124 [P] [US7] Author `docs/enterprise/pii-redaction.mdx`.
- [ ] T125 [P] [US7] Changelog entries.

**Checkpoint (US7)**: PII benchmark passes; logs are clean; downstream providers see redacted content where configured.

### Phase 4.4 — User Story 8: Granular Budgets & Rate Limits (Priority: P2)

**Goal**: Per-VK monthly/custom budgets + per-minute/hour/day request/token limits with threshold alerts.

**Independent Test**: VK with $100 month cap + 50 req/min limit → drive 60/min traffic → 429 after 50; push cumulative spend past 75% → alert fires.

#### Tests

- [ ] T126 [P] [US8] Integration test: `plugins/governance/budgets_test.go` — Redis-backed counter correctness under concurrent load (5k RPS) per R-13.
- [ ] T127 [P] [US8] Integration test: `plugins/governance/rate_limits_test.go` — sliding-window with 10 buckets per window.
- [ ] T128 [P] [US8] Playwright E2E: `ui/tests/e2e/budgets-rate-limits.spec.ts`.

#### Migrations

- [ ] T129 [US8] Add `TableVirtualKeyBudget` (vk_budget.go) and `TableVirtualKeyRateLimit` (vk_rate_limit.go) GORM structs in `framework/configstore/tables-enterprise/`. Register migration `E010_vk_budgets_rate_limits` in `migrations_enterprise.go`.

#### Plugin (extend governance)

- [ ] T130 [US8] Extend `plugins/governance/` with per-VK budget enforcement in `plugins/governance/budgets.go` (new file; no changes to existing files).
- [ ] T131 [P] [US8] Add per-VK multi-window rate limiter in `plugins/governance/rate_limits.go`.
- [ ] T132 [P] [US8] Redis-backed counter store in `plugins/governance/counters_redis.go` with PostgreSQL fallback (`FOR UPDATE SKIP LOCKED`) in `plugins/governance/counters_postgres.go` (R-13).
- [ ] T133 [P] [US8] Threshold-alert emitter in `plugins/governance/budget_alerts.go` firing to alert_destinations per spec FR-022.

#### Transports & Schema

- [ ] T134 [US8] Extend existing virtual-key endpoints in `transports/bifrost-http/handlers/virtual_keys.go` (extension, not rewrite) with `budget` + `rate_limits` fields per contracts.
- [ ] T135 [US8] Schema: extend `enterprise.governance.virtual_keys` block with optional budget + rate_limits sub-objects.

#### UI

- [ ] T136 [P] [US8] Extend `ui/src/pages/VirtualKeys/Edit.tsx` with Budget and RateLimits form sections.

#### Audit emits

- [ ] T137 [US8] Wire `budget.threshold_reached`, `budget.exceeded`, `rate_limit.exceeded`.

#### Docs & Changelog

- [ ] T138 [P] [US8] Author `docs/enterprise/budgets-rate-limits.mdx`.
- [ ] T139 [P] [US8] Changelog entries in `plugins/governance/changelog.md` + transports.

**Checkpoint (US8)**: Budgets + rate limits enforce globally across instances; alerts fire before exhaustion.

### Phase 4.5 — User Story 9: Custom Guardrail Webhooks (Priority: P2)

**Goal**: Customer-defined HTTPS endpoint as a guardrail with HMAC signing and timeout policy.

**Independent Test**: Register webhook that denies on "ProjectThunder" substring; matching request blocked with returned reason in both response and audit log.

#### Tests

- [ ] T140 [P] [US9] Integration test: `plugins/guardrails-webhook/main_test.go` — HMAC signature generation + verification.
- [ ] T141 [P] [US9] Integration test: timeout policies `fail_open` and `fail_closed` both covered.
- [ ] T142 [P] [US9] Playwright E2E: `ui/tests/e2e/custom-guardrail-webhook.spec.ts`.

#### Plugin

- [ ] T143 [US9] Scaffold `plugins/guardrails-webhook/` with `go.mod`; register as a guardrail type accepted by `framework/guardrails/Engine`.
- [ ] T144 [P] [US9] Implement HMAC signing (`t=<unix>, v1=<sha256>`) in `plugins/guardrails-webhook/sign.go` per contracts/webhook-payloads.md §4.
- [ ] T145 [P] [US9] Implement response parser with optional `redactions[]` application in `plugins/guardrails-webhook/parse.go`.

#### Transports & Schema

- [ ] T146 [US9] Schema: extend `enterprise.guardrails.central[].type` conditionals with `custom_webhook` fields (endpoint_url, signing_secret, timeout_ms, on_timeout_policy).

#### UI

- [ ] T147 [P] [US9] Extend `ui/src/pages/Guardrails/Create.tsx` with the custom-webhook type form.

#### Audit emits

- [ ] T148 [US9] Wire `guardrail.deny`/`warn`/`allow` with `metadata.guardrail_type="custom_webhook"`.

#### Docs & Changelog

- [ ] T149 [P] [US9] Author `docs/enterprise/custom-guardrail-webhook.mdx` (section within central-guardrails MDX).
- [ ] T150 [P] [US9] Changelog entries.

**Checkpoint (Train B complete)**: Org-wide guardrails + PII redaction + per-VK budgets+rate-limits + custom webhooks all live; all actions audited.

---

## Phase 5: Train C — Observability + DX (P3 + P4)

Scope: US10 Alerts, US11 Log Export, US12 Executive Dashboard, US13 Retention, US14 Prompt Library, US15 Playground, US16 Configs, US17 Service Account Keys.

### Phase 5.1 — User Story 10: Alerts & Notifications (Priority: P3)

**Goal**: Threshold alerts (error-rate, latency, cost, feedback-score) to webhook/Slack/email with dedup + recovery.

**Independent Test**: Rule "P95 >3s over 5m" + Slack destination; inject slow traffic for 6m; Slack message lands within 90s of threshold breach.

#### Tests

- [ ] T151 [P] [US10] Integration test: `framework/alerts/evaluator_test.go` — threshold + cooldown + dedup behavior (R-15).
- [ ] T152 [P] [US10] Integration test: `plugins/alerts/dispatcher_test.go` — Slack + webhook payload shapes per contracts.
- [ ] T153 [P] [US10] Playwright E2E: `ui/tests/e2e/alerts.spec.ts`.

#### Migrations

- [ ] T154 [US10] Add `TableAlertRule` (alert_rule.go) and `TableAlertDestination` (alert_destination.go) GORM structs in `framework/configstore/tables-enterprise/`. Register migration `E011_alert_rules` in `migrations_enterprise.go`.
- [ ] T155 [US10] Add `TableAlertEvent` GORM struct in `framework/logstore/tables_enterprise.go`. Register migration `E012_alert_events` in `framework/logstore/migrations_enterprise.go`.

#### Framework

- [ ] T156 [P] [US10] Create `framework/alerts/` module with `go.mod`; rule evaluator (per-rule goroutine, 30s default cadence) in `framework/alerts/evaluator.go`.
- [ ] T157 [P] [US10] Bounded dispatch queue + dedup coalescer in `framework/alerts/dispatcher.go` (R-15).

#### Plugin

- [ ] T158 [US10] Scaffold `plugins/alerts/` with `go.mod`; Slack + webhook + email dispatchers in `plugins/alerts/{slack,webhook,email}.go`.

#### Transports

- [ ] T159 [US10] Implement `transports/bifrost-http/handlers/alerts.go` CRUD on rules + destinations.
- [ ] T160 [US10] Schema: `enterprise.alerts` block (rules[], destinations[]).

#### UI

- [ ] T161 [P] [US10] Build `ui/src/pages/Alerts/Rules.tsx` with metric + operator + threshold + destinations picker.
- [ ] T162 [P] [US10] Build `ui/src/pages/Alerts/Destinations.tsx` (Slack webhook URL, generic webhook URL+secret, email list).

#### Audit emits

- [ ] T163 [US10] Wire `alert.rule_create/update/delete`, `alert.fired`, `alert.resolved`.

#### Docs & Changelog

- [ ] T164 [P] [US10] Author `docs/enterprise/alerts.mdx`.
- [ ] T165 [P] [US10] Changelog entries.

### Phase 5.2 — User Story 11: Log Export (Priority: P3)

**Goal**: Continuous export to S3 / Azure Blob / GCS / MongoDB / OTLP in streaming and scheduled modes.

**Independent Test**: 5-min S3 export config; 1,000 requests; within 10min bucket contains 1,000 records; block egress for 10min → dead-letter on persistent failure.

#### Tests

- [ ] T166 [P] [US11] Integration test: `plugins/logexport/s3_test.go` against minio; reliability under simulated outage (R-08).
- [ ] T167 [P] [US11] Integration test: `plugins/logexport/otlp_test.go` against mock OTLP collector.
- [ ] T168 [P] [US11] Playwright E2E: `ui/tests/e2e/log-exports.spec.ts`.

#### Framework

- [ ] T169 [P] [US11] Create `framework/exportsink/` module with bounded-buffer streaming pipeline + dead-letter store (R-08).

#### Plugin

- [ ] T170 [US11] Scaffold `plugins/logexport/` with `go.mod`; sinks in `plugins/logexport/{s3,azure_blob,gcs,mongodb,otlp}.go`.
- [ ] T171 [P] [US11] Dead-letter persister + retry scheduler in `plugins/logexport/deadletter.go`.

#### Migrations

- [ ] T172 [US11] Add `TableLogExportConfig` GORM struct in `framework/configstore/tables-enterprise/log_export_config.go`. Register migration `E013_log_export_configs` in `migrations_enterprise.go`.
- [ ] T173 [US11] Add `TableExportDeadLetter` GORM struct in `framework/logstore/tables_enterprise.go`. Register migration `E014_export_dead_letters` in `framework/logstore/migrations_enterprise.go`.

#### Transports & Schema

- [ ] T174 [US11] Implement `transports/bifrost-http/handlers/log_exports.go` CRUD.
- [ ] T175 [US11] Schema: `enterprise.log_exports[]` with per-destination conditionals.

#### UI

- [ ] T176 [P] [US11] Build `ui/src/pages/LogExports/List.tsx` + `Create.tsx` (destination picker + schedule).

#### Audit emits

- [ ] T177 [US11] Wire `log_export.config_update`, `.flushed`, `.failed`, `.dead_lettered`.

#### Docs & Changelog

- [ ] T178 [P] [US11] Author `docs/enterprise/log-export.mdx`.
- [ ] T179 [P] [US11] Changelog entries.

### Phase 5.3 — User Story 12: Executive Dashboard (Priority: P3)

**Goal**: Org-level aggregated dashboard (adoption, cost, guardrail saves) with <3s P95 first paint at 10M-log scale.

**Independent Test**: As Org Owner visit Executive Dashboard → 30-day totals + WoW deltas render <3s.

#### Tests

- [ ] T180 [P] [US12] Integration test: `framework/alerts/rollup_test.go` — rollup idempotency under overlapping windows.
- [ ] T181 [P] [US12] Integration test: `transports/bifrost-http/handlers/exec_dashboard_test.go` — 10M-row perf target.
- [ ] T182 [P] [US12] Playwright E2E: `ui/tests/e2e/executive-dashboard.spec.ts` with 403 for workspace-scoped users.

#### Framework

- [ ] T183 [US12] Add `TableExecutiveMetricsHourly` GORM struct in `framework/logstore/tables_enterprise.go` per data-model §7 (composite PK on `organization_id, hour_bucket, metric_name, dimension_key`; PostgreSQL partitioning by month is applied via post-create raw SQL inside the migration's `Migrate` func when the dialect is postgres). Register migration `E015_executive_metrics_hourly` in `framework/logstore/migrations_enterprise.go`.
- [ ] T184 [P] [US12] Create `framework/rollups/` module with hourly rollup job (every 15m, overlapping window) (R-20).

#### Transports

- [ ] T185 [US12] Implement `transports/bifrost-http/handlers/exec_dashboard.go` `/admin/executive/summary`.

#### UI

- [ ] T186 [P] [US12] Build `ui/src/pages/ExecutiveDashboard/Index.tsx` with 8 tiles (requests, cost, users, workspaces, top models, top workspaces, PII saves, guardrail blocks).
- [ ] T187 [P] [US12] Add date-range picker + WoW delta computation client-side.

#### Docs & Changelog

- [ ] T188 [P] [US12] Author `docs/enterprise/executive-dashboard.mdx`.
- [ ] T189 [P] [US12] Changelog entries.

### Phase 5.4 — User Story 13: Retention Policies (Priority: P3)

**Goal**: Per-workspace log + metric retention enforced by daily cron.

**Independent Test**: WS-A retention 30d, WS-B 1y; backdate rows; retention job deletes WS-A only; audit entry captures count.

#### Tests

- [ ] T190 [P] [US13] Integration test: `framework/logstore/retention_test.go` — bounded-batch DELETE, no long-running locks (R-21).
- [ ] T191 [P] [US13] Playwright E2E: `ui/tests/e2e/retention-policies.spec.ts`.

#### Migrations

- [ ] T192 [US13] Add `TableRetentionPolicy` GORM struct in `framework/configstore/tables-enterprise/retention_policy.go`. Register migration `E016_retention_policies` in `migrations_enterprise.go`.

#### Framework

- [ ] T193 [P] [US13] Retention job in `framework/logstore/retention.go` (daily cron, bounded DELETE LIMIT 10k) per R-21.
- [ ] T194 [P] [US13] Audit-retention ceiling enforcement (max(all_policies), capped 7y) in `framework/logstore/audit_retention.go`.

#### Transports & Schema

- [ ] T195 [US13] Implement `transports/bifrost-http/handlers/retention_policies.go`.
- [ ] T196 [US13] Schema: `enterprise.retention` (per-workspace overrides).

#### UI

- [ ] T197 [P] [US13] Build `ui/src/pages/Retention/PerWorkspace.tsx`.

#### Audit emits

- [ ] T198 [US13] Wire `retention.policy_update`, `retention.deletion_run` (with counts).

#### Docs & Changelog

- [ ] T199 [P] [US13] Author `docs/enterprise/retention-policies.mdx`.
- [ ] T200 [P] [US13] Changelog entries.

### Phase 5.5 — User Story 14: Prompt Library (Priority: P4)

**Goal**: Versioned prompts with variables, partials, folders, access-controlled.

**Independent Test**: Create `customer-support-v1` with `{{customer_name}}`; call `/prompts/customer-support-v1/completions` → rendered; edit to v2; v1-pinned caller still gets v1.

#### Tests

- [ ] T201 [P] [US14] Integration test: `framework/promptengine/mustache_test.go` — variables + partials + recursion-depth cap (5).
- [ ] T202 [P] [US14] Integration test: `plugins/prompts/library_test.go` — folder access scope enforcement.
- [ ] T203 [P] [US14] Playwright E2E: `ui/tests/e2e/prompt-library.spec.ts`.

#### Framework

- [ ] T204 [US14] Create `framework/promptengine/` module with Mustache render + partials in `framework/promptengine/render.go` (R-16).
- [ ] T205 [P] [US14] Add multimodal content support (text + image refs) in `framework/promptengine/multimodal.go`.

#### Migrations

- [ ] T206 [US14] Add GORM structs `TablePrompt` (prompt.go), `TablePromptVersion` (prompt_version.go), `TablePromptPartial` (prompt_partial.go), `TablePromptFolderScope` (prompt_folder_scope.go) in `framework/configstore/tables-enterprise/` per data-model §4. Register migration `E017_prompt_library` in `migrations_enterprise.go`.

#### Plugin (extend prompts)

- [ ] T207 [US14] Extend `plugins/prompts/` with library-backed engine in `plugins/prompts/library.go` (new file; existing files unchanged).

#### Transports & Schema

- [ ] T208 [US14] Implement `transports/bifrost-http/handlers/prompt_library.go` (CRUD on prompts + versions) and `/prompts/{id}/render`, `/prompts/{id}/completions` endpoints per contracts.
- [ ] T209 [US14] Schema: `enterprise.prompt_library` (enabled flag; no other required fields).

#### UI

- [ ] T210 [P] [US14] Build `ui/src/pages/PromptLibrary/FolderTree.tsx`.
- [ ] T211 [P] [US14] Build `ui/src/pages/PromptLibrary/Editor.tsx` with variable + partial picker and version history.

#### Audit emits

- [ ] T212 [US14] Wire `prompt.create`, `prompt.version_create`, `prompt.publish`, `prompt.render`.

#### Docs & Changelog

- [ ] T213 [P] [US14] Author `docs/enterprise/prompt-library.mdx`.
- [ ] T214 [P] [US14] Changelog entries.

### Phase 5.6 — User Story 15: Prompt Playground (Priority: P4)

**Goal**: Side-by-side model comparison with latency, tokens, cost per leg; saveable sessions.

**Independent Test**: Open prompt in Playground, select 3 models, run → 3 streaming response cards render with per-leg metrics.

#### Tests

- [ ] T215 [P] [US15] Playwright E2E: `ui/tests/e2e/prompt-playground.spec.ts`.

#### Transports

- [ ] T216 [US15] Implement `transports/bifrost-http/handlers/prompt_playground.go` — parallel fan-out to N models with streaming SSE response per leg.

#### UI

- [ ] T217 [P] [US15] Build `ui/src/pages/PromptPlayground/Session.tsx` with N-model selector, streaming output cards, metrics row.
- [ ] T218 [P] [US15] Session save + share via `ui/src/pages/PromptPlayground/Saved.tsx`.

#### Docs & Changelog

- [ ] T219 [P] [US15] Author `docs/enterprise/prompt-playground.mdx`.
- [ ] T220 [P] [US15] Changelog entries.

### Phase 5.7 — User Story 16: Declarative Configs (Priority: P4)

**Goal**: Named versioned routing documents (primary+fallback+retry+cache+guardrails) addressed via `x-config-id` header.

**Independent Test**: Config with primary=openai/gpt-4o, fallback=claude-sonnet-4.6, cache=semantic-60s; `x-config-id` header → simulated 500 triggers fallback; repeated prompt hits cache.

#### Tests

- [ ] T221 [P] [US16] Integration test: `plugins/canary/config_routing_test.go` — header-attached config routing + fallback + cache.
- [ ] T222 [P] [US16] Playwright E2E: `ui/tests/e2e/configs.spec.ts`.

#### Migrations

- [ ] T223 [US16] Add `TableConfig` (config.go) and `TableConfigVersion` (config_version.go) GORM structs in `framework/configstore/tables-enterprise/`. Register migration `E018_configs` in `migrations_enterprise.go`.

#### Plugin

- [ ] T224 [US16] Scaffold `plugins/canary/` (shared by US16 + US22) with `go.mod`; config-header resolver + router in `plugins/canary/router.go`.

#### Transports & Schema

- [ ] T225 [US16] Implement `transports/bifrost-http/handlers/configs.go` CRUD on configs + versions.
- [ ] T226 [US16] Header-based config lookup in `transports/bifrost-http/lib/middleware.go` (extend, not replace).
- [ ] T227 [US16] Schema: `enterprise.configs.enabled: true` + per-workspace config document validator.

#### UI

- [ ] T228 [P] [US16] Build `ui/src/pages/Configs/Editor.tsx` with JSON editor + validator.
- [ ] T229 [P] [US16] Build `ui/src/pages/Configs/VersionHistory.tsx`.

#### Audit emits

- [ ] T230 [US16] Wire `config.create`, `config.version_create`, `config.publish`.

#### Docs & Changelog

- [ ] T231 [P] [US16] Author `docs/enterprise/configs.mdx`.
- [ ] T232 [P] [US16] Changelog entries.

### Phase 5.8 — User Story 17: Service Account API Keys (Priority: P4)

**Goal**: Workspace-scoped non-user keys with own budgets + rate limits.

**Independent Test**: Create workspace-scoped service-account key with $500/mo budget; use from CI; logs attribute cost to the service account; key has no UI login.

#### Tests

- [ ] T233 [P] [US17] Integration test: `framework/tenancy/service_account_keys_test.go`.
- [ ] T234 [P] [US17] Playwright E2E: `ui/tests/e2e/service-accounts.spec.ts`.

#### Migrations

- [ ] T235 [US17] Add `TableServiceAccountAPIKey` GORM struct in `framework/configstore/tables-enterprise/service_account_api_key.go`. Register migration `E019_service_account_keys` in `migrations_enterprise.go`.

#### Framework

- [ ] T236 [P] [US17] Add `framework/tenancy/service_accounts.go`: create/rotate/revoke scoped keys.

#### Transports & Schema

- [ ] T237 [US17] Implement `transports/bifrost-http/handlers/service_account_keys.go`.
- [ ] T238 [US17] Schema: `enterprise.service_accounts.enabled`.

#### UI

- [ ] T239 [P] [US17] Build `ui/src/pages/ServiceAccounts/List.tsx` + `Create.tsx`.

#### Audit emits

- [ ] T240 [US17] Wire `service_account.create/rotate/revoke`.

#### Docs & Changelog

- [ ] T241 [P] [US17] Author `docs/enterprise/service-account-keys.mdx`.
- [ ] T242 [P] [US17] Changelog entries.

**Checkpoint (Train C complete)**: Full observability + DX depth — alerts, log export, exec dashboard, retention, prompt library + playground, configs, and service accounts all live.

---

## Phase 6: Train D — Security + Ecosystem (P5 + P6)

Scope: US18 BYOK, US19 Air-gapped, US20 SCIM, US21 Terraform, US22 Canary, US23 Data Lake ETL.

### Phase 6.1 — User Story 18: BYOK / Customer-Managed Encryption (Priority: P5)

**Goal**: AWS KMS / Azure Key Vault / GCP KMS encrypt configstore secrets by default; opt-in per-workspace for logstore payloads.

**Independent Test**: Configure AWS KMS BYOK; create VK (encrypted); disable CMK → reads fail "unavailable: key disabled"; re-enable → restores.

#### Tests

- [ ] T243 [P] [US18] Integration test: `framework/kms/aws_test.go` against localstack KMS.
- [ ] T244 [P] [US18] Integration test: `framework/kms/envelope_roundtrip_test.go` — envelope encrypt/decrypt + rotation.
- [ ] T245 [P] [US18] Integration test: `plugins/byok/logstore_encryption_test.go` — opt-in workspace sees encrypted logstore writes; others unaffected.
- [ ] T246 [P] [US18] Playwright E2E: `ui/tests/e2e/byok.spec.ts`.

#### Migrations

- [ ] T247 [US18] Add `TableKMSConfig` GORM struct in `framework/configstore/tables-enterprise/kms_config.go`. Add `PayloadEncryptionEnabled bool` field to the enterprise-owned `TableWorkspace` struct (T033). Register migration `E020_kms_configs_and_workspace_encryption` in `migrations_enterprise.go` (creates kms_configs table + adds the column to ent_workspaces via `migrator.AddColumn`).

#### Framework

- [ ] T248 [US18] Create `framework/kms/` module with `go.mod`; adapters in `framework/kms/{aws,azure,gcp,configstore}.go`.
- [ ] T249 [P] [US18] LRU data-key cache with configurable TTL (default 15m) in `framework/kms/cache.go` (research R-05, SC-009).
- [ ] T250 [P] [US18] KEK rotation handler in `framework/kms/rotation.go` (re-wrap on next write; read falls back to stored `kek_ref`).

#### Plugin

- [ ] T251 [US18] Scaffold `plugins/byok/` with `go.mod`; wire `framework/crypto` to use `framework/kms` when BYOK is active.

#### Transports & Schema

- [ ] T252 [US18] Implement `transports/bifrost-http/handlers/kms.go` for `/admin/kms/configs` PUT/GET.
- [ ] T253 [US18] Schema: `enterprise.kms[]` (provider, key_ref, auth_json, dek_cache_ttl_seconds, applies_to[]).

#### UI

- [ ] T254 [P] [US18] Build `ui/src/pages/KmsSettings/Config.tsx`.
- [ ] T255 [P] [US18] Add workspace-level `payload_encryption_enabled` toggle in `ui/src/pages/Workspaces/Security.tsx`.

#### Audit emits

- [ ] T256 [US18] Wire `kms.config_update`, `kms.rotate`, `kms.revoke_observed`.

#### Docs & Changelog

- [ ] T257 [P] [US18] Author `docs/enterprise/byok.mdx` with runbook for KMS revocation + restoration.
- [ ] T258 [P] [US18] Changelog entries.

### Phase 6.2 — User Story 19: Air-Gapped Deployment (Priority: P5)

**Goal**: Published Helm + Terraform with documented air-gapped profile for MVP scope (US1, US2, US3-OIDC, US4, US5, US18).

**Independent Test**: Helm airgapped profile deploys in egress-restricted K8s cluster; smoke test passes; eBPF confirms zero non-whitelisted outbound.

#### Tests

- [ ] T259 [US19] Smoke test script: `scripts/smoke-airgapped.sh` exercising golden flow in restricted environment.
- [ ] T260 [US19] CI job `airgapped-smoke` (from T005) is gated on this — verify integration.

#### Helm

- [ ] T261 [P] [US19] Create `helm-charts/bifrost/values-airgapped.yaml` with enterprise feature flags set per research R-06.
- [ ] T262 [P] [US19] Add air-gapped mode banner + restricted egress hints in `helm-charts/bifrost/templates/deployment.yaml`.

#### Terraform

- [ ] T263 [P] [US19] Create `terraform/modules/aws/` with KMS + EKS + RDS PostgreSQL + VPC + S3 log bucket.
- [ ] T264 [P] [US19] Create `terraform/modules/azure/` analogous (Azure Key Vault + AKS + PostgreSQL Flexible Server + Blob Storage).
- [ ] T265 [P] [US19] Create `terraform/modules/gcp/` analogous (Cloud KMS + GKE + Cloud SQL + GCS).

#### Docs

- [ ] T266 [P] [US19] Author `docs/enterprise/airgapped-deployment.mdx` with feature-support matrix and explicit NOT-SUPPORTED list.
- [ ] T267 [P] [US19] Changelog entries in helm-charts + terraform modules.

### Phase 6.3 — User Story 20: SCIM 2.0 (Priority: P5)

**Goal**: IdP user-lifecycle automation (RFC 7644 subset).

**Independent Test**: Okta SCIM; deprovision user in Okta → within 10min Bifrost user suspended, sessions terminated, keys revoked, audit entries present.

#### Tests

- [ ] T268 [P] [US20] Integration test: `framework/scim/conformance_test.go` runs a subset of the RFC 7644 conformance suite (R-19).
- [ ] T269 [P] [US20] Integration test: `transports/bifrost-http/handlers/scim_test.go` PATCH suspend → sessions/keys revocation.
- [ ] T270 [P] [US20] Playwright E2E: `ui/tests/e2e/scim-config.spec.ts`.

#### Framework

- [ ] T271 [US20] Create `framework/scim/` module with `go.mod`; filter parser (eq/ne/co/sw/ew/pr/and/or) + PATCH op executor in `framework/scim/{filters,patch}.go`.

#### Transports

- [ ] T272 [US20] Implement `transports/bifrost-http/handlers/scim.go` for `/scim/v2/{Users,Groups,Schemas,ServiceProviderConfig,ResourceTypes}` per contracts.
- [ ] T273 [US20] Schema: `enterprise.scim` block.

#### UI

- [ ] T274 [P] [US20] Build `ui/src/pages/SSOConfig/SCIM.tsx` (bearer token + enable toggle).

#### Audit emits

- [ ] T275 [US20] Wire `scim.user_create`, `scim.user_suspend`, `scim.user_delete`, `scim.group_update`.

#### Docs & Changelog

- [ ] T276 [P] [US20] Author `docs/enterprise/scim.mdx` with Okta + Entra ID setup.
- [ ] T277 [P] [US20] Changelog entries.

### Phase 6.4 — User Story 21: Terraform Provider (Priority: P6)

**Goal**: First-party TF provider on Terraform Registry for workspaces, virtual keys, configs, guardrails, admin API keys, alerts, log exports, SSO.

**Independent Test**: `terraform apply` creating workspace + VKs + config → resources exist; re-apply → plan empty (idempotency).

#### Tests

- [ ] T278 [P] [US21] Acceptance tests in `terraform/providers/bifrost/internal/*_test.go` per `hashicorp/terraform-plugin-framework` v2 conventions (R-18).

#### Provider

- [ ] T279 [US21] Scaffold `terraform/providers/bifrost/` (Go module) with plugin-framework v2.
- [ ] T280 [P] [US21] Implement `bifrost_workspace` resource + `bifrost_workspaces` data source.
- [ ] T281 [P] [US21] Implement `bifrost_virtual_key`, `bifrost_admin_api_key`, `bifrost_service_account_key`.
- [ ] T282 [P] [US21] Implement `bifrost_config`, `bifrost_guardrail`, `bifrost_alert_rule`, `bifrost_log_export`, `bifrost_sso_config`.

#### Docs

- [ ] T283 [P] [US21] Author `docs/enterprise/terraform-provider.mdx` with `terraform init`/`apply` walkthrough.
- [ ] T284 [P] [US21] Publish provider to Terraform Registry (release workflow under `.github/workflows/terraform-release.yml`).
- [ ] T285 [P] [US21] Changelog entries in `terraform/providers/bifrost/CHANGELOG.md`.

### Phase 6.5 — User Story 22: Canary Testing (Priority: P6)

**Goal**: Declarative canary percentage-split routing in Config objects with per-leg comparison reporting.

**Independent Test**: 10/90 canary on a Config; 1,000 requests → ~100 on canary target; comparison report shows both legs' latency/cost/error/feedback.

#### Tests

- [ ] T286 [P] [US22] Integration test: `plugins/canary/canary_test.go` — hash-based split stability (same VK+request-id always lands same leg).
- [ ] T287 [P] [US22] Integration test: canary traffic percentage within ±2% of configured over 1,000 requests (spec acceptance).
- [ ] T288 [P] [US22] Playwright E2E: `ui/tests/e2e/canary-report.spec.ts`.

#### Plugin (extend canary from T224)

- [ ] T289 [US22] Add canary primitive to Config document schema in `plugins/canary/canary.go` (research R-17). Canary lives in Config, NOT in governance routing-chains (per Clarifications Q3).
- [ ] T290 [P] [US22] Hash-stable split by `(virtual_key_id, request_id)` mod 100 in `plugins/canary/split.go`.
- [ ] T291 [P] [US22] Comparison report generator (joins per-leg metrics over collection window) in `plugins/canary/report.go`.

#### Schema

- [ ] T292 [US22] Extend `enterprise.configs` document JSON Schema with the `canary` block per contracts.

#### UI

- [ ] T293 [P] [US22] Build `ui/src/pages/CanaryReport/Index.tsx` showing per-leg latency/cost/error/feedback.

#### Audit emits

- [ ] T294 [US22] Wire `canary.start`, `canary.promoted`, `canary.aborted`.

#### Docs & Changelog

- [ ] T295 [P] [US22] Author `docs/enterprise/canary.mdx`.
- [ ] T296 [P] [US22] Changelog entries.

### Phase 6.6 — User Story 23: Data Lake ETL Export (Priority: P6)

**Goal**: Recurring, curated, schema-stable exports to customer data lakes (S3/BigQuery/Wasabi).

**Independent Test**: Daily `daily-completions-to-bigquery` export; wait 24h; BigQuery table contains day's records with declared schema.

#### Tests

- [ ] T297 [P] [US23] Integration test: `plugins/logexport/datalake_test.go` — Parquet schema stability + partitioned output.
- [ ] T298 [P] [US23] Playwright E2E: `ui/tests/e2e/data-lake-export.spec.ts`.

#### Migrations

- [ ] T299 [US23] Add `TableDataLakeExport` GORM struct in `framework/configstore/tables-enterprise/data_lake_export.go`. Register migration `E021_data_lake_exports` in `migrations_enterprise.go`.

#### Plugin (extend logexport)

- [ ] T300 [P] [US23] Curated-export runner with SQL projection in `plugins/logexport/datalake.go`.
- [ ] T301 [P] [US23] BigQuery sink in `plugins/logexport/bigquery.go`.
- [ ] T302 [P] [US23] Parquet writer in `plugins/logexport/parquet.go`.

#### Transports & Schema

- [ ] T303 [US23] Implement `transports/bifrost-http/handlers/data_lake_exports.go`.
- [ ] T304 [US23] Schema: `enterprise.data_lake_exports[]`.

#### UI

- [ ] T305 [P] [US23] Build `ui/src/pages/LogExports/DataLake.tsx` with schema column-picker + cron editor.

#### Docs & Changelog

- [ ] T306 [P] [US23] Author `docs/enterprise/data-lake-export.mdx`.
- [ ] T307 [P] [US23] Changelog entries.

**Checkpoint (Train D complete)**: All 23 user stories shipped. BYOK + air-gapped + SCIM + Terraform + canary + curated ETL all validated.

---

## Phase 8: Train E — Cloud Commercial (cloud mode only) + Licensing (all modes)

Scope: US24 License Activation (self-hosted), US25 License Expiry
Handling (self-hosted), US26 Per-Org Metering (cloud),
US27 Stripe Billing (cloud), US28 Billing Portal (cloud),
US29 Tier & Plan Management (cloud).

Release target: `v2.0.0` (MAJOR bump because Train E introduces the
cloud commercial layer as a user-visible product surface). v2.0.0
remains backward-compatible for self-hosted deployments — Train E
cloud plugins (`plugins/metering/`, `plugins/billing/`) do not load
in `deployment.mode: selfhosted` or `airgapped`.

### Phase 8.1 — User Story 24: License Activation (Priority: P1, self-hosted only)

**Goal**: Upload a signed license file, verify offline, show entitlements + expiry, gate features by entitlement.

**Independent Test**: Upload license with entitlements [workspaces, rbac, audit]; those 3 work, others return HTTP 402; tamper signature → rejection.

#### Tests

- [ ] T400 [P] [US24] Integration test: `plugins/license/verify_test.go` — Ed25519 signature verification, tampered-payload rejection, clock-skew tolerance.
- [ ] T401 [P] [US24] Integration test: `plugins/license/entitlement_test.go` — `IsEntitled()` gate behavior across the 23 enterprise features.
- [ ] T402 [P] [US24] Playwright E2E: `ui/tests/e2e/license-upload.spec.ts`.

#### Migrations

- [ ] T403 [US24] Add `TableLicense` GORM struct in `framework/configstore/tables-enterprise/license.go` per data-model §7.5. Register migration `E022_licenses` in `migrations_enterprise.go`.

#### Framework / Plugin

- [ ] T404 [P] [US24] Embed an ORDERED ARRAY of valid vendor public keys at build time via `framework/license/publickeys.go` generated by a release script `scripts/build-license-keys.sh` (not singular — supports multi-key rotation per FR-046b / research R-26). Verification tries each embedded key in order.
- [ ] T404a [P] [US24] Scaffold `tools/license-authority/` CLI — Go module with `go.mod`, inputs (customer name, entitlements JSON, expiry, max_users, max_workspaces, max_virtual_keys, contact email), loads Ed25519 private key from a local file path (never hardcoded), outputs a signed JWT license file. Includes a `--dry-run` mode for previewing claims. README with issuance runbook for vendor ops (FR-046a).
- [ ] T404b [P] [US24] Add `framework/license/limits.go` with `CheckLimit(dimension, current_count)` — hard cutoff for `max_workspaces` and `max_virtual_keys` at 100%; 10% soft-grace for `max_users` with warnings at 100%/105%/110% (FR-046c). Wire into resource-creation endpoints (workspaces.go, virtual_keys.go, admin_api_keys.go).
- [ ] T405 [P] [US24] Implement JWT verification (Ed25519 primary, RS256 fallback) in `plugins/license/verify.go`.
- [ ] T406 [P] [US24] Wire `IsEntitled(feature)` checks into each gated feature plugin by adding a 3-line guard at the start of their hook handlers (sibling file in each plugin: `plugins/<name>/license_gate.go` — do NOT edit main.go).
- [ ] T407 [US24] Boot-time license load: read path from `config.license.path` OR fallback to `/etc/bifrost/license.jwt`; refuse to start `selfhosted` mode without a valid license.

#### Transports & Schema

- [ ] T408 [US24] Implement `transports/bifrost-http/handlers/license.go` — GET current license info (sanitized), POST to upload new license.
- [ ] T409 [US24] Schema: add `license` block to `config.schema.enterprise.json` (path, optional).

#### UI

- [ ] T410 [P] [US24] Build `ui/src/pages/License/Status.tsx` showing customer, entitlements, expiry.
- [ ] T411 [P] [US24] Build `ui/src/pages/License/Upload.tsx` with drag-drop license file upload.

#### Audit emits

- [ ] T412 [US24] Wire `license.activated`, `license.rejected`, `license.entitlement_denied`.

#### Docs & Changelog

- [ ] T413 [P] [US24] Author `docs/enterprise/licensing.mdx` — license file format, public-key bootstrapping, activation flow.
- [ ] T414 [P] [US24] Changelog entries.

### Phase 8.2 — User Story 25: License Expiry & Grace Period (Priority: P2, self-hosted only)

**Goal**: 30/7/1-day warning, 14-day grace post-expiry, graceful degradation to OSS-only after grace.

**Independent Test**: License expiring in 2 days → banners appear. Time-travel past expiry → grace mode + daily audit. Past grace → 402 on enterprise endpoints, OSS endpoints unchanged, existing data readable.

#### Tests

- [ ] T415 [P] [US25] Integration test: `plugins/license/expiry_test.go` — time-travel fixture validates all 4 states (normal / warning / grace / expired).
- [ ] T416 [P] [US25] Playwright E2E: `ui/tests/e2e/license-expiry-banner.spec.ts`.

#### Framework / Plugin

- [ ] T417 [P] [US25] Expiry state machine in `plugins/license/state.go` (normal → warning → grace → expired).
- [ ] T418 [P] [US25] Daily audit emitter (runs via cron loop) in `plugins/license/daily_audit.go`.
- [ ] T419 [P] [US25] Read-only-mode enforcement in `plugins/license/readonly_gate.go` — blocks admin writes while allowing OSS completions + enterprise reads.
- [ ] T420 [US25] Expiry alert emission via existing alerts framework (T156) at 30, 7, 1 days.

#### UI

- [ ] T421 [P] [US25] Global UI banner component in `ui/src/components/LicenseBanner.tsx` consumed by the layout shell.

#### Audit emits

- [ ] T422 [US25] Wire `license.warning_threshold_crossed`, `license.grace_period_active`, `license.expired`.

#### Docs & Changelog

- [ ] T423 [P] [US25] Extend `docs/enterprise/licensing.mdx` with expiry / grace / renewal runbook.
- [ ] T424 [P] [US25] Changelog entries.

### Phase 8.3 — User Story 26: Per-Organization Usage Metering (Priority: P1, cloud only)

**Goal**: Accurate per-org usage attribution with daily + monthly rollups.

**Independent Test**: 10k requests across 3 orgs → per-org rollups match replay-computed reference within 0.1%.

#### Tests

- [ ] T425 [P] [US26] Integration test: `plugins/metering/accuracy_test.go` — 10k replay, ≥99.9% accuracy (SC-018).
- [ ] T426 [P] [US26] Integration test: `framework/metering/rollup_test.go` — 15-minute flush + idempotency.

#### Migrations

- [ ] T427 [US26] Add `TableMeterDaily` and `TableMeterMonthly` GORM structs in `framework/logstore/tables_enterprise.go`. Register migration `E023_meter_daily_monthly` in `framework/logstore/migrations_enterprise.go` (PostgreSQL monthly partitioning applied via raw SQL inside the migration when dialect is postgres).

#### Framework / Plugin

- [ ] T428 [US26] Create `framework/metering/` module with per-org Redis counter + PostgreSQL rollup.
- [ ] T429 [US26] Scaffold `plugins/metering/` (cloud-mode only) implementing `ObservabilityPlugin.Inject` to accumulate per-request usage.
- [ ] T430 [P] [US26] 15-minute rollup job in `framework/metering/rollup.go` (same cadence pattern as executive dashboard rollup R-20).
- [ ] T431 [P] [US26] Monthly rollup + freeze job in `framework/metering/monthly.go`.

#### Transports

- [ ] T432 [US26] Implement `transports/bifrost-http/handlers/metering.go` — GET per-org current usage + history.

#### UI

- [ ] T433 [P] [US26] Build `ui/src/pages/Billing/Usage.tsx` showing per-period usage chart (requests, tokens, cost).

#### Audit emits

- [ ] T434 [US26] Wire `meter.rollup_complete`, `meter.rollup_failed`.

#### Docs & Changelog

- [ ] T435 [P] [US26] Author `docs/enterprise/usage-metering.mdx`.
- [ ] T436 [P] [US26] Changelog entries.

### Phase 8.4 — User Story 27: Stripe Subscription & Billing (Priority: P1, cloud only)

**Goal**: Stripe integration for subscriptions + metered billing + dunning + webhooks.

**Independent Test**: Prod tier + 150k requests → $49 + $4.50 overage charge at period close; failed payment → dunning state + 14-day grace.

#### Tests

- [ ] T437 [P] [US27] Integration test: `plugins/billing/stripe_test.go` against Stripe test mode — subscription create, usage record, webhook signature verification, idempotent replay.
- [ ] T438 [P] [US27] Integration test: `plugins/billing/dunning_test.go` — failed payment flow + 14-day grace enforcement.

#### Migrations

- [ ] T439 [US27] Add `TableBillingAccount` GORM struct in `framework/configstore/tables-enterprise/billing_account.go` (register `E024_billing_accounts`). Add `TableStripeWebhookEvent` GORM struct in `framework/logstore/tables_enterprise.go` (register `E025_stripe_webhook_events` in `framework/logstore/migrations_enterprise.go`).

#### Plugin

- [ ] T440 [US27] Scaffold `plugins/billing/` (cloud-mode only) with `go.mod`; Stripe SDK integration in `plugins/billing/stripe.go`.
- [ ] T441 [P] [US27] Usage push (cycle close) in `plugins/billing/push.go` with idempotency keys.
- [ ] T442 [P] [US27] Webhook handler signature verification + idempotent replay in `plugins/billing/webhook.go`.
- [ ] T443 [P] [US27] Dunning state machine in `plugins/billing/dunning.go` (14-day read-only grace on payment_failed).

#### Transports

- [ ] T444 [US27] Implement `transports/bifrost-http/handlers/stripe_webhook.go` — POST `/v1/webhooks/stripe` with signature check and event dispatch.
- [ ] T445 [US27] Schema: `enterprise.billing.stripe` block (api_key_env, webhook_secret_env, tier price IDs).

#### Audit emits

- [ ] T446 [US27] Wire `subscription.created/updated/canceled`, `payment.succeeded/failed`, `dunning.entered/cleared`.

#### Docs & Changelog

- [ ] T447 [P] [US27] Author `docs/enterprise/billing.mdx` with Stripe setup + webhook configuration runbook.
- [ ] T448 [P] [US27] Changelog entries.

### Phase 8.5 — User Story 28: Customer Billing Portal (Priority: P2, cloud only)

**Goal**: Customer-facing portal for usage + payment method + invoice download.

**Independent Test**: Customer admin views current-cycle usage, updates payment method via Stripe hosted flow, downloads 3 past invoices as PDFs.

#### Tests

- [ ] T449 [P] [US28] Playwright E2E: `ui/tests/e2e/billing-portal.spec.ts`.

#### Transports

- [ ] T450 [US28] Implement `transports/bifrost-http/handlers/billing_portal.go` — GET current-cycle + projections, POST update-payment-method (returns Stripe-hosted redirect URL), GET invoice list, GET invoice PDF.

#### UI

- [ ] T451 [P] [US28] Build `ui/src/pages/Billing/Overview.tsx` with usage, projected cost, payment method on file.
- [ ] T452 [P] [US28] Build `ui/src/pages/Billing/Invoices.tsx` with PDF download links.
- [ ] T453 [P] [US28] Integrate Stripe's hosted Customer Portal for payment-method updates.

#### Docs & Changelog

- [ ] T454 [P] [US28] Extend `docs/enterprise/billing.mdx` with customer portal walkthrough.
- [ ] T455 [P] [US28] Changelog entries.

### Phase 8.6 — User Story 29: Tier & Plan Management (Priority: P2, cloud only)

**Goal**: Self-service Dev → Prod upgrade (and downgrade at period end) with <60s feature activation.

**Independent Test**: Dev customer clicks Upgrade, completes Stripe Checkout, feature set (retention, rate limits, guardrails) updates within 60s.

#### Tests

- [ ] T456 [P] [US29] Integration test: `plugins/billing/tier_test.go` — tier transition + hot-reload feature flag propagation.
- [ ] T457 [P] [US29] Playwright E2E: `ui/tests/e2e/tier-upgrade.spec.ts` (SC-019: <5min end-to-end).

#### Plugin

- [ ] T458 [US29] Tier enforcement in `plugins/billing/tier.go` — emits tier config into `BifrostContext` for consumption by rate-limit, retention, and guardrail plugins. Enforcement matches the plan.md Tier Feature Matrix exactly (FR-050a). Each gated feature's plugin consults the tier context and returns HTTP 402 on a feature not in the caller's tier.
- [ ] T459 [P] [US29] Downgrade-at-period-end scheduler in `plugins/billing/downgrade_scheduler.go`.

#### Transports

- [ ] T460 [US29] Implement `transports/bifrost-http/handlers/tier.go` — GET current tier + limits, POST upgrade (returns Stripe Checkout URL), POST downgrade.

#### UI

- [ ] T461 [P] [US29] Build `ui/src/pages/Billing/UpgradeTier.tsx` with tier comparison cards + Stripe Checkout redirect.
- [ ] T462 [P] [US29] Enterprise-tier request form in `ui/src/pages/Billing/EnterpriseInquiry.tsx` (no self-service, routes to sales workflow).

#### Audit emits

- [ ] T463 [US29] Wire `tier.upgrade_requested`, `tier.activated`, `tier.downgrade_scheduled`.

#### Docs & Changelog

- [ ] T464 [P] [US29] Author `docs/enterprise/tiers-and-plans.mdx`.
- [ ] T465 [P] [US29] Changelog entries.

**Checkpoint (Train E complete)**: Self-hosted deployments enforce license entitlements and handle expiry gracefully. Cloud deployments charge correctly via Stripe, expose a customer portal, and support self-service tier upgrades. Self-hosted customers upgrading from v1.9 to v2.0 see no behavior change.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Program-wide cleanup, compliance validation, and release readiness.

- [ ] T308 [P] Run program-wide constitution audit: every plugin has OTEL + Prometheus + audit (R-09 CI must pass clean); every table has tenancy columns; every handler has RBAC enforcement.
- [ ] T309 [P] Program-wide performance validation: enable full P1+P2 stack, sustain 5k RPS for 1h, validate SC-005 (≤1ms p50 / ≤3ms p99 added latency per feature).
- [ ] T310 [P] Program-wide security scan: run the secrets-in-logs scanner against a 24h log sample and confirm zero plaintext (SC-013).
- [ ] T311 [P] Upgrade-path validation: install v1.5.2, populate with a customer fixture, upgrade to v1.6.0 with no config changes, replay golden set, confirm byte-identical behavior (SC-001).
- [ ] T312 [P] End-to-end quickstart validation: operator follows [quickstart.md](./quickstart.md) and confirms total elapsed time ≤30 minutes (SC-002).
- [ ] T313 [P] Parity traceability audit: verify every user story (US1–US23) has its Portkey URL cited in spec.md AND at least one corresponding MDX docs page (SC-003).
- [ ] T314 [P] Compliance mapping page: author `docs/enterprise/compliance.mdx` listing SOC 2 / ISO 27001 / HIPAA / GDPR controls and the Bifrost features that address them.
- [ ] T315 [P] Per-train release tasks: pre-release top-level `changelog.md` entry for each shipping version (one entry per v1.6.0 / v1.7.0 / v1.8.0 / v1.9.0 / v2.0.0) summarizing the trains it includes.
- [ ] T316 [P] `AGENTS.md` addendum documenting enterprise plugin patterns (tenancy, audit emit, observability-completeness).
- [ ] T317 Release coordination: execute the changelog-writer skill via `/changelog-writer` to bump versions in the core→framework→plugins→transport hierarchy.
- [ ] T318 `.github/pull_request_template.md` Constitution checklist is filled in on the release PR.
- [ ] T319 Per-train tag + publish: at the close of each train, tag the corresponding version (`v1.6.0` for Train A, `v1.7.0` for B, `v1.8.0` for C, `v1.9.0` for D, `v2.0.0` for E) on `main` after the release PR merges; publish Helm chart + Terraform provider from the tag.
- [ ] T466 [P] Program-wide validation: SC-016 (<100ms offline license verification on boot) + SC-017 (expiry/grace state machine) + SC-018 (≥99.9% metering accuracy) + SC-019 (<5min tier upgrade end-to-end).
- [ ] T467 [P] Deployment-mode matrix test: boot in each of `cloud` / `selfhosted` / `airgapped` modes; confirm the expected plugin set loads in each (metering/billing absent in non-cloud; license required in non-cloud; phone-home disabled in non-cloud).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)** has no dependencies — start immediately.
- **Phase 2 (Foundational)** depends on Phase 1 completion and **BLOCKS ALL USER STORIES**.
- **Phase 3 (Train A)** depends on Phase 2 completion. Within Train A, US1 delivers the synthetic-default-org seeding that every later story assumes — so US1 should land before US2/US3/US4/US5 finalize (those stories can start in parallel but cannot merge without US1's migration).
- **Phase 4 (Train B)** depends on Train A completion (budgets + guardrails scope by org/workspace/virtual-key, which Train A introduces).
- **Phase 5 (Train C)** depends on Train A (tenant scoping) + Train B (guardrail events feed alerts + export).
- **Phase 6 (Train D)** depends on Trains A–C (BYOK encrypts already-existing entities; canary extends Configs from US16; ETL extends export from US11).
- **Phase 7 (Polish)** depends on all trains complete.

### User Story Dependencies Inside Each Train

- **Train A**: US1 is a prerequisite for US2 (roles reference workspaces). US3/US4/US5 can be developed in parallel once US1's migration lands.
- **Train B**: US6 must land before US9 (webhook guardrails extend the central engine). US7 is independent. US8 is independent.
- **Train C**: All stories are independent of each other; each depends only on Train B outputs.
- **Train D**: US19 depends on US18 (air-gapped profile enables BYOK). US22 depends on US16 from Train C. US23 depends on US11 from Train C. US20 and US21 are independent.

### Within Each User Story

1. **Tests first** (integration + Playwright) — write and confirm they fail against the existing codebase.
2. **Migrations** (configstore/logstore schema changes).
3. **Framework additions** (primitive types, helpers).
4. **Plugin implementation**.
5. **Transport handler + schema update**.
6. **UI pages + Playwright tests green**.
7. **Audit emits wired**.
8. **Docs + changelog entries**.

### Parallel Opportunities

- All Phase 1 tasks marked [P] run in parallel.
- All Phase 2 sub-sections (tenancy, crypto, audit sink, enterprise-gate, middleware) can progress in parallel by different developers; they merge before any US begins.
- Within a user story, all [P]-tagged implementation tasks (typically framework + UI + docs) run in parallel after migrations land.
- Across a train, user stories can be staffed in parallel by different developers once foundational migrations from the first story in the train land.
- Across trains, Train B and Train C can both start the moment Train A's migrations are in; Train D can start the moment Train C's Configs (US16) and Log Export (US11) are operational.

### Parallel Example: Train A User Story 1

```bash
# Once Phase 2 is complete and T033 migration is merged, these run in parallel:
Task: T030 Integration test orgs/workspaces repo
Task: T031 Integration test workspaces handler 403
Task: T032 Playwright E2E two-workspace isolation
Task: T035 framework/tenancy/orgs.go
Task: T036 framework/tenancy/workspaces.go
Task: T040 ui/src/pages/Workspaces/List.tsx
Task: T041 ui/src/pages/Workspaces/Create.tsx + Detail.tsx
Task: T042 ui/src/pages/OrgSettings/General.tsx
Task: T044 docs/enterprise/organizations-workspaces.mdx
```

### Parallel Example: Train B User Story 6

```bash
# Phase 4.1 (Guardrail Engine) is complete → these run in parallel:
Task: T100 Integration test engine ordering
Task: T101 Integration test partner fail-closed
Task: T102 Playwright E2E central guardrails
Task: T104 Partner adapters (5 files — can be further parallelized per partner)
Task: T105 Deterministic types (regex/json/code/count)
Task: T106 LLM-based types (prompt_injection, gibberish)
Task: T109 UI pages Guardrails/*.tsx
Task: T111 docs/enterprise/central-guardrails.mdx
```

---

## Implementation Strategy

### MVP First (Train A — User Story 1 only)

1. Phase 1 Setup complete.
2. Phase 2 Foundational complete.
3. Phase 3.1 (US1) complete.
4. STOP and VALIDATE: SC-001 upgrade-path + SC-002 partial (orgs/workspaces only) + audit entries present.
5. Deploy MVP: workspaces are live, isolation holds, audit runs. Ship as `v1.6.0-preview`.

### Incremental Delivery (Full Bifrost v1.6 through v1.9)

| Release | Trains | What it delivers |
|---------|--------|-------------------|
| v1.6.0 | A | Tenancy + identity + audit (procurement-gate essentials) |
| v1.7.0 | A + B | + Central guardrails, PII redaction, granular budgets/rate limits |
| v1.8.0 | A + B + C | + Alerts, log export, exec dashboard, retention, prompts, configs |
| v1.9.0 | A + B + C + D | + BYOK, air-gapped, SCIM, Terraform, canary, curated ETL |

Each release is backward-compatible; enabling later trains is a config change, not a binary swap.

### Parallel Team Strategy

With 4 engineer-pairs (or one engineer working Train A → D sequentially):

1. **Pair α** — Train A (Tenancy + Identity). Sets the foundation; no other pair can start stories until Phase 2 is in.
2. **Pair β** — Train B (Governance Depth). Starts once Phase 2 lands; depends only on tenancy primitive.
3. **Pair γ** — Train C (Observability + DX). Starts after Train B lands so alerts + export can consume guardrail events.
4. **Pair δ** — Train D (Security + Ecosystem). Starts after Train C lands so BYOK / canary / ETL have their underlying substrates.

Within each pair, stories are delivered in priority-order inside their train; checkpoint demos happen at each train exit.

---

## Notes

- [P] tasks touch different files and have no incomplete prerequisites.
- [Story] tags map tasks to the user stories in [spec.md](./spec.md) for traceability.
- Every enterprise feature PR bundles: schema update + docs MDX + module changelog entry + UI strings (Principle IX) — these are line items in the task list, NOT optional polish.
- Integration tests MUST hit real PostgreSQL, real vectorstore, real object storage (localstack/minio OK). No mocks at the integration tier (Principle VIII).
- Every new plugin must register OTEL + Prometheus + audit emits; the CI completeness check (T003) blocks merges that don't.
- `core/**` is UNTOUCHED for every task above; T001 enforces this at CI.
- Commit cadence: one commit per task or per logical task group; commit messages carry the task ID (e.g., `T033: tables-enterprise + E004_orgs_workspaces_users_roles migration`).
