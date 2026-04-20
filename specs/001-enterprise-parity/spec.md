# Feature Specification: Bifrost Enterprise Parity

**Feature Branch**: `001-enterprise-parity`
**Created**: 2026-04-19
**Status**: Draft
**Input**: User description: "Bifrost Enterprise Parity — implement Portkey-equivalent enterprise capabilities in Bifrost without modifying the core module, so Bifrost can be sold into regulated enterprises (fintech, healthcare) while preserving 100% backward compatibility for existing OSS deployments."

## Clarifications

### Session 2026-04-19

- Q: Does air-gapped deployment (US19 / FR-037) require a per-feature support matrix decided now, or can it be deferred? → A: Decided now — MVP air-gapped scope is P1 features (Orgs, Workspaces, RBAC, SSO via OIDC only, Audit, Admin API Keys) plus P5 BYOK. Other features may be additively declared air-gap-supported later behind per-feature flags, but they are NOT in the v1 air-gapped profile.
- Q: Is multi-org-per-deployment required for MVP, or is single-org-per-deployment sufficient? → A: Single-org-per-deployment in v1, schema-ready for multi-org. Every tenant-scoped table/collection carries `organization_id` from day one, defaulted to a synthetic single organization. Multi-org activation is a future config flag (`multi_org_enabled`) that requires no schema migration.
- Q: Does canary traffic splitting (US22) interact with the governance plugin's routing-chain logic, or is it a new Config-level primitive? → A: Canary is a new primitive inside the declarative Config object (US16 / FR-033). Governance routing-chains remain scoped to budget/virtual-key enforcement and are not extended. Config owns routing/fallback/retry/cache/canary; governance owns spend-and-limits.
- Q: Does BYOK (US18) apply to logstore request/response payloads, or only to configstore secrets? → A: BYOK encrypts configstore secrets by default (provider keys, webhook secrets, admin API keys, prompt templates, OAuth client secrets). Logstore payload encryption is an opt-in per-workspace setting with 15-minute data-key caching (per SC-009). This preserves hot-path performance for customers who do not need it.
- Q: Does Prompt Library (US14) store templates in the existing configstore, or does it need a dedicated backing store? → A: v1 stores prompts, versions, partials, and folders in configstore, reusing existing multi-tenancy, encryption, and hot-reload infrastructure. A dedicated `framework/promptstore/` subsystem is explicitly deferred to v2 and will be introduced only if prompt volume or version-history size exceeds configstore operational tolerances.
- Q: Does Bifrost support both a vendor-hosted cloud tier AND customer self-hosted (on-prem) from one codebase? → A: Yes. Single binary, single image. Behavior differs via a new `deployment.mode` config field with values `cloud` (vendor SaaS, multi-org, metering + billing enabled), `selfhosted` (customer on-prem, single-org, license-key gated, no phone-home by default), and `airgapped` (customer on-prem, no outbound, MVP-feature subset per FR-037). Each mode sets opinionated defaults for multi-org, telemetry, licensing, and metering so operators flip one flag instead of many.
- Q: What is the commercial gating model for self-hosted enterprise features? → A: License-key-gated with hard expiry. A signed license file (offline-verifiable, no network call required) encodes the customer identity, issued-at + expires-at timestamps, and a per-feature entitlement list. Enterprise features consult a license plugin via context key; on expiry the deployment enters a 14-day grace period with a prominent UI banner and daily audit entries, after which enterprise features disable gracefully (revert to OSS-only behavior) while leaving existing data accessible read-only. OSS core features are never license-gated. Cloud mode does not require a customer license file (the vendor operates that deployment).
- Q: Is cloud-tier billing/metering in scope for this feature, or deferred to a separate track? → A: In scope — expanded into this spec as a new Train E "Cloud Commercial" (US26–US29). Train E activates only in `deployment.mode: cloud` and covers per-organization usage metering, Stripe-based subscription + usage billing, a customer-facing billing portal, and tier/plan management (Dev/Prod/Enterprise). Train E is strictly additive: self-hosted and air-gapped modes do not load the metering, Stripe, or billing-portal plugins at all (per Principle IV).
- Q: Who/what issues self-hosted license files? → A: A dedicated CLI tool `tools/license-authority/` that vendor ops runs locally. Inputs (customer name, entitlement list, expiry, seat/workspace/VK limits, contact email); outputs a signed JWT file. Vendor's Ed25519 private key is held in a password-manager vault, loaded into the CLI only at issuance time, never committed to the repo. v1 is manual issuance triggered by sales handoff; v2 may auto-issue on Stripe webhook for cloud→self-hosted hybrid customers. The CLI is NOT a separate service and does not run in production.
- Q: US8 scope — upstream governance plugin already ships full budget/rate-limit enforcement. What should US8 become? → A: Drop US8 implementation tasks (T126-T139). Reduce to single task: add budget-threshold-alert emission (50%/75%/90%) to existing governance tracker. FR-020, FR-021, FR-023 marked ALREADY_SHIPPED.
- Q: Governance + prompts plugins are disabled in enterprise mode. Enterprise-gate runs a parallel tenant resolution. What should change? → A: (1) Re-enable governance AND prompts plugins in enterprise mode — they must coexist. (2) Remove enterprise-gate plugin entirely — it is redundant. Governance plugin already resolves VK → Team (workspace) → Customer (organization), which IS the tenant resolution. Enterprise auth (admin-api-keys, service-account-keys, SSO sessions) should be added as extensions to the existing auth middleware, not as a separate plugin. Enterprise migrations (E001-E004) move to a framework-level migration runner invoked at server startup.
- Q: Should there be a top-level success criterion for 100% enterprise surface coverage? → A: Yes. Added SC-020: "Zero ContactUsView stubs remain in the enterprise build." CI check validates no "enterprise license" placeholder text in built JS bundle.
- Q: Prompt deployment strategies (A/B testing, canary prompt rollouts) — separate US or fold into US14? → A: Fold into US14 (Prompt Library). The production-version pointer is already there; deployment strategies are the natural extension. Fill the prompt-deployments UI stub as part of US14 work.
- Q: 4 enterprise stubs (MCP Tool Groups, MCP Auth Config, Large Payload, Proxy/SCIM) have no user story. How to cover? → A: Add one new user story US30 "Enterprise Platform Features" covering all 4 as a group. Priority P3. Each has pre-wired routes and some backend support — primarily UI work to fill the stubs.
- Q: RBAC model — our backend has 14 resources × 3 verbs but frontend already defines 29 resources × 6 operations. Which wins? → A: Align backend to match the frontend's existing 29 RbacResource enum (GuardrailsConfig, GuardrailsProviders, GuardrailRules, UserProvisioning, Cluster, Settings, Users, Logs, Observability, VirtualKeys, ModelProvider, Plugins, MCPGateway, AdaptiveRouter, AuditLogs, Customers, Teams, RBAC, Governance, RoutingRules, PIIRedactor, PromptRepository, PromptDeploymentStrategy, AccessProfiles, BusinessUnits, AlertChannels, SCIMConfig, MCPToolGroups, MCPAuthConfig) and 6 RbacOperation enum (Read, View, Create, Update, Delete, Download). Drop our 14-resource model. The frontend is already wired — backend must match it.
- Q: UI routing — fill existing 28 fallback stubs or keep parallel routes we created? → A: Fill existing fallback stubs with real implementations. Drop all parallel routes we created (/workspace/workspaces/, /workspace/organization/). Upstream's routing and sidebar links already point to the correct pages. For US1: businessUnitsView → org/customer management, teamsView → workspace/team management, usersView → user management. For all other enterprise features: replace the ContactUsView stub with a working component in ui/app/enterprise/components/.
- Q: Tenancy tables — keep parallel ent_organizations/ent_workspaces alongside governance_customers/governance_teams, or consolidate? → A: Option B — use governance_customers as organizations and governance_teams as workspaces directly. Drop ent_organizations and ent_workspaces entirely. Do NOT add enterprise columns to the upstream tables yet — deal with enterprise-specific fields (SSO, retention, residency, etc.) later when specific features require them. The existing Customer/Team fields (Name, BudgetID, RateLimitID, Profile/Claims/Config JSON) are sufficient for MVP. The JSON extension fields on Team (Profile, Claims, Config) are the natural extension point when needed.
- Q: How does the signing key rotate if compromised or nearing end-of-life? → A: Multi-key embed. Each Bifrost binary ships with an ordered array `[pubkey_v1, pubkey_v2, …]` of valid public keys. On license verification, each embedded key is tried in order; the first valid signature wins. Rotation cadence: 12 months. Rotation flow — (a) generate new keypair, (b) embed new public key alongside existing in the next binary release, (c) start issuing new licenses signed with new key, (d) after all customers are known to be on a binary version containing the new public key, drop the old key in the subsequent release. Air-gapped customers control their update cadence; the vendor MUST support the old key until the longest-deployed air-gapped binary has been superseded.
- Q: What happens when a license's declared limits (`max_workspaces`, `max_users`, `max_virtual_keys`) are exceeded at runtime? → A: Block-new, allow-existing. Existing resources continue to function unchanged; attempts to CREATE new resources beyond the limit return HTTP 402 with a clear renewal-contact link. `max_users` receives a 10% soft-grace (e.g., a 100-seat license allows 110 active users) with escalating UI warnings at 100%, 105%, 110%. Hard cutoff for `max_workspaces` and `max_virtual_keys` because those affect persistence scale, not just identity count. This pattern prevents production breakage on an overshoot and forces a renewal conversation rather than an outage.
- Q: Cloud mode — single-region or multi-region in v1? → A: Schema-ready for multi-region, operationally single-region (US-EAST-1) in v1. The `organizations.data_residency_region` column already exists from data-model §1; it defaults to `us-east-1` in v1 and is read-only in the UI. Multi-region activation (adding EU-WEST-1 for GDPR) is a v2 concern that requires no schema migration, only the deployment of a second regional instance and a region-routing front door. Customers requiring EU residency in v1 should use the self-hosted deployment model in their own EU infrastructure.

### Session 2026-04-20

- Q: What actually counts as "in scope" for enterprise parity? → A: **Reuse-over-new.** The scope is exposing upstream logic that already exists, not inventing parallel systems. A fallback `ContactUsView` teaser in the frontend (marketing copy) is NOT proof that the feature exists — it must have real upstream data model, handler, plugin, or context infrastructure to be in scope. Net-new backend inventions (new tables, new handlers, new plugins with no upstream precedent) are out of scope for this spec and MUST be extracted into their own feature specs before any code is written.
- Q: US5 (admin API keys) — in scope or descoped? → A: **DESCOPED 2026-04-20.** Upstream ships one admin auth path: `auth_config.admin_username` + `admin_password` (basic auth), surfaced by the existing enterprise `apiKeysIndexView` fallback which shows the curl example. A named/scoped/expiring multi-key system has no upstream equivalent and is therefore net-new; if ever revived it must be its own spec. The `apiKeysIndexView` stub remains an OSS fallback re-export — it correctly represents the existing admin-auth surface.
- Q: US14 Prompt Library — in scope as a whole, or split? → A: **Split.** Basic prompt library (templates, variable substitution, folders, version history) is IN SCOPE — already satisfied by the re-enabled upstream `plugins/prompts/` in Phase 1 T012 (no new code required here). Deployment-strategies subset (A/B split, canary, production-version pointer — T068–T070) is OUT OF SCOPE per SR-01: extending the prompts plugin with new deployment primitives is net-new and needs its own feature spec. FR-029 retained with this split called out explicitly; FR-029-deployments moves to a future spec. The prompt-deployments UI stubs (`promptDeploymentView.tsx`, `promptDeploymentsAccordionItem.tsx`) stay as OSS fallback re-exports with a DESCOPED whitelist entry in `scripts/check-sc020-enterprise-stubs.sh`.
- Q: Audit entry `actor_id` attribution in v1 (no per-request auth→user middleware wired) → A: **Synthetic "upstream-admin".** When the default tenant resolver produces a TenantContext (i.e., admin action reached a handler via the upstream basic-auth credential), audit entries are attributed with `actor_type="system"` and `actor_id="upstream-admin"`. This makes logs self-documenting (no confusing blanks), matches the real identity behind the call (one shared admin credential), and will flip to real user IDs automatically when session→user middleware lands in a future spec — callers of `emitAudit` don't change.
- Q: v1 RBAC enforcement surface — server-side scope checks on which endpoints? → A: **UI-only gates in v1, binary server auth (matches upstream posture per SR-01).** `useRbac(resource, operation)` hides UI controls; basic-auth / session-token remains the only server gate. The 24-resource × 6-operation scope model is persisted in `ent_roles` / `ent_user_role_assignments` and consumed by the UI via `/api/rbac/me`, but no server handler reads `TenantContext.RoleScopes` today. Adding server-side scope enforcement is a **net-new enforcement layer** with no upstream precedent — same SR-01 class as admin API keys — and belongs in its own feature spec. A client bypassing the UI and calling enterprise endpoints with valid basic-auth WILL succeed; this limitation is documented in `docs/enterprise/rbac.mdx` under "Notes & limits".
- Q: `ent_audit_entries` — which database (configstore vs logstore)? → A: **Configstore** (e.g. `config.db` in SQLite deployments). Audit rows are semantically closer to the governance + RBAC tables they reference (`role.create` entry vs `ent_roles` row) than to request-body logs, and co-locating them keeps self-hosted deployments on a single SQLite file. The original plugin doc comment said "logstore DB"; that predates the post-audit table refactor. Plugin comment, FR-012, and `docs/enterprise/audit-logs.mdx` are updated to match shipped reality. No data migration needed.

## Scope Rules

### SR-01 Reuse-over-new (added 2026-04-20)

**Rule:** Every user story in this spec MUST map to existing upstream code that is disabled, gated, or un-surfaced. Stories that would require brand-new tables, brand-new handlers, or brand-new plugins that have no upstream precedent are OUT OF SCOPE until extracted into their own feature spec.

**Classification gate (apply to every user story + phase before implementation):**

| In scope — *expose existing logic* | Out of scope — *net-new, requires own spec* |
|---|---|
| US1 — Orgs & Workspaces (governance_customers + governance_teams already exist) | **US3** — SSO handlers (SAML/OIDC) — new `handlers/sso.go` + new SAML/OIDC stack |
| US2 — Granular RBAC (tenancy.RoleRepo + frontend rbacContext enum already exist) | **US5** — Admin API Keys — **DESCOPED 2026-04-20** (upstream basic-auth already provides admin auth) |
| US4 — System-Wide Audit Logs (TableAuditEntry + audit.Emit already exist) | **US6** — Central Guardrails — new `plugins/guardrails-central/` plugin |
| US8 — Budget Threshold Alerts (already-shipped governance tracker; extension only) | **US7** — PII Redactor — new `plugins/pii-redactor/` plugin |
| US12 — Executive Dashboard (existing metrics/analytics handlers) | **US9** — Custom Guardrail Webhooks — depends on US6 guardrails-central |
| US13 — Retention Periods (configstore extension only) | **US10** — Alerts & Notifications — new `plugins/alerts/` plugin |
| US14 basic — Prompt Library (templates, variables, folders, version history) — satisfied by upstream `plugins/prompts/` re-enable (Phase 1 T012) | **US11** — Log Export — new `plugins/logexport/` plugin |
| | **US14-deployments** — A/B split, canary, production-version pointer — net-new extension of `plugins/prompts/` (T068–T070); needs own spec |
| US30 — Enterprise Platform stubs (MCP tool groups, MCP auth, large payload, proxy/SCIM — all wrap existing handlers) | **US15** — Prompt Playground — new playground handler |
| | **US16** — Declarative Config Objects — new `handlers/configs.go` + new storage |
| | **US17** — Service Account Keys — new table + handler |
| | **US18** — BYOK — new `framework/kms/` module + KMS adapters |
| | **US19** — Air-Gapped / Cluster — new Helm profile + cluster plumbing |
| | **US20** — SCIM 2.0 — new `handlers/scim.go` |
| | **US21** — Terraform Provider — new provider repo |
| | **US22** — Canary Routing — depends on US16 |
| | **US23** — Data-Lake ETL — depends on US11 logexport |
| | **US24–US29** — License + Cloud Commercial (Train E) — new license plugin, new metering, new Stripe integration, new billing portal, new tier matrix |

**Phase-level consequence (tasks.md alignment):**

- **Phase 3 (US1), Phase 4 (US2), Phase 5 (US4), Phase 12 (US12/US13), Phase 15 (US30)** — in scope; exposing existing logic.
- **Phase 6 (US5)** — DESCOPED; fallback stub remains.
- **Phase 7 (US6), Phase 8 (US7), Phase 9 (US8 done / US9 descoped), Phase 10 (US10), Phase 11 (US11)** — US8 in scope (already-shipped governance tracker extended in-place); everything else net-new and requires its own feature spec before implementation starts.
- **Phase 13 (US14–US16), Phase 14 (US17), Phase 16 (US3/US19/US20), Phase 17 (US18/US21–US23), Phase 18 (US24–US29)** — net-new; require their own specs.

**How to apply to a user story or phase:**

1. Identify the upstream artifact that implements the feature today (table, handler, plugin, middleware, context).
2. If no such artifact exists, stop — this is net-new; open a fresh spec via `/speckit-specify`.
3. If the artifact exists but is guarded or un-surfaced, the in-scope work is: remove the guard, wire the UI, add a thin query handler at most. No new persistence, no new plugin.
4. A "Contact Us" / marketing teaser in a fallback UI is not an upstream artifact.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Organizations & Workspaces Multi-Tenant Isolation (Priority: P1)

As an **Org Owner** onboarding a regulated enterprise onto Bifrost, I can
create an **Organization** with multiple **Workspaces** (e.g., "Product",
"Data Science", "Compliance") so that each team's virtual keys, prompts,
model catalogs, logs, analytics, and provider configs are fully isolated
from other teams.

**Why this priority**: Workspace isolation is the foundational primitive
that every other enterprise capability (RBAC, audit logs, budgets,
guardrails) scopes against. Without it, no regulated buyer will deploy.
Portkey gates this entire capability behind its Enterprise tier and
treats it as the #1 differentiator.

**Independent Test**: Create two workspaces in one organization, provision
a user scoped to workspace A only, and verify that user cannot list or
access any resource (virtual key, prompt, log entry, analytics query,
provider config) belonging to workspace B via the UI or API. Delivers
data-isolation guarantees suitable for compliance review.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/org-management

**Acceptance Scenarios**:

1. **Given** a fresh deployment upgraded from Bifrost v1.5.2 with existing
   virtual keys, **When** the administrator enables Organizations, **Then**
   all pre-existing virtual keys, teams, customers, providers, and logs
   auto-migrate into a default organization with a single "default"
   workspace and continue to function without re-configuration.
2. **Given** an Org Owner, **When** they create a new workspace, **Then**
   the workspace has independent virtual keys, prompts, model catalog,
   provider configurations, guardrail policies, and log view.
3. **Given** a user with access only to Workspace A, **When** they query
   the logs API or analytics API, **Then** results include only requests
   made against Workspace A's virtual keys; Workspace B data is never
   returned and never appears in counts or aggregates.
4. **Given** a deleted workspace, **When** the deletion completes, **Then**
   all associated virtual keys are revoked, logs are scheduled for
   retention-policy-driven deletion, and an audit entry records the action.

---

### User Story 2 — Granular RBAC with Custom Roles (Priority: P1)

As an **Org Admin**, I can assign users one of three default
organization-level roles (Owner, Admin, Member) and/or workspace-level
roles (Manager, Member), and I can additionally define **custom roles**
with per-resource scopes so that sensitive resources (provider credentials,
guardrail policies, budget controls) can only be touched by users with
explicit permission.

**Why this priority**: Enterprise buyers need role separation for SOC 2 /
ISO 27001 controls around privileged access. Without granular RBAC, the
auditor cannot demonstrate that engineers don't have access to billing
controls and finance users don't have access to provider keys.

**Independent Test**: Create a custom role "ReadOnly Analyst" with only
read scopes on metrics + completions logs; assign a user; verify that
user can view dashboards but receives 403 on any write endpoint
(creating a virtual key, editing a guardrail, modifying a provider).

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/access-control-management

**Acceptance Scenarios**:

1. **Given** an Org Owner, **When** they assign the Admin role to a user,
   **Then** that user can manage virtual keys, prompts, configs,
   guardrails, integrations, providers, and team members, but cannot
   modify billing or delete the organization.
2. **Given** an Org Admin, **When** they create a custom role with scopes
   `metrics:read, completions:read`, **Then** users with that role see
   analytics dashboards but all write actions return permission-denied
   with a clear error message indicating the missing scope.
3. **Given** a user promoted from Workspace Member to Workspace Manager,
   **When** they next authenticate, **Then** they can invite new users,
   approve API key creation, and edit workspace-level guardrails within
   that workspace only.
4. **Given** any role change, **When** the change is saved, **Then** an
   audit log entry captures the actor, subject user, previous role, new
   role, and timestamp.

---

### User Story 3 — Enterprise SSO via SAML 2.0 and OIDC (Priority: P1)

As an **IT Administrator** at a Fortune 500 company, I can configure my
corporate identity provider (Okta, Azure AD / Entra ID, Google Workspace,
or any OIDC/SAML 2.0 provider) so that all Bifrost access goes through
our SSO with MFA enforced, and users are auto-provisioned on first login.

**Why this priority**: SSO is the single most common non-negotiable
procurement requirement. No enterprise security review will approve
another set of credentials for engineers to manage.

**Independent Test**: Configure Okta as the OIDC provider, log in with
an Okta user that has no pre-existing Bifrost account, verify the user
is created automatically with default role and placed into a pending-
invite workspace, and verify all subsequent auth flows through Okta.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/org-management/sso

**Acceptance Scenarios**:

1. **Given** SSO is configured with Okta OIDC, **When** a new employee
   authenticates via their Okta credentials, **Then** a Bifrost user is
   auto-provisioned with the default role defined in SSO settings, and
   any pending workspace invitations matching their email are
   auto-accepted.
2. **Given** SSO is configured with SAML 2.0 Entra ID, **When** a user
   authenticates, **Then** Bifrost consumes the SAML assertion,
   validates the signature, maps `groups` claims to workspace
   memberships per configuration, and creates a session.
3. **Given** SSO is configured and marked "required", **When** a user
   attempts local password login, **Then** the attempt is rejected with
   a message directing them to the SSO login URL.
4. **Given** an admin rotates the IdP signing certificate, **When** they
   update the metadata in Bifrost settings, **Then** active sessions
   continue uninterrupted and new sessions use the new certificate.

---

### User Story 4 — System-Wide Audit Logs (Priority: P1)

As a **Compliance Officer** preparing for a SOC 2 audit, I can view,
filter, and export a complete system audit trail of every administrative
action taken against Bifrost so that I can demonstrate who did what,
when, from where, and what changed.

**Why this priority**: Audit logs are a SOC 2 CC7.2, ISO 27001 A.12.4,
and HIPAA 164.312(b) required control. No compliant deployment can ship
without them.

**Independent Test**: Perform five administrative actions (create virtual
key, edit guardrail, assign role, rotate admin API key, delete workspace)
as one user, then view the audit log page filtered by that user and
verify each action appears with full before/after state diff, actor IP,
request ID, and ISO 8601 timestamp; export to CSV and verify fields
match.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/audit-logs

**Acceptance Scenarios**:

1. **Given** any administrative action in the UI or Admin API, **When**
   the action completes, **Then** an audit log entry is recorded with
   actor ID, actor IP, action type, resource type, resource ID,
   before/after values for changed fields, request ID, and timestamp.
2. **Given** a compliance officer, **When** they filter the audit log by
   date range, actor, action type, or resource type, **Then** results
   update within 2 seconds and match the filter exactly.
3. **Given** audit logs containing 100,000+ entries, **When** a
   compliance officer exports the filtered result to CSV or JSON,
   **Then** the export completes and contains the full set of matching
   entries without truncation.
4. **Given** an attempted action that is denied (e.g., insufficient
   scope), **When** the denial fires, **Then** an audit entry records
   the attempt with outcome=denied and the reason.

---

### User Story 5 — Organization-Level Admin API Keys (Priority: P1) — DESCOPED 2026-04-20

> **Status: DESCOPED 2026-04-20.** Per Scope Rule SR-01 (Reuse-over-new), this story is net-new: upstream has no scoped, named, expiring multi-key admin credential — only the single `auth_config.admin_username` + `admin_password` basic-auth credential, which is already surfaced by the enterprise `apiKeysIndexView` fallback. A parallel scoped-key system has no existing upstream backend to expose. To revive this feature, open a separate spec via `/speckit-specify`. The acceptance scenarios below are preserved for reference in that future spec.

As a **Platform Engineer**, I can create org-level Admin API Keys with
specific scopes (e.g., "manage virtual keys only", "read analytics only")
and expiry dates so I can automate workspace and key lifecycle from CI/CD
without using a human user's credentials.

**Why this priority**: Automated provisioning (Terraform, CI/CD) is
standard enterprise practice; without scoped admin keys, teams resort to
personal access tokens which violate the principle of least privilege.

**Independent Test**: Create an Admin API Key with scope
`virtual_keys:write`, use it to create a new virtual key via API,
confirm success; attempt to use the same key to delete a workspace and
confirm 403 with a message naming the missing scope.

**Parity mapping**: https://portkey.ai/docs/api-reference/admin-api/introduction

**Acceptance Scenarios**:

1. **Given** an Org Owner, **When** they create an Admin API Key with a
   set of scopes and an expiration date, **Then** the key value is
   displayed exactly once at creation, only a prefix is stored for
   display thereafter, and the key is immediately usable.
2. **Given** an Admin API Key near expiration, **When** 7 days and 1 day
   remain, **Then** an email alert fires to the key creator and org
   owners; after expiration, the key stops working and attempts are
   logged as denied.
3. **Given** a compromised Admin API Key, **When** an Org Owner rotates
   or revokes it, **Then** the key immediately stops working across all
   active sessions and an audit entry records the revocation.

---

### User Story 6 — Central & Org-Wide Guardrails (Priority: P2)

As a **Security Team Lead**, I can enable centrally-enforced guardrails
across an entire organization (deterministic: regex, JSON schema, code
detection, word count; LLM-based: prompt injection, gibberish; partner-
integrated: Aporia, Pillar, Patronus, SydeLabs, Pangea) so that every
request — regardless of which workspace or virtual key it originates
from — is subject to our corporate AI safety policy.

**Why this priority**: Central guardrails are the primary AI-safety
control demanded by legal, security, and AI-governance committees at
regulated firms. Per-workspace guardrails exist today but org-wide
enforcement requires new central policy.

**Independent Test**: Configure an org-wide prompt-injection guardrail
with action=deny; from two different workspaces submit a request
containing a known prompt-injection payload; confirm both requests are
blocked with HTTP 446, a clear user-facing reason, and a guardrail
event in the audit trail.

**Parity mapping**: https://portkey.ai/docs/product/guardrails

**Acceptance Scenarios**:

1. **Given** an org-wide guardrail is configured, **When** any request
   arrives at any workspace, **Then** the guardrail evaluates against
   request input and/or response output per its configuration, and
   applies its configured action (deny with 446, allow-with-warning
   with 246, retry, fallback, or log-only).
2. **Given** multiple guardrails configured, **When** a request is
   processed, **Then** guardrails execute per the configured execution
   mode (sequential vs parallel, sync vs async) and compose their
   verdicts per the configured combination policy (any-deny,
   all-must-pass, majority).
3. **Given** a partner guardrail integration (e.g., Aporia), **When** the
   partner API is unreachable, **Then** the guardrail falls back per the
   configured on-failure policy (block / allow / warn) and emits a
   metric + audit entry capturing the failure.
4. **Given** an admin updating guardrail configuration, **When** they
   save changes, **Then** the change takes effect within 30 seconds
   across all gateway instances without requiring a restart.

---

### User Story 7 — PII Anonymizer / Central Redaction (Priority: P2)

As a **Chief Privacy Officer**, I can enable an org-wide PII redaction
guardrail that automatically detects and redacts personal identifiable
information (names, SSNs, credit cards, phone numbers, emails, addresses,
health identifiers) from request and response payloads before those
payloads are persisted to logs so that no PII is ever stored in Bifrost's
logstore, even accidentally.

**Why this priority**: PII in logs is the #1 source of data-breach
incidents in AI gateways. HIPAA 164.514 and GDPR Article 5 require
demonstrable minimization. This must be opt-out-proof for compliance.

**Independent Test**: Enable PII redaction; submit a request containing
an obvious SSN, credit card, and email; verify the downstream provider
receives either the redacted or original content per policy; verify the
Bifrost log entry contains only redacted placeholders for all three PII
types; verify the redaction event is counted in the PII-redacted metric.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/security-portkey

**Acceptance Scenarios**:

1. **Given** PII redaction is enabled org-wide, **When** a request
   containing an SSN, email, and phone number is logged, **Then** the
   stored log entry contains `<REDACTED:SSN>`, `<REDACTED:EMAIL>`,
   `<REDACTED:PHONE>` placeholders and never the raw values.
2. **Given** PII redaction is configured with mode
   `redact-before-provider`, **When** a request is routed to a
   downstream LLM provider, **Then** the provider receives redacted
   content and the response similarly has PII redacted before it is
   returned to the caller (with a header indicating redaction occurred).
3. **Given** a request where PII detection fails or times out, **When**
   the fail-closed policy is active, **Then** the request is blocked
   with HTTP 446 rather than being logged unredacted.
4. **Given** a configured PII pattern set, **When** the admin adds a
   custom regex (e.g., internal employee ID format), **Then** new
   requests are redacted against the expanded pattern set within 30
   seconds.

---

### User Story 8 — Budget Threshold Alerts (Priority: P2)
*(status per SR-01 classification table: in scope as an extension of the already-shipped governance tracker — delivered 2026-04-20 as T055)*

**Upstream status**: The governance plugin (`plugins/governance/`) already
ships full per-virtual-key budget caps (dollar/token, any reset duration),
multi-window rate limits (per-minute/hour/day for requests and tokens),
enforcement at VK/team/customer/provider/model levels, calendar-aligned
resets, and HTTP 429/402 responses. Tables `governance_budgets` and
`governance_rate_limits` are fully functional.

As a **FinOps Manager**, I can configure threshold alerts (50%, 75%, 90%)
on existing budget caps so that stakeholders are notified via webhook
and/or Slack before budget exhaustion.

**Why this priority**: Budget enforcement is already shipped. The only
Portkey parity gap is proactive threshold alerting before limits are hit.

**Independent Test**: Set a virtual key with a `$100/month` budget;
generate traffic summing to $50 and verify a 50% threshold alert fires
to the configured Slack channel; continue to $75 and $90 and verify
subsequent alerts; verify alerts include current spend, threshold
percentage, and time remaining in period.

**Parity mapping**: https://portkey.ai/docs/product/ai-gateway (Virtual Keys)

**Acceptance Scenarios**:

1. **Given** a virtual key with a monthly budget of $100 and alert
   thresholds at 50%/75%/90%, **When** cumulative spend crosses each
   threshold, **Then** a notification fires to the configured alert
   destinations (webhook, Slack) with current spend, threshold, and
   time remaining in period.
2. **Given** threshold alerts are configured, **When** a budget resets
   at period boundary, **Then** alert state resets and thresholds
   re-arm for the new period.

---

### User Story 9 — Custom Guardrail Webhooks (Priority: P2)

As a **Security Engineer**, I can register a custom HTTPS webhook
endpoint as a guardrail so that Bifrost calls my endpoint synchronously
with request content and my endpoint returns allow/deny/warn with an
optional reason, letting me enforce company-specific policies that
don't fit built-in guardrail types.

**Why this priority**: Every regulated enterprise has proprietary
policies (e.g., "no mentions of unreleased product codenames in
prompts", "block requests referencing specific client names") that
generic guardrails cannot cover.

**Independent Test**: Point Bifrost at a simple webhook that returns
`{"verdict": "deny", "reason": "contains internal codename"}` for any
request containing "ProjectThunder"; submit matching and non-matching
requests and verify only matching requests are blocked with the
returned reason surfaced in both the API response and audit log.

**Parity mapping**: https://portkey.ai/docs/product/guardrails

**Acceptance Scenarios**:

1. **Given** a custom webhook guardrail is registered with a timeout of
   500ms, **When** a request is processed, **Then** Bifrost calls the
   webhook with signed payload containing prompt content and metadata,
   and respects the response verdict.
2. **Given** a webhook that fails to respond within the configured
   timeout, **When** the timeout elapses, **Then** the configured
   on-timeout policy (fail-open / fail-closed) takes effect and a
   guardrail-timeout metric is incremented.
3. **Given** webhook payload signing is enabled, **When** the webhook
   receives a request, **Then** it can verify the HMAC signature using
   the shared secret configured at guardrail creation time and reject
   unsigned or tampered payloads.

---

### User Story 10 — Alerts & Notifications (Priority: P3)

As an **Operations Engineer**, I can configure threshold-based alerts
on error rate, latency, cost-per-hour, and user-feedback-score metrics
that fire to webhooks and Slack so that on-call is paged before end-
users notice a degradation.

**Why this priority**: Alerting closes the loop on observability.
Without it, the dashboards exist but nobody watches them proactively.

**Independent Test**: Configure an alert rule "P95 latency > 3000ms
for 5 minutes" with Slack destination; inject synthetic slow responses
from a provider for 6 minutes; verify Slack message arrives within 90s
of the 5-minute window elapsing, containing the current value, the
threshold, and a link to the relevant dashboard.

**Parity mapping**: https://portkey.ai/docs/product/observability

**Acceptance Scenarios**:

1. **Given** an alert rule configured for error-rate > 5% over 10
   minutes, **When** the threshold is crossed, **Then** the configured
   notification (webhook, Slack, email) fires within 90 seconds with
   actual value, threshold, affected workspace/virtual-key scope, and
   a deep link to the relevant dashboard.
2. **Given** an active alert, **When** the metric recovers below
   threshold for the configured cooldown window, **Then** a resolution
   notification is sent and the alert state returns to OK.
3. **Given** multiple alerts in quick succession, **When** notifications
   fire, **Then** they are deduplicated and coalesced per the configured
   grouping policy to avoid paging storms.

---

### User Story 11 — Log Export to Data Destinations (Priority: P3)

As a **Data Platform Engineer**, I can configure Bifrost to continuously
export logs and/or metrics to my company's data destinations (AWS S3,
Wasabi, MongoDB / DocumentDB, ClickHouse via OpenTelemetry collector,
Azure Blob Storage, Google Cloud Storage) so that Bifrost telemetry
flows into our centralized observability and compliance stacks.

**Why this priority**: Enterprises have existing data lakes and SIEM
systems; they will not adopt Bifrost's built-in UI as a replacement.
Export is the bridge.

**Independent Test**: Configure S3 log export with a 5-minute interval
to a customer-owned bucket; generate 1,000 requests; verify within
10 minutes the bucket contains gzipped JSONL files totaling exactly
1,000 log records, each with the schema documented in the export
contract.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/components

**Acceptance Scenarios**:

1. **Given** an S3 export destination is configured, **When** requests
   are logged to Bifrost, **Then** within the configured batch interval
   the records are written to the target bucket with a documented
   partitioning scheme (e.g., `year=YYYY/month=MM/day=DD/hour=HH/`) and
   a documented record schema.
2. **Given** the export destination is unreachable, **When** exports
   queue, **Then** retries occur with exponential backoff up to a
   configured maximum age, after which records are dead-lettered to a
   local retry store and an alert fires.
3. **Given** both streaming and scheduled (hourly/daily) modes are
   available, **When** the admin selects streaming, **Then** records
   flush within the configured latency target; **When** the admin
   selects scheduled, **Then** batches flush at the configured cron
   interval.

---

### User Story 12 — Executive Reporting Dashboard (Priority: P3)

As a **VP of Engineering** or **CFO**, I can view an org-level
executive dashboard aggregating AI adoption (active users, workspaces,
requests/week trend), ROI (cost/request trend, cost per workspace,
top-cost users), and strategic impact (models in production, providers
in use, guardrail save rate) so that I can report AI program health to
the board.

**Why this priority**: C-suite visibility drives funding continuation.
Engineering dashboards are too granular for executive consumption.

**Independent Test**: As an Org Owner, navigate to the Executive
Dashboard; verify the page shows trailing 30-day totals and
week-over-week deltas for: total requests, total cost, unique active
users, workspaces active, top 5 models by cost, top 5 workspaces by
cost, PII-redaction saves, guardrail-blocked incidents.

**Parity mapping**: https://portkey.ai/docs/product/observability

**Acceptance Scenarios**:

1. **Given** an Org Owner, **When** they visit the Executive Dashboard,
   **Then** aggregated metrics render within 3 seconds for an org with
   up to 10M logs/month.
2. **Given** a date range selector, **When** the user changes the range,
   **Then** all charts update consistently and week-over-week deltas
   recompute against the new comparison window.
3. **Given** workspace-scoped users, **When** they attempt to access
   the Executive Dashboard, **Then** they see 403 with a message
   indicating the dashboard is org-level only.

---

### User Story 13 — Custom Retention Periods (Priority: P3)

As a **Data Protection Officer**, I can configure per-workspace log and
metric retention periods (e.g., 30 days for one workspace, 7 years for
a HIPAA-regulated workspace) so that we retain only what we are legally
required to and no longer.

**Why this priority**: GDPR Article 5(1)(e) mandates storage limitation.
Over-retention creates breach liability; under-retention breaks
healthcare audit requirements.

**Independent Test**: Configure Workspace A with 30-day retention and
Workspace B with 1-year retention; backdate test log records to 31
days old in both; run the retention job; verify Workspace A's old
records are deleted and Workspace B's are retained, and that an audit
entry records the retention-driven deletion count.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering

**Acceptance Scenarios**:

1. **Given** a workspace retention policy, **When** the scheduled
   retention job runs, **Then** log and metric records older than the
   policy horizon for that workspace are deleted (or moved to cold
   storage if configured) and a summary audit entry records the count.
2. **Given** a retention policy change from 30 to 90 days, **When** the
   change takes effect, **Then** records in the 30–90 day range are
   retained going forward and records previously deleted remain deleted
   (no un-delete).

---

### User Story 14 — Prompt Library with Versioning (Priority: P4)
*(status per SR-01 2026-04-20: **basic library IN SCOPE** — already satisfied by `plugins/prompts/` re-enable in Phase 1 T012. **Deployment-strategies subset (A/B, canary, production pointer) DESCOPED** — T068–T070 stay as `[~]`; needs its own spec.)*

As a **Prompt Engineer**, I can author prompt templates with variable
substitution and reusable fragments (partials), commit versioned
revisions, organize them into folders with per-folder access controls,
and support multimodal content (text + images) so that the
organization's prompt assets are centrally managed and production
callers reference a named+versioned template rather than hardcoding
prompts.

**Why this priority**: Prompt-library maturity is a 2026 table-stakes
feature; teams without it end up with prompt drift across repos and
inconsistent outputs.

**Independent Test**: Create a prompt "customer-support-v1" with a
`{{customer_name}}` variable; call the `/prompts/customer-support-v1/
completions` endpoint with `{"variables": {"customer_name": "Alice"}}`;
verify the rendered prompt replaces the variable and the call returns
a provider response; edit the prompt and publish v2; confirm a caller
pinned to v1 still receives the v1 rendering.

**Parity mapping**: https://portkey.ai/docs/product/prompt-library

**Acceptance Scenarios**:

1. **Given** a prompt template with variables and partials, **When** a
   caller hits the prompt completion endpoint with variable values,
   **Then** variables are substituted, partials are expanded, and the
   fully rendered prompt is sent to the configured model.
2. **Given** multiple versions of a prompt, **When** a caller references
   the prompt with or without an explicit version, **Then** the
   addressed version (or the designated "production" version if
   unspecified) is rendered.
3. **Given** a prompt folder with a scoped role, **When** a user without
   the scope attempts to view or edit prompts in that folder, **Then**
   they receive 403.
4. **Given** a prompt with deployment strategies configured, **When** a
   caller hits the completions endpoint without a version pin, **Then**
   the deployment strategy (A/B split, canary %, or pinned production
   version) determines which version is rendered. The
   `prompt-deployments` UI stub at `/workspace/prompt-repo` is filled
   with a real deployment-strategy editor (not ContactUsView).

---

### User Story 15 — Prompt Playground & Side-by-Side Comparison (Priority: P4)

As a **Prompt Engineer**, I can open any prompt template in an
interactive playground that lets me run the same prompt against
multiple models (e.g., GPT-4o vs Claude 4.6 vs Gemini 2.5) side-by-side
and compare outputs, latencies, and costs so that I can make
evidence-based model selection decisions before shipping to production.

**Why this priority**: Teams without comparison tooling make model
choices on vibes, leading to either over-paying for premium models on
simple tasks or under-serving complex tasks with cheap models.

**Independent Test**: Open a prompt in the playground, select three
models, submit; verify three response cards render with the full
response text, token counts, latency, and cost per call; verify the
session is optionally saveable for sharing.

**Parity mapping**: https://portkey.ai/docs/product/prompt-library

**Acceptance Scenarios**:

1. **Given** the playground, **When** the user selects N models and
   clicks run, **Then** N parallel requests are issued, each result
   renders as it arrives (no blocking), and a summary row shows
   per-model latency, token counts, and cost.
2. **Given** multiple playground runs, **When** the user saves a
   session, **Then** the session (models, variables, outputs) is
   retained in the workspace and shareable with other users who have
   prompt read-scope.

---

### User Story 16 — Declarative Config Objects (Priority: P4)

As a **Platform Engineer**, I can define a named, versioned "Config"
object — declarative JSON describing a routing strategy (fallbacks,
retries, cache settings, attached guardrails, metadata tags) — and
attach it to inbound requests via a single `x-config-id` header so
that my application code stays simple and the gateway's routing
behavior is managed as data.

**Why this priority**: Hardcoded routing in application code resists
experimentation. A versioned, header-attached config object is the
standard pattern (Portkey's Config primitive) that enables canary
rollouts and rollbacks from the gateway side.

**Independent Test**: Define a Config with primary=openai/gpt-4o,
fallback=anthropic/claude-sonnet-4.6, cache=semantic-60s; attach it
via header; verify a simulated OpenAI 500 response triggers the
Anthropic fallback on retry; verify repeated identical prompts hit
the cache within 60 seconds.

**Parity mapping**: https://portkey.ai/docs/product/ai-gateway

**Acceptance Scenarios**:

1. **Given** a Config is registered in a workspace, **When** a request
   arrives with `x-config-id: <id>`, **Then** the gateway applies the
   Config's routing, fallback, retry, cache, and guardrail directives
   for that request.
2. **Given** a Config change, **When** the admin publishes a new
   version, **Then** callers default to the latest unless they pin a
   specific version in their header.

---

### User Story 17 — Service Account API Keys (Priority: P4)

As a **Platform Engineer**, I can create non-user "service account"
API keys scoped to a single workspace with their own budget and rate
limits so that production services authenticate without being tied to
a human user's lifecycle (offboarding, vacation, etc.).

**Why this priority**: Tying production auth to human identities is a
common compliance finding. Service accounts are the standard pattern.

**Independent Test**: Create a service account key scoped to Workspace A
with a monthly $500 budget; use it from a CI job to make requests;
verify requests succeed, analytics attribute cost to the service
account, and the service account has no login / UI access.

**Parity mapping**: https://portkey.ai/docs/api-reference/admin-api/introduction

**Acceptance Scenarios**:

1. **Given** a service account key, **When** it is used to authenticate
   against an API endpoint, **Then** the request is attributed to the
   service account in logs and analytics.
2. **Given** a service account, **When** an admin rotates its key,
   **Then** the old key stops working within 60 seconds and all
   in-flight requests using it are rejected on next call.

---

### User Story 18 — BYOK / Customer-Managed Encryption Keys (Priority: P5)

As a **Cloud Security Architect**, I can configure Bifrost to encrypt
sensitive data at rest (provider API keys, prompt templates, logs)
using a customer-managed key in AWS KMS, Azure Key Vault, or GCP KMS
so that revoking the external key immediately renders Bifrost data
unreadable (crypto-shredding).

**Why this priority**: Several regulated verticals (financial services,
US federal) require customer-held encryption keys as a contractual
condition of purchase.

**Independent Test**: Configure AWS KMS BYOK with a customer CMK;
create a virtual key (stored encrypted); disable the CMK in AWS; verify
all subsequent reads of encrypted fields fail with a clear
"unavailable: key disabled" error; re-enable the CMK and verify access
restores.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/security-portkey

**Acceptance Scenarios**:

1. **Given** BYOK is configured, **When** Bifrost writes a sensitive
   field (provider API key, webhook secret, prompt template), **Then**
   the field is encrypted using a data key wrapped by the customer
   CMK.
2. **Given** a CMK rotation event, **When** the CMK rotates in the
   customer KMS, **Then** Bifrost seamlessly decrypts with the previous
   version and re-encrypts with the new version on next write.
3. **Given** a CMK access revocation, **When** Bifrost attempts to
   decrypt, **Then** the operation fails with a clear error naming the
   KMS failure and the operation is retried per policy before being
   surfaced to the user.

---

### User Story 19 — Private-Cloud / VPC / Air-Gapped Deployments (Priority: P5)

As a **DevOps Engineer** at a regulated firm, I can deploy Bifrost
entirely inside my VPC (or air-gapped on-prem datacenter) using
provided Helm charts and Terraform modules with a documented
"air-gapped" profile that works without any outbound calls to
Bifrost-owned infrastructure or phone-home telemetry.

**Why this priority**: Air-gapped deployments are contractual for US
federal and financial-services clients. Portkey's hybrid/VPC/air-gapped
story is a closed-deals driver.

**Independent Test**: Using the air-gapped Helm profile, deploy
Bifrost into an EKS cluster with egress blocked except to AWS APIs;
verify all features listed as "air-gapped supported" function end-to-
end; inspect network traffic and confirm zero calls leave the VPC.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/private-cloud-deployments

**Acceptance Scenarios**:

1. **Given** the published Helm chart with the `profile: airgapped`
   value set, **When** an engineer runs `helm install`, **Then** the
   deployment succeeds with internal container registry references
   only, no phone-home telemetry, and a documented set of features
   marked as supported in air-gapped mode.
2. **Given** the Terraform module for AWS/Azure/GCP, **When** the
   engineer runs `terraform apply`, **Then** the module creates the
   full Bifrost stack (compute, database, object storage, KMS
   configuration) per variables with sensible defaults and clear docs.

---

### User Story 20 — SCIM 2.0 User Provisioning (Priority: P5)

As an **IT Administrator**, I can connect my IdP's SCIM 2.0 endpoint
to Bifrost so that user lifecycle events (create, update role, suspend,
delete) in the IdP automatically propagate to Bifrost without manual
user management.

**Why this priority**: IT teams at enterprises of any size refuse to
manually mirror user changes across every SaaS tool. SCIM is the
universal standard.

**Independent Test**: Connect Okta via SCIM; deprovision a user in
Okta; within 10 minutes verify the user's Bifrost sessions are
terminated and their Admin API Keys are revoked with audit entries.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/org-management/sso

**Acceptance Scenarios**:

1. **Given** SCIM is configured, **When** the IdP creates a user,
   **Then** Bifrost provisions a matching user with mapped role and
   workspace memberships.
2. **Given** a user is suspended in the IdP, **When** the SCIM update
   arrives, **Then** the Bifrost user's active sessions are terminated,
   their keys are revoked, and an audit entry records the
   deprovisioning.

---

### User Story 21 — Terraform Provider (Priority: P6)

As a **Platform Engineer**, I can manage Bifrost resources
(workspaces, virtual keys, configs, guardrail policies, admin API
keys) as Terraform resources so that our entire Bifrost estate is
version-controlled as IaC alongside the rest of our infrastructure.

**Why this priority**: IaC-first teams will not adopt a tool that only
exposes GUI or ad-hoc API management.

**Independent Test**: Publish a Terraform example creating a
workspace, two virtual keys, and one config; `terraform apply`;
verify resources exist in Bifrost; modify the Terraform and re-apply;
verify drift is corrected; `terraform destroy`; verify cleanup.

**Parity mapping**: Portkey official Terraform provider (reference
implementation).

**Acceptance Scenarios**:

1. **Given** the published Terraform provider, **When** an engineer
   declares `bifrost_workspace`, `bifrost_virtual_key`,
   `bifrost_config`, `bifrost_guardrail`, `bifrost_admin_api_key`
   resources, **Then** `terraform plan` and `apply` correctly create,
   update, and destroy them against the Admin API.

---

### User Story 22 — Canary Testing & Traffic-Split Routing (Priority: P6)

As a **Prompt Engineer**, I can configure a Config that routes a
percentage of traffic to a new model (e.g., 10% to
`openai/gpt-5-preview`, 90% to current production) and view side-by-
side metrics (latency, cost, error rate, feedback score) so that
I can promote the new model with evidence.

**Why this priority**: Safe rollout of new LLM versions without code
changes is the routing-layer's most valuable feature.

**Independent Test**: Configure a 10/90 canary; send 1,000 requests;
verify ~100 were routed to the canary target; inspect the comparison
view showing parallel latency/cost/error metrics for both legs.

**Parity mapping**: https://portkey.ai/docs/product/ai-gateway

**Acceptance Scenarios**:

1. **Given** a Config with a canary rule, **When** requests arrive,
   **Then** the configured traffic percentage is routed to the canary
   target within statistical tolerance (±2% over 1,000 requests).
2. **Given** a canary in progress, **When** an engineer views the
   comparison report, **Then** the report shows per-leg latency
   percentiles, cost per request, error rate, and feedback score.

---

### User Story 23 — Recurring ETL Export to Customer Data Lakes (Priority: P6)

As a **Data Engineer**, I can define recurring, transformed exports
from Bifrost to customer-owned data lakes (S3, Wasabi, BigQuery) in
the customer's preferred schema and format so that Bifrost telemetry
joins the existing analytical pipeline with minimal ETL work on our
side.

**Why this priority**: Raw log export (US11) is a file drop. Mature
data teams want curated, schema-stable, transformed datasets
(completions, aggregated spend, guardrail events) on a schedule.

**Independent Test**: Define an export "daily-completions-to-bigquery"
selecting a curated subset of fields with a documented schema; wait
24 hours; query BigQuery and verify the partitioned table contains
the day's records with the declared schema.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/components

**Acceptance Scenarios**:

1. **Given** a curated export definition, **When** the scheduled run
   executes, **Then** the transformed dataset is written to the
   target in the declared schema and the run's success or failure is
   logged and alerted.

---

### User Story 30 — Enterprise Platform Features (Priority: P3)

As a **Platform Engineer**, I can configure MCP tool groups (grouping
and managing MCP tools per workspace), MCP auth configuration (OAuth
credentials for MCP server connections), large payload optimization
(streaming and chunking for oversized request/response bodies), and
proxy/SCIM proxy settings so that all enterprise-gated platform
capabilities are functional — not just marketing stubs.

**Why this priority**: These 4 features are currently visible as
"Contact Us" stubs in the enterprise UI. Each has a pre-wired route
and sidebar entry. Filling them completes 100% enterprise surface
coverage.

**Scope**:
- **MCP Tool Groups** (`/workspace/mcp-tool-groups`): manage named
  groups of MCP tools assignable to virtual keys. Backend: existing
  MCP handler has tool/client CRUD; UI stub needs real implementation.
- **MCP Auth Config** (`/workspace/mcp-auth-config`): configure OAuth
  credentials for authenticated MCP server connections. Backend:
  existing OAuth2 infrastructure; UI stub needs real implementation.
- **Large Payload Settings** (fragment in client settings): configure
  streaming thresholds and chunking for large request/response bodies.
  Backend: governance plugin already supports large-payload streaming
  mode (`IsLargePayloadMode`); UI stub needs real implementation.
- **Proxy Config** (`/workspace/config/proxy`): enterprise proxy
  settings including SCIM proxy endpoint configuration. Backend:
  proxy config handler exists; UI stub needs SCIM section filled.

**Independent Test**: Navigate to each of the 4 routes in enterprise
mode; verify real UI (not ContactUsView) renders with functional
forms backed by API calls.

**Acceptance Scenarios**:

1. **Given** enterprise mode, **When** a user navigates to
   `/workspace/mcp-tool-groups`, **Then** they see a functional tool
   group management interface (not a "Contact Us" stub).
2. **Given** enterprise mode, **When** a user navigates to
   `/workspace/mcp-auth-config`, **Then** they see OAuth credential
   configuration for MCP servers.
3. **Given** enterprise mode, **When** a user views client settings,
   **Then** the large payload settings fragment renders with
   configurable streaming thresholds.
4. **Given** enterprise mode, **When** a user navigates to
   `/workspace/config/proxy`, **Then** the proxy page includes the
   SCIM proxy section (not "SCIM proxy support is available in
   Bifrost Enterprise" placeholder).

---

### User Story 24 — License Activation & Entitlement Enforcement (Priority: P1, self-hosted only)

As a **Self-hosted Operator**, I can upload a signed license file
(issued by the vendor) that Bifrost verifies offline using an
embedded public key; the license unlocks exactly the enterprise
features purchased for the purchased term, with clear visibility
into which features are enabled and when the license expires.

**Why this priority**: Self-hosted commercial distribution requires
entitlement enforcement. Without it there is no commercial product
at all for on-prem customers.

**Independent Test**: Upload a license file with entitlements
[workspaces, rbac, audit-logs] and `expires_at = +30 days`; verify
all three user stories work; observe Guardrails is NOT active
because it was not in the entitlement list. Try uploading a license
with a tampered signature; verify rejection with a clear error.

**Parity mapping**: Not a Portkey parity item — this is a
commercial-distribution requirement specific to Bifrost's
self-hosted SKU.

**Acceptance Scenarios**:

1. **Given** a valid signed license file, **When** the operator
   uploads it via UI or places it at the configured path, **Then**
   Bifrost verifies the signature against the embedded vendor public
   key, parses claims (customer name, issued_at, expires_at,
   entitlements, max_seats, max_workspaces), and displays a
   confirmation including expiry date and entitlement list.
2. **Given** an uploaded license, **When** a gated feature is
   invoked, **Then** the license plugin's
   `IsEntitled(feature_name)` check returns true/false and the
   feature either executes or returns a clear "feature-not-licensed"
   error (HTTP 402 or equivalent).
3. **Given** a license with invalid signature or unparseable body,
   **When** upload is attempted, **Then** Bifrost rejects with a
   specific reason (signature mismatch / malformed claims / wrong
   key) and does NOT activate.
4. **Given** the air-gapped profile (US19), **When** a license is
   uploaded, **Then** verification completes without any network
   call (offline-verifiable design).

---

### User Story 25 — License Expiry & Grace Period (Priority: P2, self-hosted only)

As a **Self-hosted Operator**, when my license approaches expiry I
receive proactive alerts, after expiry I have a 14-day grace period
during which everything continues to work with a prominent UI banner,
and after grace-period expiry enterprise features disable gracefully
(with data intact) while OSS core continues unchanged.

**Why this priority**: Hard-expiring features in production is a
customer-destroying experience. Graceful degradation + grace period
is industry standard (Elastic, GitLab) and materially reduces
incident risk.

**Independent Test**: Install a license expiring in 2 days; observe
UI warning banners at 30-days/7-days/1-day. Time-travel the system
clock past expiry; observe the 14-day grace banner + daily audit
entries. Time-travel past grace; observe enterprise features return
"license-expired" errors while OSS endpoints continue to respond
normally; existing audit data remains readable.

**Parity mapping**: N/A (commercial-distribution requirement).

**Acceptance Scenarios**:

1. **Given** a license with `expires_at` in the future, **When** the
   remaining time crosses thresholds at 30 / 7 / 1 days, **Then** a
   UI banner appears with escalating severity and an email alert
   fires to the license-contact address.
2. **Given** a license past `expires_at` but within 14-day grace,
   **When** any enterprise feature is invoked, **Then** it continues
   to operate, a daily audit entry `license.grace_period_active`
   records the state, and a critical UI banner is visible to all
   admins.
3. **Given** a license past `expires_at + 14 days`, **When** an
   enterprise feature is invoked, **Then** it returns HTTP 402 with
   a clear renewal link, while OSS endpoints (chat/completions etc.)
   continue to serve unchanged. Existing enterprise data (audit
   logs, workspaces, prompts) is READABLE but not modifiable until
   renewed.
4. **Given** a renewed license upload, **When** validation succeeds,
   **Then** all enterprise features re-enable within 60 seconds
   without a restart.

---

### User Story 26 — Per-Organization Usage Metering (Priority: P1, cloud only)

As a **Bifrost Cloud Operator**, I can accurately attribute every
request's cost to the originating organization with daily and
monthly rollups, so that I can invoice customers correctly and
show them a running cost breakdown in their billing portal.

**Why this priority**: Metering is the load-bearing primitive for
everything else in Train E. Without accurate per-org usage, neither
invoicing (US27) nor the billing portal (US28) nor tier enforcement
(US29) can function.

**Independent Test**: Drive 10,000 requests across 3 organizations
with mixed model usage; query the daily-metering rollup; verify
per-org request counts, input/output tokens, and total cost in USD
match a replay-computed reference within 0.1% tolerance.

**Parity mapping**: https://portkey.ai/docs/product/enterprise-offering/components

**Acceptance Scenarios**:

1. **Given** a cloud deployment, **When** requests are served,
   **Then** each request increments per-org counters (requests,
   input_tokens, output_tokens, cost_usd_micros, cache_hits) in
   real time and is included in a daily rollup within 15 minutes of
   day close.
2. **Given** daily rollups for 30 days, **When** the monthly
   billing job runs, **Then** a monthly statement is produced per
   organization with line items by model, by workspace, by virtual
   key.
3. **Given** the `deployment.mode` is NOT `cloud`, **When** the
   system boots, **Then** the metering plugin does not load; no
   metering tables are populated; no cost is visible to the
   operator in the UI.

---

### User Story 27 — Subscription & Usage Billing via Stripe (Priority: P1, cloud only)

As a **Bifrost Cloud Operator**, I can integrate with Stripe
(subscription + metered usage APIs) so that customers are billed
monthly with a base subscription + usage overage, with failed
payments retried per Stripe's recovery process and dunning
notifications sent automatically.

**Why this priority**: Invoicing is the revenue path. Without
automation, the finance team becomes the bottleneck for every new
customer.

**Independent Test**: Create a cloud customer with subscription tier
`Prod` ($49/mo + $9/100k requests overage); drive 150k requests in
a billing period; verify Stripe receives a $49 subscription charge +
$4.50 overage charge at period close; verify invoice line items
render correctly in the billing portal.

**Parity mapping**: https://portkey.ai/pricing

**Acceptance Scenarios**:

1. **Given** a customer with an active Stripe subscription, **When**
   the monthly billing cycle closes, **Then** Bifrost pushes usage
   records (via Stripe's Metered Billing API) for overage items,
   Stripe generates the invoice, and the customer is charged.
2. **Given** a failed payment, **When** Stripe signals failure via
   webhook, **Then** Bifrost enters the customer into a dunning
   state, enforces a read-only grace period for 14 days, and
   reverts to full access on payment success.
3. **Given** a subscription cancellation, **When** the cancellation
   takes effect at period end, **Then** the customer's org is
   downgraded to Dev tier (free, limited) without data loss.

---

### User Story 28 — Customer Billing Portal (Priority: P2, cloud only)

As a **Cloud Customer Admin**, I can view my organization's usage
history, current billing cycle cost, payment method, and past
invoices in a dedicated portal, and I can update my payment method
or download invoices without contacting support.

**Why this priority**: Self-service billing reduces support burden
and matches enterprise buyer expectations.

**Independent Test**: As customer admin, visit **Billing** → view
current-period usage chart + projected cost; update payment method
via Stripe-hosted flow; download last 3 monthly invoices as PDFs.

**Parity mapping**: https://portkey.ai/pricing (customer portal pattern)

**Acceptance Scenarios**:

1. **Given** a customer admin, **When** they visit the billing
   portal, **Then** they see current-cycle usage (requests, tokens,
   cost), projected cost for the cycle, and the payment method on
   file.
2. **Given** a customer admin, **When** they initiate a payment-
   method update, **Then** they're redirected through Stripe's
   hosted payment-method update flow and returned with the new
   method active.
3. **Given** past invoices, **When** the admin downloads one,
   **Then** a branded PDF is generated via Stripe's invoice API
   with per-line-item detail.

---

### User Story 29 — Tier & Plan Management (Priority: P2, cloud only)

As a **Cloud Customer Admin**, I can upgrade my organization from
Dev → Prod → Enterprise (or downgrade) via self-service, with the
new tier's features + limits applying within 60 seconds of payment
success.

**Why this priority**: Friction-free tier upgrades convert more
customers than any sales motion. A "contact sales" button for Prod
signup is a 2020 pattern.

**Independent Test**: As a Dev-tier admin, click Upgrade to Prod,
complete payment via Stripe Checkout; within 60 seconds the org's
rate limits increase, log retention expands, guardrails become
available, and a confirmation email is sent.

**Parity mapping**: https://portkey.ai/pricing

**Acceptance Scenarios**:

1. **Given** a Dev-tier customer, **When** they upgrade to Prod via
   self-service, **Then** Stripe charges the subscription, Bifrost
   updates the org's tier within 60 seconds, and tier-dependent
   features (retention, rate limits, guardrails) reflect the new
   tier.
2. **Given** an Enterprise-tier conversion, **When** a customer
   requests Enterprise, **Then** the UI captures the request and
   routes it to a sales workflow (no self-service for Enterprise
   SKU — confirmed by stakeholder intent).
3. **Given** a downgrade, **When** the downgrade takes effect,
   **Then** features beyond the new tier are disabled at period
   boundary (not mid-cycle) to avoid surprising the customer.

---

### Edge Cases

- **Upgrade path (zero-config)**: An existing Bifrost v1.5.2 OSS
  deployment starts the enterprise-capable build. Behavior MUST be
  byte-identical by default. Enabling any enterprise feature is opt-in
  via config. Existing virtual keys, teams, customers, providers, and
  logs MUST remain queryable via the same APIs with the same IDs.
- **Workspace deletion with active keys**: If a workspace is deleted,
  active virtual keys are revoked (not deleted), incoming requests
  using them return a clear error, and the audit trail records the
  revocation. Deletion is soft for 30 days to allow recovery.
- **Conflicting guardrails**: When org-wide, workspace, and per-key
  guardrails all apply, execution order is org → workspace → key,
  and a deny at any level wins.
- **SSO outage**: If the IdP is unreachable, currently-active sessions
  continue until expiry, but no new sessions can be established. The
  admin can enable a documented "break-glass" local login for a
  specific set of emergency users, with its usage audited and alerted.
- **KMS outage with BYOK**: Read operations cache decrypted data keys
  in memory for a configurable TTL; the admin is alerted when KMS is
  unreachable; writes requiring new data-key generation fail fast
  rather than silently falling back to an unencrypted path.
- **Clock skew in rate limits**: Rate-limit windows use a monotonic
  server-side counter; individual instance clock drift up to 30s does
  not cause limits to reset early. Limits are enforced per-key
  globally across instances via a shared counter backend.
- **Admin API Key rotation with active callers**: Rotation creates a
  new key and marks the old one "revoking" with a grace period
  (default 60s) before hard revocation, so callers can rotate without
  downtime.
- **Multi-region data residency**: Workspace data residency is a
  configured field; logs and metrics for that workspace are stored
  only in the designated region's storage backend. Cross-region
  replication requires explicit admin opt-in with a compliance
  warning.

## Requirements *(mandatory)*

### Functional Requirements

**Multi-Tenancy & Identity:**

- **FR-001**: System MUST support a hierarchy of Organization →
  Workspace → (Members, Virtual Keys, Prompts, Configs, Guardrails,
  Model Catalog, Provider Configs), with complete data isolation
  between organizations. Every tenant-scoped table or collection
  MUST carry an `organization_id` column/field from the initial
  release, even when the deployment operates in single-org mode, so
  that activation of multi-org (via the `multi_org_enabled` flag)
  does not require a schema migration.
- **FR-001a**: In v1, each deployment hosts a single organization by
  default. A `multi_org_enabled` configuration flag MAY activate
  multi-organization mode in a future release; API surfaces MUST
  return the current organization context regardless of mode so
  clients behave identically.
- **FR-002**: System MUST auto-migrate pre-existing v1.5.2 data into
  a default organization + default workspace on first startup of the
  enterprise-capable build, with zero configuration required.
- **FR-003**: System MUST persist four default roles — Owner, Admin,
  Manager, Member — plus an unbounded set of customer-defined custom
  roles. (v1 enforcement scope, per clarification 2026-04-20: role
  model is **stored and consumed by the UI's `useRbac` gate**;
  server-side scope enforcement on admin handlers is explicitly out
  of v1 scope and needs its own feature spec.)
- **FR-004**: System MUST support per-resource scopes matching the
  upstream UI's 24-resource RBAC enum (GuardrailsConfig,
  GuardrailsProviders, GuardrailRules, UserProvisioning, Cluster,
  Settings, Users, Logs, Observability, VirtualKeys, ModelProvider,
  Plugins, MCPGateway, AdaptiveRouter, AuditLogs, Customers, Teams,
  RBAC, Governance, RoutingRules, PIIRedactor, PromptRepository,
  PromptDeploymentStrategy, AccessProfiles) with 6 operations
  (Read, View, Create, Update, Delete, Download). (v1 enforcement
  scope, per clarification 2026-04-20: scopes are UI-gated only —
  see FR-003 note and `docs/enterprise/rbac.mdx` §"Notes & limits".)
- **FR-005** *(DESCOPED 2026-04-20 per SR-01 — US3 net-new; needs own spec)*: System MUST support SSO via SAML 2.0 and OIDC with at
  minimum Okta, Azure AD / Entra ID, Google Workspace, and generic
  OIDC compliance.
- **FR-006** *(DESCOPED 2026-04-20 per SR-01 — US3)*: System MUST auto-provision users on first SSO login and
  auto-accept pending invitations matching the authenticated email.
- **FR-007** *(DESCOPED 2026-04-20 per SR-01 — US20 SCIM net-new; needs own spec)*: System MUST support SCIM 2.0 for user lifecycle
  management from external IdPs.
- **FR-008** *(DESCOPED 2026-04-20 per SR-01 — US5; upstream `auth_config` basic-auth already covers admin auth)*: System MUST support org-level Admin API Keys with per-
  key scopes, creator tracking, expiration, and one-time display at
  creation.
- **FR-009** *(DESCOPED 2026-04-20 per SR-01 — US17 net-new; needs own spec)*: System MUST support service-account API Keys scoped to
  a single workspace with their own budgets, rate limits, and no UI
  login capability.

**Audit & Governance:**

- **FR-010**: System MUST record an audit entry for every
  administrative or governance action including actor ID, actor IP,
  action type, resource type, resource ID, before/after values,
  outcome (allowed/denied), request ID, and ISO 8601 timestamp.
  **Attribution in v1 (no session→user middleware yet, per
  clarification 2026-04-20):** when an admin action reaches a
  handler via the synthetic default tenant resolver (upstream basic-
  auth credential, single shared identity), audit entries carry
  `actor_type="system"`, `actor_id="upstream-admin"`. Real per-user
  attribution activates automatically once tenant-resolution
  middleware lands in a later spec — `emitAudit` callers require no
  change.
- **FR-011**: System MUST expose audit log query with filters
  (actor, action, resource, time range, outcome) and export to CSV
  and JSON, complete (no truncation) up to at least 10M entries.
- **FR-012**: System MUST persist audit entries for a duration at
  least as long as the longest configured retention policy, and in
  no case less than 1 year when audit logging is enabled. Per
  clarification 2026-04-20, audit rows live in the **configstore**
  (co-located with `ent_roles`, `ent_users`, and governance tables
  they reference — a `role.create` entry sits alongside its row).
  Automatic retention-based cleanup is NOT in v1 scope; rows
  accumulate indefinitely until the operator runs cleanup SQL or a
  retention task ships in its own spec.

**Guardrails** *(FR-013..FR-018 DESCOPED 2026-04-20 per SR-01 — US6/US9 net-new `plugins/guardrails-central/`; needs own spec)*:

- **FR-013**: System MUST support organization-scoped guardrails
  applied before workspace-scoped and per-key guardrails.
- **FR-014**: System MUST support deterministic guardrail types
  (regex, JSON schema, word/character/sentence count, code-language
  detection, PII detection) and LLM-based guardrail types (prompt
  injection, gibberish, semantic-policy check).
- **FR-015**: System MUST support partner guardrail integrations
  (Aporia, Pillar, Patronus, SydeLabs, Pangea) via adapter pattern.
- **FR-016**: System MUST support custom guardrail webhooks with
  HMAC request signing, configurable timeout, and fail-open /
  fail-closed policy.
- **FR-017**: System MUST support guardrail execution modes:
  synchronous vs asynchronous, parallel vs sequential, and composition
  policies (any-deny, all-must-pass, majority).
- **FR-018**: System MUST support guardrail actions: deny with HTTP
  446, allow-with-warning with HTTP 246, retry, fallback to a
  configured alternate, and log-only.
- **FR-019** *(DESCOPED 2026-04-20 per SR-01 — US7 net-new `plugins/pii-redactor/`; needs own spec)*: System MUST apply PII redaction to request and response
  payloads before persistence to logs, with configurable mode
  (redact-in-logs-only vs redact-before-provider), a configurable PII
  pattern set, and a fail-closed mode.

**Budgets & Rate Limits:**

- **FR-020** *(ALREADY_SHIPPED upstream — `plugins/governance/`)*: System MUST enforce per-virtual-key budget caps
  with configurable reset durations via `governance_budgets` table.
  No new implementation required.
- **FR-021** *(ALREADY_SHIPPED upstream — `plugins/governance/`)*: System MUST enforce per-virtual-key rate limits
  on tokens and requests via `governance_rate_limits` table. No new
  implementation required.
- **FR-022** *(DELIVERED 2026-04-20 as T055 — defaults-only in v1, per-budget configurability deferred)*: System MUST fire threshold alerts at 50%, 75%, and 90% of
  budget to available alert destinations before budget exhaustion.
  Delivered via `plugins/governance/tracker_thresholds.go` (sibling-
  file extension); destinations in v1 = `logger.Warn` + WebSocket
  push. Per-budget configuration of the threshold levels and
  external destinations (Slack / webhook / email per US10) are
  explicitly OUT of v1 scope; both need their own feature spec.
- **FR-023** *(ALREADY_SHIPPED upstream — `plugins/governance/resolver.go`)*: System MUST return HTTP 429 / 402.
  `DecisionRateLimited` and `DecisionBudgetExceeded` already map to
  HTTP 429 / 402 in the transport.

**Alerts & Observability:**

- **FR-024** *(DESCOPED 2026-04-20 per SR-01 — US10 net-new `plugins/alerts/`; needs own spec)*: System MUST support threshold-based alert rules on
  error rate, p50/p95/p99 latency, cost-per-hour, and feedback score,
  with multi-destination delivery (webhook, Slack, email).
- **FR-025** *(DESCOPED 2026-04-20 per SR-01 — US11 net-new `plugins/logexport/`; needs own spec)*: System MUST support log export to at minimum AWS S3,
  Azure Blob, Google Cloud Storage, MongoDB/DocumentDB, and OTLP
  collector, with both streaming and scheduled modes.
- **FR-026**: System MUST offer an executive dashboard at organization
  level aggregating adoption, cost, and guardrail metrics across
  all workspaces.
- **FR-027**: System MUST support per-workspace configurable retention
  periods for logs and metrics.
- **FR-028**: Every enterprise feature (per the Constitution) MUST
  emit OpenTelemetry spans, Prometheus metrics, and audit log
  entries.

**Prompts & Configs:**

- **FR-029** *(ALREADY_SHIPPED upstream — `plugins/prompts/` re-enabled in Phase 1 T012)*: System MUST support prompt templates with variable
  substitution (Mustache-compatible), partials (reusable fragments),
  folder organization with per-folder access scopes, and full version
  history. Persisted in the existing configstore (reusing its multi-
  tenancy, encryption, and hot-reload infrastructure). A dedicated
  `framework/promptstore/` subsystem is explicitly deferred to v2
  and is out of scope for this feature. **Note:** the "designated
  production version per prompt" pointer and deployment strategies
  (A/B split, canary) are DESCOPED per SR-01 clarification 2026-04-20
  (see US14-deployments in the SR-01 table — net-new extension of
  the prompts plugin; needs its own spec).
- **FR-030** *(ALREADY_SHIPPED upstream — `plugins/prompts/` exposes these endpoints; re-enabled in Phase 1 T012)*: System MUST expose `/prompts/<id>/render` and
  `/prompts/<id>/completions` endpoints that accept variables and
  return the rendered or fully-executed completion.
- **FR-031** *(ALREADY_SHIPPED upstream — `plugins/prompts/`)*: System MUST support multimodal prompts combining text
  and images.
- **FR-032** *(DESCOPED 2026-04-20 per SR-01 — US15 net-new playground handler; needs own spec)*: System MUST support an interactive Playground for
  running a single prompt against N selected models in parallel,
  displaying streaming per-model outputs with latency, token count,
  and cost.
- **FR-033** *(DESCOPED 2026-04-20 per SR-01 — US16 net-new handler + storage; needs own spec)*: System MUST support declarative Config objects
  (JSON) describing primary + fallback providers, retry policy,
  cache policy, attached guardrails, and metadata, addressable via
  `x-config-id` header with optional version pin.
- **FR-034** *(DESCOPED 2026-04-20 per SR-01 — US22 depends on US16; needs own spec)*: System MUST support canary routing as a primitive
  defined WITHIN the declarative Config object (not within the
  governance plugin's routing-chain). The Config canary primitive
  declaratively splits a configured percentage of traffic to a
  specified target with comparison reporting. Governance plugin
  routing-chain logic remains scoped exclusively to budget and
  virtual-key enforcement and MUST NOT be extended to implement
  canary behavior.

**Security & Deployment:**

- **FR-035** *(DESCOPED 2026-04-20 per SR-01 — US18 net-new `framework/kms/` module; needs own spec)*: System MUST support BYOK for at-rest encryption with
  AWS KMS, Azure Key Vault, and GCP KMS. By default, BYOK encrypts
  configstore-managed secrets (provider API keys, webhook secrets,
  admin and service-account API keys, prompt templates, OAuth
  client secrets). The customer-managed key wraps a per-record
  data encryption key with in-memory caching.
- **FR-035a** *(DESCOPED 2026-04-20 per SR-01 — US18)*: BYOK encryption of logstore request/response
  payloads MUST be supported as an opt-in per-workspace setting
  with a configurable data-key cache TTL (default 15 minutes per
  SC-009). When the setting is disabled, logstore writes use the
  configstore encryption key as before.
- **FR-036**: System MUST never log or emit provider API keys,
  webhook secrets, admin API keys, or OAuth client secrets in
  plaintext to any logstore, metric label, trace attribute, or
  external export.
- **FR-037** *(DESCOPED 2026-04-20 per SR-01 — US19 air-gapped Helm profile is net-new; needs own spec. Note US5/US18 references within it are also descoped)*: System MUST publish Helm charts and Terraform modules
  with a documented "air-gapped" profile that operates with zero
  outbound connections to Bifrost-owned infrastructure. In v1 the
  air-gapped profile explicitly supports: Organizations &
  Workspaces (US1), Granular RBAC (US2), SSO via OIDC only (US3 —
  SAML support for air-gapped is deferred), Audit Logs (US4),
  Admin API Keys (US5), BYOK / Customer-Managed Encryption (US18).
  All other user stories in this spec are NOT guaranteed to
  operate in the v1 air-gapped profile and MAY be declared
  air-gap-supported additively in later releases behind per-
  feature capability flags.
- **FR-038** *(DESCOPED 2026-04-20 per SR-01 — US21 Terraform provider is a new repo; needs own spec)*: System MUST publish a Terraform provider exposing
  workspaces, virtual keys, configs, guardrails, and admin API keys
  as managed resources.

**Non-Breaking & Compatibility:**

- **FR-039**: All new configuration fields MUST be optional; default
  behavior of a Bifrost deployment upgraded from v1.5.2 with no
  config changes MUST be byte-identical to its prior behavior.
- **FR-040**: All existing plugin hook signatures
  (`LLMPlugin.PreLLMHook`, `PostLLMHook`,
  `HTTPTransportPlugin.HTTPTransportPreHook`, etc.) MUST remain
  stable across this feature's release.
- **FR-041**: All existing `config.schema.json` required fields MUST
  remain as-is; any new required fields MUST be gated behind a
  feature-enable flag whose default is `false`.

**Deployment Modes & Licensing:**

- **FR-042**: System MUST expose a top-level `deployment.mode`
  configuration field with enum values
  `cloud | selfhosted | airgapped`. Each mode sets opinionated
  defaults for `multi_org_enabled`, phone-home telemetry, license
  enforcement, metering, and billing subsystems per the deployment-
  modes table in `docs/enterprise/deployment-modes.mdx`.
**FR-043..FR-046c and FR-047..FR-050b are all *(DESCOPED 2026-04-20 per SR-01 — Train E US24–US29 is net-new license/metering/billing infrastructure; each needs its own feature spec before implementation)*. Listed below for reference.**

- **FR-043** *(DESCOPED — US24)*: In `selfhosted` and `airgapped` modes, the license
  plugin MUST verify a signed license file (offline-verifiable,
  using a vendor public key embedded in the binary) on boot and on
  config reload. Verification MUST complete without any network
  call.
- **FR-044**: System MUST enforce license entitlements per-feature.
  Gated features consume the license plugin's
  `IsEntitled(feature_name)` check via context key and return a
  clear "feature-not-licensed" error (HTTP 402) when the check
  returns false.
- **FR-045**: On license expiry, the system MUST enter a 14-day
  grace period with UI banner + daily audit entries. After grace
  expiry, enterprise features MUST disable gracefully (revert to
  OSS-only behavior) while preserving existing data in read-only
  form. OSS core (chat/completions, embeddings, etc.) MUST continue
  unchanged throughout.
- **FR-046**: License threshold alerts MUST fire at 30, 7, and 1
  days before expiry via the configured email contact plus the
  existing alert destinations framework (US10).
- **FR-046a**: License-issuance tooling MUST be implemented as a
  CLI under `tools/license-authority/` that runs outside
  production and accepts customer name, entitlement list,
  expiration, and limits as inputs, producing a signed JWT file.
  The vendor's Ed25519 private key MUST NEVER be embedded in the
  Bifrost binary or committed to source control.
- **FR-046b**: Bifrost binaries MUST embed an ordered array of
  valid vendor public keys to support key rotation. License
  verification MUST try each embedded key in order and accept a
  license if any key validates the signature. Rotation cadence is
  12 months; the oldest embedded key is retired only after the
  longest-deployed air-gapped build has been superseded by a
  build containing the replacement key.
- **FR-046c**: When a license's declared limits are exceeded,
  system MUST block CREATION of new resources in the exceeded
  dimension (HTTP 402 with renewal link) while allowing all
  existing resources to continue functioning. `max_users` receives
  a 10% soft-grace window with escalating warnings at 100%, 105%,
  and 110% of the limit; `max_workspaces` and `max_virtual_keys`
  are hard-enforced at 100%.

**Cloud Commercial (Train E, `deployment.mode: cloud` only):**

- **FR-047**: In cloud mode, system MUST meter every request's
  attribution to an organization (requests, input_tokens,
  output_tokens, cost_usd_micros, cache_hits), producing daily
  rollups within 15 minutes of day close and monthly rollups
  within 1 hour of month close.
- **FR-048**: In cloud mode, system MUST integrate with Stripe for
  subscription management + Metered Billing API usage records.
  Failed payments MUST follow Stripe's dunning flow with a 14-day
  read-only grace period.
- **FR-049**: In cloud mode, system MUST expose a customer-facing
  billing portal showing current-cycle usage, projected cost,
  payment method, and past invoices (PDF download via Stripe).
- **FR-050**: In cloud mode, tier transitions (Dev → Prod)
  MUST be self-service and apply within 60 seconds of payment
  success. Enterprise-tier transitions route through a sales
  workflow (no self-service).
- **FR-050a**: In cloud mode, the tier-to-feature enforcement
  matrix MUST match the table published in
  `docs/enterprise/tiers-and-plans.mdx` (also summarized in
  plan.md §Deployment Modes → Tier Feature Matrix). Each gated
  feature's plugin consults `BifrostContext` for the current tier
  and returns HTTP 402 on a feature not available in the caller's
  tier.
- **FR-050b**: Cloud deployments in v1 operate in a single region
  (default US-EAST-1). The `organizations.data_residency_region`
  column is read-only in the UI and set to the deployment's region
  at organization-creation time. Multi-region support is schema-
  ready but not operationally activated in v1; EU-resident
  customers requiring v1 residency compliance must use the self-
  hosted deployment model.

### Key Entities

- **Organization**: Top-level tenant boundary. Contains one or more
  workspaces and one or more users. Has SSO settings, SCIM settings,
  org-wide guardrails, default retention policy, BYOK configuration,
  and billing/license state (if licensing is enabled).
- **Workspace**: Sub-organization isolation boundary. Contains virtual
  keys, prompts, configs, model catalog entries, provider
  configurations, workspace-scoped guardrails, budgets, rate limits,
  log views, and analytics scope. Has a region/residency attribute.
- **User**: Person authenticated into the system. Linked to an
  identity provider identity (SSO subject) and a set of role
  assignments at organization and workspace levels.
- **Role**: Either a built-in (Owner, Admin, Member, Manager) or
  custom-defined collection of resource scopes with verbs.
- **Virtual Key**: Per-workspace credential wrapping a provider
  credential selection, budget, rate limits, metadata tags, and
  attached guardrails.
- **Admin API Key**: Org-scoped programmatic credential with set of
  scopes, creator, expiration.
- **Service Account API Key**: Workspace-scoped non-user credential
  analogous to a virtual key but with admin-like scopes.
- **Prompt**: Named, folder-organized, versioned template with
  variables, optional partials, and multimodal content support.
  Carries `production_version` pointer.
- **Config**: Declarative routing/retry/cache/guardrail bundle,
  versioned per workspace.
- **Guardrail**: Policy applied to request or response content at org,
  workspace, or key scope. Has type (deterministic / LLM / partner /
  custom-webhook), configuration, execution mode, and actions.
- **Audit Entry**: Immutable record of an administrative action.
- **Alert Rule**: Threshold + window + destinations.
- **Log Export**: Destination + schedule + schema definition for
  outbound log delivery.
- **Retention Policy**: Per-workspace log and metric retention
  horizons.
- **KMS Configuration**: BYOK provider, key ARN/ID, data-key
  caching policy.
- **SSO Configuration**: Org-level identity provider binding.
- **SCIM Configuration**: Org-level provisioning binding.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An existing Bifrost v1.5.2 OSS deployment upgrading to
  the enterprise-capable build with zero config changes experiences
  no behavior change, measured by: identical responses to a
  100-request golden-set replay, identical metric series names and
  cardinality, and identical config validation outcome.
- **SC-002**: A new enterprise deployment can complete end-to-end
  onboarding (SSO configured → workspace created → user provisioned
  with scoped role → virtual key with budget + rate limits →
  central PII redaction enabled → test request executed →
  corresponding audit entries viewed) in under 30 minutes by an
  operator following the published quickstart.
- **SC-003**: 100% of the 23 user stories have a direct mapping to a
  documented Portkey feature URL (parity traceability matrix
  captured in the feature's docs).
- **SC-004**: Every enterprise feature emits all three telemetry
  signals (OpenTelemetry span, Prometheus metric, audit log entry)
  for its representative operations — validated by a completeness
  test in CI.
- **SC-005**: Enabling enterprise features adds no more than 1ms
  p50 and 3ms p99 overhead to the request hot path under the
  reference load profile (5k RPS, mixed provider mix).
- **SC-006**: No enterprise feature requires a modification under
  `core/**` — enforced by a CI check that diffs `core/` against the
  pre-feature baseline.
- **SC-007** *(applies when US7 PII redactor ships in its own spec)*: PII redaction achieves ≥98% recall on an industry-
  standard PII benchmark set (PII-NER eval corpus) with false-
  positive rate ≤2%.
- **SC-008** *(applies when US6 guardrails-central ships in its own spec)*: Guardrail execution at p95 adds ≤50ms when
  synchronous-parallel with up to 5 guardrails active.
- **SC-009** *(applies when US18 BYOK ships in its own spec)*: BYOK-enabled deployments can sustain full read/write
  throughput with data-key caching set to 15 minutes, with KMS call
  rate ≤ 1/minute per running instance.
- **SC-010** *(applies when US11 log export ships in its own spec)*: Log export reliably delivers ≥99.9% of log records
  to the configured destination under a 1,000 RPS sustained load
  with target unreachable for up to 10 minutes (via retry + dead-
  letter queue).
- **SC-011** *(needs its own perf harness — no task yet; open as a Phase-19 follow-up)*: Audit log query for a filtered range over 10M entries
  returns first page within 2 seconds p95.
- **SC-012** *(applies when US20 SCIM ships in its own spec)*: User-lifecycle events via SCIM propagate to Bifrost
  within 10 minutes p95 of IdP event.
- **SC-013**: Zero plaintext secrets in any logstore record, metric
  label, trace attribute, or external export — validated by a
  continuous scanner in CI and in production. *(T099 deferred — scanner harness pending.)*
- **SC-014** *(applies when US19 air-gapped Helm profile ships in its own spec)*: Published Helm airgapped profile installs and passes
  smoke tests on a cluster with egress restricted to cluster-
  internal only.
- **SC-015** *(applies when US21 Terraform provider ships in its own spec)*: Terraform provider achieves round-trip stability
  (apply → no-op plan) across 10 consecutive runs against a
  representative workspace configuration.
- **SC-016** *(applies when US24 license activation ships in its own spec)*: Offline license signature verification completes in
  <100ms on boot for a license file up to 4KB, measured on the
  reference container image. No network call is issued during
  verification (validated by eBPF socket monitor).
- **SC-017** *(applies when US25 license expiry ships in its own spec)*: License-expiry grace period + UI banner + daily audit
  entries + graceful-degradation-at-grace-end all function as
  designed, validated by a time-travel fixture that steps the
  system clock through pre-expiry / expiry / grace / post-grace
  phases and asserts state transitions.
- **SC-018** *(applies when US26–US27 metering + Stripe ship in their own spec)*: Cloud-tier billing accurately attributes ≥99.9% of
  requests to the correct organization with correct cost, validated
  by a reconciliation test that replays a metered fixture and
  compares Bifrost's rollup to an independently computed reference.
- **SC-019** *(applies when US29 tier management ships in its own spec)*: Customer self-service tier upgrade (Dev → Prod)
  completes in <5 minutes end-to-end (click Upgrade → Stripe
  checkout → confirmation email → tier features active).
- **SC-020** (revised 2026-04-20 per SR-01): Zero ContactUsView stubs
  remain for user stories classified **IN SCOPE** by SR-01 (US1, US2,
  US4, US8, US12, US13, US14, US30 — the expose-existing set).
  Descoped user stories (see SR-01 out-of-scope column) retain their
  ContactUsView fallback by design — the teaser is the honest
  representation of "not-yet-built, pending its own feature spec".
  Validated by a CI check that greps the built JS bundle for "This
  feature is a part of the Bifrost enterprise license" **at file
  paths corresponding to in-scope stubs only**; out-of-scope paths
  are whitelisted. A later feature spec that moves a story from
  out-of-scope → in-scope MUST also remove its whitelist entry.

## Assumptions

- **Deployment modes**: Bifrost ships one binary / image that
  operates in one of three modes per the `deployment.mode` config
  field: `cloud` (vendor SaaS — multi-org + metering + billing
  enabled, telemetry on), `selfhosted` (customer on-prem —
  single-org + license-key gated, no phone-home by default),
  `airgapped` (customer on-prem, no outbound, MVP feature subset
  per FR-037). The three modes share 100% of code paths and differ
  only in which plugins load and which defaults apply.
- **Licensing model (self-hosted)**: Enterprise features in
  `selfhosted` and `airgapped` modes are gated by a signed
  license file verified offline with an embedded vendor public
  key. License encodes customer identity, issued_at, expires_at,
  and per-feature entitlements. Hard expiry with 14-day grace
  period. OSS core features are never license-gated. Cloud mode
  does not use licenses (the vendor operates the deployment).
- **Multi-tenancy granularity**: v1 deployments host a single
  organization (matching the enterprise self-host pattern and
  Bifrost's existing customer/team data shape) but every tenant-
  scoped table is schema-ready for multi-org via an
  `organization_id` column defaulted to a synthetic single org.
  Activation of multi-org mode is gated by a future
  `multi_org_enabled` configuration flag and requires no schema
  migration. True multi-org SaaS is explicitly deferred to a later
  release.
- **Migration path**: First-boot migration into a default
  organization + default workspace is automatic and non-destructive.
  Admins can subsequently rename, reorganize, or split into
  additional workspaces.
- **Identity primacy**: SSO + SCIM is the primary identity pattern
  for enterprise. Local username/password remains available for
  break-glass only and is disabled by default in the enterprise
  profile.
- **Cloud billing provider**: Stripe is the chosen billing backend
  for Train E (Metered Billing API for usage, subscriptions +
  invoices + hosted payment updates + webhooks for dunning).
  Alternative providers (Chargebee, Orb) are out of scope for v1.
  Internal per-VK budget caps (US8) remain authoritative for
  self-hosted; cloud mode's Stripe integration sits alongside them
  for invoicing purposes.
- **Commercial tiers** (cloud, sketch): `Dev` (free, limited
  requests/month + 3-day log retention), `Prod` ($49/mo + $9 per
  100k overage requests, 30-day retention, guardrails available),
  `Enterprise` (custom contract, custom retention, dedicated
  support, SLA). Final tier thresholds confirmed with stakeholders
  before Train E implementation.
- **Mobile app support**: Not in scope for v1; the UI is web-only.
- **Feedback-score metric**: Assumes Bifrost exposes a feedback
  ingestion endpoint parallel to Portkey's (the existing logging
  plugin's feedback-score field in Bifrost is the source of truth).
- **Model catalog discovery**: Out of scope; the existing Bifrost
  `framework/modelcatalog` subsystem already supports dynamic model
  lists and is used unchanged.
- **Cold storage for retention**: When retention policies cause
  deletion, cold-storage migration (for compliance hold beyond
  active retention) is supported only via the Log Export feature
  (US11). A native built-in cold tier is not in scope for v1.
- **Regions/residency**: Per-workspace residency is metadata-only
  in v1 — the admin must ensure the workspace is hosted by the
  appropriate regional deployment. True cross-region routing of a
  single organization's workspaces is out of scope.
- **Prompt language**: Prompt templates use a Mustache-compatible
  variable syntax. Full Jinja/Handlebars compatibility is
  explicitly out of scope.
- **Baseline version**: Bifrost v1.5.2 is the starting point.
  Enterprise features ship across releases v1.6.0 → v2.0.0 mapped
  to Trains A–E (per plan.md §Phased Delivery Strategy). v1.6.0 →
  v1.9.0 are MINOR bumps preserving SemVer (Trains A–D). v2.0.0 is
  a MAJOR bump introducing the cloud commercial layer (Train E);
  v2.0.0 remains backward-compatible for self-hosted deployments
  (Train E plugins do not load in `selfhosted` / `airgapped` mode).
- **Terminology — license file vs license key**: Throughout this
  spec the terms "license file" and "license key" refer to the
  same artifact: a signed JWT file containing the customer's
  entitlements, expiry, and limits, verified offline against an
  embedded vendor public key. The vendor's signing keypair is a
  separate concept (see research R-26).
