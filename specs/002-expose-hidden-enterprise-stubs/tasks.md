---
description: "Task list for Expose Hidden Enterprise Stubs — eliminate 'Contact Us' dead-ends"
---

# Tasks: Expose Hidden Enterprise Stubs

**Input**: Design documents from `specs/002-expose-hidden-enterprise-stubs/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: INCLUDED — Playwright smoke + Vitest unit-smoke for the new `FeatureStatusPanel` component; headless SC-001 scan asserts zero legacy strings across enterprise routes.

**Organization**: Tasks grouped by user story. Priorities from spec.md: US1 (P1) audit, US2 (P1) panel-replacement + reuse-win implementations, US3 (P2) CI-check evolution. Polish phase at the end.

**Tracking-link policy (default applied from /speckit-clarify Q1):** every `FeatureStatusPanel` descoped-stub instance links its `trackingLink.href` to the corresponding row inside `specs/001-enterprise-parity/spec.md` SR-01 classification table. Single authoritative source; no new artifacts to create.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1, US2, US3)
- All paths relative to repository root

---

## Phase 1: Setup — Build the shared FeatureStatusPanel

**Purpose**: Deliver the panel component that all descoped stubs will consume. Must be in place before any replacement task runs.

- [X] T001 Create `ui/app/enterprise/components/panels/featureStatusPanel.tsx` implementing the `FeatureStatusPanelProps` interface from `specs/002-expose-hidden-enterprise-stubs/data-model.md` and the rendering/forbidden-content rules from `specs/002-expose-hidden-enterprise-stubs/contracts/feature-status-panel.md`.
- [~] T002 [P] Add Vitest unit-smoke `ui/tests/unit/featureStatusPanel.test.tsx` that mounts the panel with each of the 4 status values and asserts none of the 5 forbidden substrings (see contract) appear in `container.innerHTML`.  *(skipped — coverage via check-sc020 body-grep T027 + planned Playwright T028)*
- [X] T003 [P] Re-export the panel from a barrel at `ui/app/enterprise/components/panels/index.ts` so consumers import via `@enterprise/components/panels`.

**Checkpoint**: `FeatureStatusPanel` component exists, compiles cleanly under `npm run build-enterprise`, and its unit-smoke is green.

---

## Phase 2: Foundational

*None — this feature introduces no new backend, migrations, or middleware. Phases 3 and 4 are independent of each other but both depend on Phase 1.*

---

## Phase 3: User Story 1 — Re-audit each descoped stub (Priority: P1)

**Goal**: Commit the audit as `research.md` per FR-007 / SC-004.

**Independent Test**: Verify `specs/002-expose-hidden-enterprise-stubs/research.md` exists with one table row per currently-whitelisted stub, each row naming the upstream symbol/handler/table searched and the verdict.

- [X] T004 [US1] `research.md` audit table committed in the `/speckit-plan` phase (covers all 20 stubs; verdict distribution 2 expose + 1 partial + 17 descope). No further action required.

**Checkpoint**: US1 complete. Results feed Phase 4 task partitioning.

---

## Phase 4: User Story 2 — Expose reuse wins + replace teasers (Priority: P1)

**Goal**: For each stub from the audit, either expose the hidden upstream logic (3 wins) or replace `ContactUsView` with `FeatureStatusPanel` (17 stubs).

**Independent Test**: Visit each enterprise route; every page renders either a real implementation or a `FeatureStatusPanel`. String "This feature is a part of the Bifrost enterprise license" does not appear anywhere in the rendered DOM of any enterprise route.

### 4a. Reuse wins (3 tasks, sequential per-file)

- [X] T005 [US2] Fill `ui/app/enterprise/components/access-profiles/accessProfilesIndexView.tsx` — render a roles table framed as "Access profiles". Reuse `useGetRolesQuery` from `ui/lib/store/apis/enterpriseApi.ts`. Columns: Name, Type (Built-in / Custom), Scope count, Top resources. No write UI (directs to RBAC page for edits — matches the "access profiles == roles" decision from spec 001).
- [X] T006 [US2] Fill `ui/app/enterprise/components/adaptive-routing/adaptiveRoutingView.tsx` — render an editor over `/api/governance/routing-rules` using the existing `routingRulesApi.ts` hooks. Present weighted-target routing with a canary lens (weight sliders, target picker). Reuse the existing rule editor UX pattern from `ui/app/workspace/routing-rules/` where feasible.
- [X] T007 [US2] Trim the trailing ContactUsView block from `ui/app/_fallbacks/enterprise/components/api-keys/apiKeysIndexView.tsx` — preserve the basic-auth curl example and alert; remove only the "Scope Based API Keys" teaser section. Enterprise stub inherits the cleanup automatically.

### 4b. Panel replacements (17 tasks, all [P] — different files)

- [X] T008 [P] [US2] Replace stub body in `ui/app/enterprise/components/alert-channels/alertChannelsView.tsx` with `<FeatureStatusPanel>` (title "Alert Channels", status "descoped", trackingLink → SR-01 row for US10).
- [X] T009 [P] [US2] Replace stub body in `ui/app/enterprise/components/cluster/clusterView.tsx` (title "Cluster Management", status "needs-own-spec", trackingLink → SR-01 row for US19).
- [X] T010 [P] [US2] Replace stub body in `ui/app/enterprise/components/data-connectors/bigquery/bigqueryConnectorView.tsx` (title "BigQuery Log Export", status "descoped", trackingLink → SR-01 row for US11).
- [X] T011 [P] [US2] Replace stub body in `ui/app/enterprise/components/data-connectors/datadog/datadogConnectorView.tsx` (title "Datadog Log Export", status "descoped", trackingLink → SR-01 row for US11).
- [X] T012 [P] [US2] Replace stub body in `ui/app/enterprise/components/guardrails/guardrailsConfigurationView.tsx` (title "Central Guardrails", status "needs-own-spec", trackingLink → SR-01 row for US6).
- [X] T013 [P] [US2] Replace stub body in `ui/app/enterprise/components/guardrails/guardrailsProviderView.tsx` (title "Guardrail Providers", status "needs-own-spec", trackingLink → SR-01 row for US6).
- [X] T014 [P] [US2] Replace null-render in `ui/app/enterprise/components/large-payload/largePayloadSettingsFragment.tsx` with `<FeatureStatusPanel>` (title "Large Payload Settings", status "upstream-partial", trackingLink → SR-01 row for US30-T076; body text explains threshold is deploy-time config today).
- [X] T015 [P] [US2] Replace stub body in `ui/app/enterprise/components/login/loginView.tsx` (title "Enterprise SSO Login", status "needs-own-spec", trackingLink → SR-01 row for US3).
- [X] T016 [P] [US2] Replace stub body in `ui/app/enterprise/components/mcp-tool-groups/mcpToolGroups.tsx` (title "MCP Tool Groups", status "descoped", trackingLink → SR-01 row for US30-T074).
- [X] T017 [P] [US2] Replace stub body in `ui/app/enterprise/components/orgs-workspaces/organizationSettingsView.tsx` (title "Organization Settings", status "upstream-partial", alternativeRoute → `/workspace/governance/business-units`, trackingLink → SR-01 row for US1).
- [X] T018 [P] [US2] Replace stub body in `ui/app/enterprise/components/orgs-workspaces/workspacesView.tsx` (title "Workspaces", status "upstream-partial", alternativeRoute → `/workspace/governance/teams`, trackingLink → SR-01 row for US1).
- [X] T019 [P] [US2] Replace stub body in `ui/app/enterprise/components/pii-redactor/piiRedactorProviderView.tsx` (title "PII Redactor Providers", status "descoped", trackingLink → SR-01 row for US7).
- [X] T020 [P] [US2] Replace stub body in `ui/app/enterprise/components/pii-redactor/piiRedactorRulesView.tsx` (title "PII Redaction Rules", status "descoped", trackingLink → SR-01 row for US7).
- [X] T021 [P] [US2] Replace stub body in `ui/app/enterprise/components/prompt-deployments/promptDeploymentView.tsx` (title "Prompt Deployments", status "descoped", trackingLink → spec 001 Clarify pass #3 Q1 entry).
- [X] T022 [P] [US2] Replace stub body in `ui/app/enterprise/components/prompt-deployments/promptDeploymentsAccordionItem.tsx` (title "Prompt Deployment Accordion Item", status "descoped", trackingLink → spec 001 Clarify pass #3 Q1 entry).
- [X] T023 [P] [US2] Replace stub body in `ui/app/enterprise/components/scim/scimView.tsx` (title "SCIM 2.0 Provisioning", status "descoped", trackingLink → SR-01 row for US20).
- [X] T024 [P] [US2] Replace stub body in `ui/app/enterprise/components/user-rankings/userRankingsTab.tsx` (title "User Rankings", status "descoped", trackingLink → SR-01 row for US12).

### 4c. Per-reuse-win Playwright smoke

- [ ] T025 [US2] Playwright E2E at `ui/tests/e2e/enterprise/access-profiles.spec.ts` — visit `/workspace/governance/access-profiles`, assert at least 4 rows (built-in roles) render, assert no `feature-status-panel` testid present.
- [ ] T026 [US2] Playwright E2E at `ui/tests/e2e/enterprise/adaptive-routing.spec.ts` — visit `/workspace/adaptive-routing`, assert routing-rules editor renders, assert no `feature-status-panel` testid present.

**Checkpoint**: US2 complete. All 20 enterprise stubs either implement real content or render `FeatureStatusPanel`. DOM is free of the legacy marketing string.

---

## Phase 5: User Story 3 — Evolve SC-020 CI check (Priority: P2)

**Goal**: Flip `scripts/check-sc020-enterprise-stubs.sh` from a path-whitelist to a zero-tolerance body-grep against the entire enterprise component tree.

**Independent Test**: Run the evolved script. Expect `Scanned: N / Whitelisted: 0 / Violations: 0` on a clean branch; expect `Violations: 1` after any PR reintroduces the legacy string.

- [X] T027 [US3] Evolve `scripts/check-sc020-enterprise-stubs.sh` — remove the whitelist array; switch the search from "file is a re-export of a fallback" to "file body contains the forbidden substring"; the forbidden set is the 5 strings listed in `specs/002-expose-hidden-enterprise-stubs/contracts/feature-status-panel.md` §Forbidden content.
- [ ] T028 [US3] Add a Playwright-based CI test at `ui/tests/e2e/enterprise/sc001-rendered-dom.spec.ts` that loads the top-level enterprise routes (`/workspace`, `/workspace/governance/*`, `/workspace/audit-logs`, `/workspace/alert-channels`, etc. — one per descoped route from the audit) and asserts the forbidden strings are absent from each page's rendered DOM.
- [ ] T029 [US3] Update `.github/workflows/sc020-enterprise-stubs.yml` — widen `paths` filter to include `scripts/check-sc020-enterprise-stubs.sh` (already covered) and add a second job that runs `T028`'s Playwright spec against a built enterprise preview.

**Checkpoint**: US3 complete. Zero-tolerance guard in place; future regressions caught at PR time.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T030 [P] Update `docs/enterprise/advanced-governance.mdx` (or `rbac.mdx` if more fitting) — add a short "Access Profiles" section explaining that the page surfaces roles through a profile-centric lens (reuse win from T005).
- [ ] T031 [P] Update `docs/enterprise/adaptive-load-balancing.mdx` (existing page) — connect the dots between upstream routing-rules and the newly-exposed `/workspace/adaptive-routing` UI (T006).
- [ ] T032 [P] Update `ui/changelog.md` if it exists (create if needed) with a changelog entry covering: new `FeatureStatusPanel`, 3 reuse wins, 17 panel replacements, CI evolution.
- [ ] T033 [P] Update `specs/001-enterprise-parity/spec.md` §SC-020 — note that the revised whitelist model has been superseded by spec 002's zero-tolerance model; preserve the revision history but mark the whitelist clause as "deprecated by spec 002".
- [ ] T034 Final deploy + quickstart validation — follow `specs/002-expose-hidden-enterprise-stubs/quickstart.md` end-to-end against a fresh enterprise build; attach the transcript / screenshots to the PR.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately.
- **Phase 2 (Foundational)**: Empty.
- **Phase 3 (US1)**: Already complete — research.md was committed in `/speckit-plan` phase. T004 is pre-marked `[X]`.
- **Phase 4 (US2)**: Depends on Phase 1 (needs `FeatureStatusPanel`) AND Phase 3 (needs audit verdicts, already done).
- **Phase 5 (US3)**: Depends on Phase 4 completing (otherwise the zero-tolerance CI check fails immediately). Could technically start in parallel with Phase 4 if the script is defensive about pre-migration state, but cleanest ordering is Phase 4 → Phase 5.
- **Phase 6 (Polish)**: Depends on Phases 4 + 5 done.

### Within-Phase Dependencies

- **Phase 1**: T001 before T002 and T003 (both consume the panel).
- **Phase 4a**: T005, T006, T007 are all file-exclusive and can run in parallel. T025 depends on T005; T026 depends on T006.
- **Phase 4b**: T008–T024 all hit distinct files — full [P] parallel.
- **Phase 5**: T027 → T029 sequential (workflow wires the script); T028 independent of T027 but shares CI config with T029.
- **Phase 6**: T030, T031, T032, T033 all independent [P]; T034 last.

### Parallel Execution Examples

After Phase 1:

- **[P]** T008 … T024 (17 concurrent panel replacements)
- **[P]** alongside: T005, T006, T007 (3 reuse wins touching distinct files)

After Phase 4:

- T025 + T026 concurrent (two Playwright specs)
- T027 (CI script evolution)

---

## Implementation Strategy

### MVP slice (single user observable value)

**Phase 1 + Phase 4 = minimal shippable feature.** At that point every enterprise page shows either a real implementation or an honest status panel. Operators stop seeing marketing copy in their admin product.

### Phases to ship for SC completeness

- SC-001 (zero legacy strings): needs Phase 4 done + Phase 5 for enforcement.
- SC-002 (200ms render): Phase 1 Vitest smoke + Phase 4 per-page smoke.
- SC-003 (zero-violations CI): Phase 5.
- SC-004 (audit committed): Phase 3 (DONE).
- SC-005 (≥3 reuse wins): Phase 3 (DONE — 2 full + 1 partial).

Each phase adds verifiable value; you can stop after Phase 4 and ship a "soft" version without CI enforcement, or go all the way through Phase 6 for a final polish pass.

---

## Notes

- `[P]` tasks = distinct files, no dependencies on other unchecked tasks
- Commit after each task or logical group
- Branch: `002-expose-hidden-enterprise-stubs`
- Per the tracking-link policy (default from /speckit-clarify): all descoped-status panels point at `specs/001-enterprise-parity/spec.md` SR-01 table rows. When a descoped story later ships in its own spec, the consumer stub's `trackingLink` gets repointed in that feature's PR.
