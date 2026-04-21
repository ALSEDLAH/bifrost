# Phase 0 Research — Enterprise Stub Audit

**Feature**: 002-expose-hidden-enterprise-stubs
**Date**: 2026-04-20
**Purpose**: Satisfy US1 + FR-007 by recording, for each of the 20 enterprise stubs currently whitelisted in `scripts/check-sc020-enterprise-stubs.sh`, what upstream surface was searched and what verdict resulted.

## Methodology

Each stub was re-audited against the current upstream codebase using these searches:

1. **Handler grep** — `grep -rn "<topic>\b" transports/bifrost-http/handlers/*.go`
2. **Table grep** — `grep -rn "<topic>\|Table<Topic>" framework/configstore/tables*.go framework/logstore/tables*.go`
3. **Plugin grep** — `ls plugins/` + `grep -rn "<topic>" plugins/*/main.go`
4. **UI API hooks** — `grep -rn "<topic>" ui/lib/store/apis/*.ts` (some endpoints already have client bindings even if the UI stub doesn't use them)
5. **Config schema** — `grep -n "<topic>" transports/config.schema*.json`

Verdicts:

- **`expose`** — upstream handler + table + at least one of {client hook, existing fallback implementation} all exist. Writing a thin UI is a few hours of work.
- **`expose-partial`** — upstream surface exists but is incomplete for the UI's intent (e.g. read path only, or fallback works but embeds a teaser that needs trimming).
- **`descope → FeatureStatusPanel`** — no upstream backing. Honest informational panel replaces the marketing teaser.

## Audit Table

| # | Stub path | Topic | Upstream handler | Upstream table | Client hook | Fallback content | **Verdict** | Rationale |
|---|---|---|---|---|---|---|---|---|
| 1 | `access-profiles/accessProfilesIndexView.tsx` | Named scope bundles | `/api/rbac/roles` (spec 001 T032-T033) | `ent_roles` | `useGetRolesQuery` | ContactUsView teaser | **`expose`** | Roles = access profiles semantically. Thin wrap of `/api/rbac/roles` already worked once (spec 001 built it; it was reverted during US5 descope cleanup). Re-land with the "these are roles, labeled as profiles" framing. |
| 2 | `adaptive-routing/adaptiveRoutingView.tsx` | Traffic-split / weighted routing | `/api/governance/routing-rules` (governance.go:301-305, full CRUD) | `TableRoutingRule` (framework/configstore/tables/routing_rules.go) | `routingRulesApi.ts` full hook set | ContactUsView teaser | **`expose`** | Upstream governance routing-rules ARE weighted-target routing. Wrapping the existing rule editor with a "canary-focused" lens is UI-only work. Doesn't need US22's Config-object net-new plumbing. |
| 3 | `alert-channels/alertChannelsView.tsx` | Alert rules + destinations (Slack/webhook/email) | **none** — no alert handler | **none** | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | US10 is explicitly net-new (`plugins/alerts/`). Panel links to SR-01 entry + future spec placeholder. |
| 4 | `api-keys/apiKeysIndexView.tsx` | Admin API keys | upstream `auth_config` basic-auth (already surfaced by fallback content) | `auth_config` | `useGetCoreConfigQuery` | **Real curl-example content** + trailing ContactUsView teaser for "Scope Based API Keys" | **`expose-partial`** | Fallback shows the real basic-auth curl example (truthful). But it tacks on a ContactUsView teaser about scope-based keys (descoped per spec 001 US5). Action: strip the trailing teaser; keep the basic-auth docs. |
| 5 | `cluster/clusterView.tsx` | Cluster node management | **none** — no `/api/cluster` endpoints | **none** | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | US19 net-new. No cluster registry or node API exists upstream. |
| 6 | `data-connectors/bigquery/bigqueryConnectorView.tsx` | Log export to BigQuery | **none** — no exporter plugin | **none** | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | US11 net-new (`plugins/logexport/`). |
| 7 | `data-connectors/datadog/datadogConnectorView.tsx` | Log export to Datadog | **none** | **none** | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | Same as #6; US11 bundle. |
| 8 | `guardrails/guardrailsConfigurationView.tsx` | Central guardrail policies | config shipped in spec 010, enforcement shipped in spec 016 | full | full | real admin UI + runtime | **`shipped in specs 010 + 016`** | Rule CRUD in ent_guardrail_rules; plugins/guardrailsruntime evaluates each request via PreLLMHook + PostLLMHook with regex + OpenAI Moderation + custom-webhook providers, block/flag/log actions, audit.Emit on every match. |
| 9 | `guardrails/guardrailsProviderView.tsx` | External guardrail provider credentials (Aporia, Patronus, etc.) | config shipped in spec 010, enforcement shipped in spec 016 | full | full | real admin UI + runtime | **`shipped in specs 010 + 016`** | Provider CRUD in ent_guardrail_providers. Three v1 types: openai-moderation, regex, custom-webhook. Additional SaaS adapters (Aporia, Patronus, Bedrock Guardrails) remain future work on the same data model. |
| 10 | `large-payload/largePayloadSettingsFragment.tsx` | Streaming threshold settings | **none as endpoint** — threshold lives in `Config.StreamingDecompressThreshold`, set at deploy time only; no CRUD endpoint | — | fallback `useGetLargePayloadConfigQuery` is a no-op stub | renders `null` | **`descope → FeatureStatusPanel`** | Threshold is a deploy-time config, not a per-workspace setting. Surfacing as a read-only "currently N MB" panel would need a new endpoint. Defer; panel explains "this is set via deploy config" with doc link. |
| 11 | `login/loginView.tsx` | Enterprise SSO login page (SAML/OIDC) | **full (basic auth)** — `/api/session/login` + `GetAuthConfig` are build-agnostic in `session.go` | **full** | `useLoginMutation` shipped in the fallback form | **fallback re-export** | **`fallback re-export (basic auth)`** | Reversed 2026-04-20 after observing enterprise UI had no working auth UI at all (login stub + hidden admin setup = exposed dashboard). Basic-auth form is the reusable admin auth path; dedicated SSO handler remains a future spec. |
| 12 | `mcp-tool-groups/mcpToolGroups.tsx` | Group MCP tools for access control | **none** — `TableMCPClient` has no `tool_group_id`; no grouping endpoint | **none** | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | US30-T074 net-new concept. |
| 13 | `orgs-workspaces/organizationSettingsView.tsx` | Org settings UI | covered by `/api/governance/customers` — but this path is **unused** (no route mounts it; businessUnitsView is the canonical surface) | `governance_customers` | `useGetCustomersQuery` | stub re-export | **`descope → FeatureStatusPanel`** | Vestigial file kept only to satisfy the vite `@enterprise` alias. Panel explains "use /workspace/governance/business-units for org management". |
| 14 | `orgs-workspaces/workspacesView.tsx` | Workspaces UI | covered by `/api/governance/teams` — **unused**; teamsView is canonical | `governance_teams` | `useGetTeamsQuery` | stub re-export | **`descope → FeatureStatusPanel`** | Same as #13. |
| 15 | `pii-redactor/piiRedactorRulesView.tsx` | PII redaction rule config | **none** — no `plugins/pii-redactor/` | **none** | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | US7 net-new plugin. |
| 16 | `pii-redactor/piiRedactorProviderView.tsx` | PII provider credentials | **none** | **none** | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | Same bundle as #15. |
| 17 | `prompt-deployments/promptDeploymentView.tsx` | A/B split, canary, production-version pointer for prompts | upstream `plugins/prompts/` has versioning but **no production-version pointer, no A/B, no canary** (confirmed spec 001 Q1) | `TablePrompt`, `TablePromptVersion` exist but no deployment-strategy fields | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | US14-deployments descoped in spec 001 clarification Q1. |
| 18 | `prompt-deployments/promptDeploymentsAccordionItem.tsx` | Accordion sub-component for #17 | — | — | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | Companion of #17. |
| 19 | `scim/scimView.tsx` | SCIM 2.0 config page | **none** — no `handlers/scim.go` | **none** | — | ContactUsView teaser | **`descope → FeatureStatusPanel`** | US20 net-new handler. |
| 20 | `user-rankings/userRankingsTab.tsx` | Top-users dashboard tab | **full** — `RDBLogStore.GetUserRankings` exists upstream at rdb.go:1654; only needed plugin wrapper + HTTP route | **full** | `UserRankingsResponse` + `useGetUserRankingsQuery` shipped | Real table + trend arrows | **`shipped in spec 003`** | Original "descope → FeatureStatusPanel" verdict reversed after spec 003 audit found complete upstream plumbing. Live in `/workspace/dashboard` → User Rankings tab; drills through to `/workspace/logs?user_ids=...`. |

## Totals

- **`expose`**: **2** (access-profiles, adaptive-routing)
- **`expose-partial`**: **1** (api-keys fallback cleanup)
- **`descope → FeatureStatusPanel`**: **17**
- **Total audited**: **20**

**SC-005 target (≥3 reuse wins)**: ✅ **met** — 2 full + 1 partial = 3 net improvements.

## Key observations

1. **Spec 001's Phase-1 scan was thorough.** Out of 20 stubs, only 3 yielded reuse wins on the redux pass. The remaining 17 are genuinely net-new per SR-01 — the marketing teasers are all that's currently possible without licensing new backend work.
2. **The vestigial `orgs-workspaces/*` pair (#13, #14)** is a classic case: files kept only for alias-resolver tidiness. Previously left as fallback re-exports; under US2 they flip to `FeatureStatusPanel` that points operators at the canonical `businessUnitsView` / `teamsView` paths.
3. **The `api-keys` page (#4) is the most interesting edge case.** Its fallback content is already honest — it shows the real basic-auth curl example operators use today. The only issue is a trailing `ContactUsView` teaser tacked on at the bottom about "scope-based API keys" (US5, descoped). The fix is a 10-line fallback-file trim, not a new implementation.
4. **No new backend work surfaced.** SR-01 holds; this feature is purely UI surface management.

## Decisions (consolidated for the reader)

| Decision | What it means for Phase 2 tasks |
|---|---|
| Implement `access-profiles` as a read-only "Roles as Profiles" view | Wraps `useGetRolesQuery`; small 80-line component |
| Implement `adaptive-routing` as an editor over existing routing-rules | Wraps `routingRulesApi.ts` hooks; medium 200-line component with weight sliders |
| Trim the `api-keys` fallback trailer | Edit the _fallback_ file to drop the trailing `ContactUsView`; enterprise stub inherits the fix for free |
| Build `FeatureStatusPanel` once | ~80-line shared component; consumed by 17 stubs via a 3-line import |
| Evolve SC-020 CI check | Flip the script from path-whitelist to body-grep against all `ui/app/enterprise/components/**/*.tsx` |

## Alternatives considered

- **Revive descoped user stories wholesale.** Rejected — explicitly forbidden by SR-01 and would re-open the "invent-vs-expose" boundary this feature is closing.
- **Delete vestigial files (#13, #14) outright.** Rejected — the vite alias resolver needs every `@enterprise/components/...` path to resolve. `FeatureStatusPanel` is the cheaper stub.
- **Make `FeatureStatusPanel` a thin `ContactUsView` wrapper with different copy.** Rejected — the two components should be visually distinct. `ContactUsView` legitimately remains the OSS-build component (it's a real sales surface there); `FeatureStatusPanel` is an admin-internal "state" panel. Confusing them invites drift.
- **Auto-generate panel copy from the SR-01 table.** Attractive but overkill — 17 one-line descriptions is trivially typeable and reviewable.
