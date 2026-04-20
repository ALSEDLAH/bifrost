# Implementation Plan: Expose Hidden Enterprise Stubs

**Branch**: `002-expose-hidden-enterprise-stubs` | **Date**: 2026-04-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/002-expose-hidden-enterprise-stubs/spec.md`

## Summary

Close the final "Contact Us" gap left by spec 001. Two thrusts:

1. Run a systematic Phase-1-redux audit over the 20 currently-whitelisted enterprise stubs. For each: record what upstream handler/table/plugin was searched, the verdict, and the decision. Commit the output as `research.md` in this spec dir (FR-007 / SC-004).
2. Implement the decisions: **expose** stubs where the audit found reusable upstream logic; **replace** remaining stubs' marketing teasers with a new `FeatureStatusPanel` component that shows honest status + tracking link. Evolve `scripts/check-sc020-enterprise-stubs.sh` from a path-whitelist into a zero-tolerance build-wide guard.

Non-goals (SR-01 still authoritative): no new backend plugins, handlers, or tables. If a stub has no upstream surface, it gets an honest panel — not invented logic.

## Technical Context

**Language/Version**: TypeScript 5.9 + React 19 + Vite 8 (UI only; no Go changes expected).
**Primary Dependencies**: existing `@enterprise/lib` + `@/components/ui/*` (Radix + Tailwind).
**Storage**: N/A — this feature only relocates existing UI fallbacks.
**Testing**: Playwright E2E for the reuse wins (access-profiles, adaptive-routing) and visual smoke for the `FeatureStatusPanel`.
**Target Platform**: Enterprise Bifrost build (`BIFROST_IS_ENTERPRISE=true`).
**Project Type**: UI-only feature inside an existing web service.
**Performance Goals**: 200ms from first paint to panel visible (SC-002) — trivially met by server-rendered React.
**Constraints**: zero `core/**` changes (Principle I); no new backend surfaces per SR-01.
**Scale/Scope**: 20 stubs re-audited, ~2–3 real implementations + ~17 informational panels + 1 CI-check evolution.

## Constitution Check

| # | Principle | Status | Evidence / Notes |
|---|-----------|--------|------------------|
| I | Core Immutability | ✅ | Zero `core/**` changes. UI only. |
| II | Non-Breaking | ✅ | Only replacing UI fallbacks; no config fields added, no hook signatures touched. |
| III | Plugin-First | ✅ | No plugins added; reuses existing governance plugin endpoints where audit surfaces reuse wins. |
| IV | Config-Driven Gating | ✅ | No new gates. Existing `IS_ENTERPRISE` flag continues to differentiate the fallback tree from the enterprise stub tree. |
| V | Multi-Tenancy First | ✅ | Each exposed surface already carries `organization_id` / `workspace_id` via its upstream handler. |
| VI | Observability | ✅ | No new backend; existing audit/metric wiring suffices. |
| VII | Security by Default | ✅ | No new endpoints; no secrets introduced. |
| VIII | Test Coverage | ✅ | Playwright specs for the reuse-win pages; visual-diff for the shared `FeatureStatusPanel`. |
| IX | Docs & Schema Sync | ✅ | `docs/enterprise/*.mdx` touched for pages that flip status. Changelog updated. |
| X | Dependency Hierarchy | ✅ | UI-only change; no Go-side imports reorganized. |
| XI | Upstream-Mergeability | ✅ | `FeatureStatusPanel` is a new enterprise component; lives in `ui/app/enterprise/components/panels/`. Touches only enterprise-owned tree. Fallback `ContactUsView` is untouched for OSS build. |

No violations. No complexity-tracking entries needed.

## Project Structure

### Documentation (this feature)

```text
specs/002-expose-hidden-enterprise-stubs/
├── plan.md                 # This file
├── spec.md                 # Feature specification
├── research.md             # Phase 0 — per-stub audit (THE deliverable for US1)
├── data-model.md           # FeatureStatusPanel props schema + audit-row schema
├── contracts/
│   └── feature-status-panel.md   # Component's public interface
├── quickstart.md           # Reviewer walkthrough
└── checklists/
    └── requirements.md     # 16/16 passed at /speckit-specify time
```

### Source Code (repository root)

```text
# NEW shared component
ui/app/enterprise/components/panels/
└── featureStatusPanel.tsx  # Honest status panel; replaces ContactUsView inside enterprise build

# MODIFIED — stubs flipping to real implementations (audit reuse wins)
ui/app/enterprise/components/access-profiles/accessProfilesIndexView.tsx  # wraps /api/rbac/roles
ui/app/enterprise/components/adaptive-routing/adaptiveRoutingView.tsx     # wraps /api/governance/routing-rules
ui/app/enterprise/components/api-keys/apiKeysIndexView.tsx                # trims fallback ContactUsView teaser

# MODIFIED — stubs flipping from ContactUsView → FeatureStatusPanel
ui/app/enterprise/components/alert-channels/alertChannelsView.tsx
ui/app/enterprise/components/cluster/clusterView.tsx
ui/app/enterprise/components/data-connectors/bigquery/bigqueryConnectorView.tsx
ui/app/enterprise/components/data-connectors/datadog/datadogConnectorView.tsx
ui/app/enterprise/components/guardrails/guardrailsConfigurationView.tsx
ui/app/enterprise/components/guardrails/guardrailsProviderView.tsx
ui/app/enterprise/components/large-payload/largePayloadSettingsFragment.tsx
ui/app/enterprise/components/login/loginView.tsx
ui/app/enterprise/components/mcp-tool-groups/mcpToolGroups.tsx
ui/app/enterprise/components/orgs-workspaces/organizationSettingsView.tsx
ui/app/enterprise/components/orgs-workspaces/workspacesView.tsx
ui/app/enterprise/components/pii-redactor/piiRedactorProviderView.tsx
ui/app/enterprise/components/pii-redactor/piiRedactorRulesView.tsx
ui/app/enterprise/components/prompt-deployments/promptDeploymentView.tsx
ui/app/enterprise/components/prompt-deployments/promptDeploymentsAccordionItem.tsx
ui/app/enterprise/components/scim/scimView.tsx
ui/app/enterprise/components/user-rankings/userRankingsTab.tsx

# MODIFIED — CI check evolves
scripts/check-sc020-enterprise-stubs.sh   # path-whitelist → zero-tolerance body-grep

# UNTOUCHED (fallback used by OSS build)
ui/app/enterprise/components/views/contactUsView.tsx
ui/app/_fallbacks/enterprise/components/**
```

**Structure Decision**: One new shared component + three reuse-win rewrites + seventeen tiny stub-fills that call the panel with per-feature copy. No backend work. CI check modified in place.

## Phased Delivery

### Phase 0 — Research (complete; see [research.md](./research.md))

Audit of the 20 currently-whitelisted stubs. Summary:

| Verdict | Count | Stubs |
|---|---|---|
| **expose** (full reuse win) | 2 | access-profiles, adaptive-routing |
| **expose-partial** (reuse + trim) | 1 | api-keys (fallback already shows real basic-auth info but has a trailing ContactUsView block to trim) |
| **descope → FeatureStatusPanel** | 17 | the rest |

SC-005 target (≥3 reuse wins) **met**.

### Phase 1 — Design & Contracts

Artifacts produced:

1. [data-model.md](./data-model.md) — `FeatureStatusPanel` props schema + `AuditRow` shape.
2. [contracts/feature-status-panel.md](./contracts/feature-status-panel.md) — component's public interface (props, slots, rendering rules).
3. [quickstart.md](./quickstart.md) — one-page reviewer walkthrough.
4. `CLAUDE.md` updated to point at this plan between the SPECKIT markers.

### Phase 2 — Tasks

Generated by `/speckit-tasks` after this plan lands. Anticipated shape (not authoritative until tasks.md exists):

- T001: Build `FeatureStatusPanel` shared component + Storybook entry (or equivalent smoke-render).
- T002–T004: Implement the 3 reuse wins (access-profiles, adaptive-routing, api-keys cleanup).
- T005–T021: Replace ContactUsView usage in each of the 17 remaining stubs with `FeatureStatusPanel`.
- T022: Evolve `check-sc020-enterprise-stubs.sh` to zero-tolerance mode; empty the whitelist.
- T023: Playwright headless scan asserting zero legacy-string matches in the rendered DOM across enterprise routes.
- T024: Changelog entries + flipped `docs/enterprise/*.mdx` pages (access-profiles, adaptive-routing).

## Complexity Tracking

Empty — no principle gates triggered partial or fail.
