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
- Q: How does the signing key rotate if compromised or nearing end-of-life? → A: Multi-key embed. Each Bifrost binary ships with an ordered array `[pubkey_v1, pubkey_v2, …]` of valid public keys. On license verification, each embedded key is tried in order; the first valid signature wins. Rotation cadence: 12 months. Rotation flow — (a) generate new keypair, (b) embed new public key alongside existing in the next binary release, (c) start issuing new licenses signed with new key, (d) after all customers are known to be on a binary version containing the new public key, drop the old key in the subsequent release. Air-gapped customers control their update cadence; the vendor MUST support the old key until the longest-deployed air-gapped binary has been superseded.
- Q: What happens when a license's declared limits (`max_workspaces`, `max_users`, `max_virtual_keys`) are exceeded at runtime? → A: Block-new, allow-existing. Existing resources continue to function unchanged; attempts to CREATE new resources beyond the limit return HTTP 402 with a clear renewal-contact link. `max_users` receives a 10% soft-grace (e.g., a 100-seat license allows 110 active users) with escalating UI warnings at 100%, 105%, 110%. Hard cutoff for `max_workspaces` and `max_virtual_keys` because those affect persistence scale, not just identity count. This pattern prevents production breakage on an overshoot and forces a renewal conversation rather than an outage.
- Q: Cloud mode — single-region or multi-region in v1? → A: Schema-ready for multi-region, operationally single-region (US-EAST-1) in v1. The `organizations.data_residency_region` column already exists from data-model §1; it defaults to `us-east-1` in v1 and is read-only in the UI. Multi-region activation (adding EU-WEST-1 for GDPR) is a v2 concern that requires no schema migration, only the deployment of a second regional instance and a region-routing front door. Customers requiring EU residency in v1 should use the self-hosted deployment model in their own EU infrastructure.

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

### User Story 5 — Organization-Level Admin API Keys (Priority: P1)

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

### User Story 8 — Granular Budgets & Rate Limits per Virtual Key (Priority: P2)

As a **FinOps Manager**, I can set per-virtual-key budget caps (monthly
dollar or token limit, or custom billing period) and rate limits (per-
minute / hourly / daily request-count and token-count) so that a single
team or customer cannot exceed its allocation, and threshold alerts
automatically notify stakeholders before overage.

**Why this priority**: Unbounded AI spend is the single largest CFO
concern in AI deployments. Bifrost already has budgets, but per-key
granularity, multiple simultaneous rate-limit windows, and
threshold-based alerting (50%, 75%, 90%) are the Portkey parity gap.

**Independent Test**: Set a virtual key with `$100/month` cap and
`50 req/min` rate limit; send requests at 60/min and observe after
request #50 in a minute the 429 response; separately generate traffic
summing to $95 worth of cost and verify a threshold alert fires;
continue to $100+ and verify requests reject with budget-exceeded.

**Parity mapping**: https://portkey.ai/docs/product/ai-gateway (Virtual Keys)

**Acceptance Scenarios**:

1. **Given** a virtual key with a monthly budget of $100, **When**
   cumulative spend reaches configured alert thresholds (50%, 75%, 90%),
   **Then** a webhook and/or Slack notification fires to the configured
   recipients with current spend, threshold, and time remaining in
   period.
2. **Given** a virtual key with a request rate limit of 50 req/min,
   **When** the 51st request in a 60-second window arrives, **Then**
   Bifrost returns HTTP 429 with a `Retry-After` header and a clear body
   message naming which limit was hit.
3. **Given** a virtual key with simultaneous per-minute, per-hour, and
   per-day token limits, **When** any one limit is exceeded, **Then**
   subsequent requests are rejected with the most specific applicable
   limit named in the error.
4. **Given** a new billing period boundary, **When** the period rolls
   over, **Then** spend counters reset automatically and the key is
   usable again (unless other limits are hit).

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
- **FR-003**: System MUST enforce three default org-level roles
  (Owner, Admin, Member) and two default workspace-level roles
  (Manager, Member) plus an unbounded set of customer-defined custom
  roles.
- **FR-004**: System MUST support per-resource scopes (metrics,
  completions, prompts, configs, guardrails, integrations, providers,
  models, team-management) with read / write / delete verbs each.
- **FR-005**: System MUST support SSO via SAML 2.0 and OIDC with at
  minimum Okta, Azure AD / Entra ID, Google Workspace, and generic
  OIDC compliance.
- **FR-006**: System MUST auto-provision users on first SSO login and
  auto-accept pending invitations matching the authenticated email.
- **FR-007**: System MUST support SCIM 2.0 for user lifecycle
  management from external IdPs.
- **FR-008**: System MUST support org-level Admin API Keys with per-
  key scopes, creator tracking, expiration, and one-time display at
  creation.
- **FR-009**: System MUST support service-account API Keys scoped to
  a single workspace with their own budgets, rate limits, and no UI
  login capability.

**Audit & Governance:**

- **FR-010**: System MUST record an audit entry for every
  administrative or governance action including actor ID, actor IP,
  action type, resource type, resource ID, before/after values,
  outcome (allowed/denied), request ID, and ISO 8601 timestamp.
- **FR-011**: System MUST expose audit log query with filters
  (actor, action, resource, time range, outcome) and export to CSV
  and JSON, complete (no truncation) up to at least 10M entries.
- **FR-012**: System MUST persist audit entries for a duration at
  least as long as the longest configured retention policy, and in
  no case less than 1 year when audit logging is enabled.

**Guardrails:**

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
- **FR-019**: System MUST apply PII redaction to request and response
  payloads before persistence to logs, with configurable mode
  (redact-in-logs-only vs redact-before-provider), a configurable PII
  pattern set, and a fail-closed mode.

**Budgets & Rate Limits:**

- **FR-020**: System MUST enforce per-virtual-key budget caps in
  dollars and/or tokens with configurable billing period (monthly,
  weekly, custom).
- **FR-021**: System MUST enforce per-virtual-key rate limits on
  request count and token count with per-minute, per-hour, and
  per-day windows, any of which may be set independently.
- **FR-022**: System MUST fire threshold alerts (configurable, default
  50%, 75%, 90%) to configured destinations before budget exhaustion.
- **FR-023**: System MUST return HTTP 429 with a machine-readable
  body and `Retry-After` header when rate limit is exceeded, and
  HTTP 402 (or 429 per preference) when budget is exceeded.

**Alerts & Observability:**

- **FR-024**: System MUST support threshold-based alert rules on
  error rate, p50/p95/p99 latency, cost-per-hour, and feedback score,
  with multi-destination delivery (webhook, Slack, email).
- **FR-025**: System MUST support log export to at minimum AWS S3,
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

- **FR-029**: System MUST support prompt templates with variable
  substitution (Mustache-compatible), partials (reusable fragments),
  folder organization with per-folder access scopes, full version
  history, and a designated "production" version per prompt. In
  v1, prompts, versions, partials, and folder metadata are
  persisted in the existing configstore (reusing its multi-tenancy,
  encryption, and hot-reload infrastructure). A dedicated
  `framework/promptstore/` subsystem is explicitly deferred to v2
  and is out of scope for this feature.
- **FR-030**: System MUST expose `/prompts/<id>/render` and
  `/prompts/<id>/completions` endpoints that accept variables and
  return the rendered or fully-executed completion.
- **FR-031**: System MUST support multimodal prompts combining text
  and images.
- **FR-032**: System MUST support an interactive Playground for
  running a single prompt against N selected models in parallel,
  displaying streaming per-model outputs with latency, token count,
  and cost.
- **FR-033**: System MUST support declarative Config objects
  (JSON) describing primary + fallback providers, retry policy,
  cache policy, attached guardrails, and metadata, addressable via
  `x-config-id` header with optional version pin.
- **FR-034**: System MUST support canary routing as a primitive
  defined WITHIN the declarative Config object (not within the
  governance plugin's routing-chain). The Config canary primitive
  declaratively splits a configured percentage of traffic to a
  specified target with comparison reporting. Governance plugin
  routing-chain logic remains scoped exclusively to budget and
  virtual-key enforcement and MUST NOT be extended to implement
  canary behavior.

**Security & Deployment:**

- **FR-035**: System MUST support BYOK for at-rest encryption with
  AWS KMS, Azure Key Vault, and GCP KMS. By default, BYOK encrypts
  configstore-managed secrets (provider API keys, webhook secrets,
  admin and service-account API keys, prompt templates, OAuth
  client secrets). The customer-managed key wraps a per-record
  data encryption key with in-memory caching.
- **FR-035a**: BYOK encryption of logstore request/response
  payloads MUST be supported as an opt-in per-workspace setting
  with a configurable data-key cache TTL (default 15 minutes per
  SC-009). When the setting is disabled, logstore writes use the
  configstore encryption key as before.
- **FR-036**: System MUST never log or emit provider API keys,
  webhook secrets, admin API keys, or OAuth client secrets in
  plaintext to any logstore, metric label, trace attribute, or
  external export.
- **FR-037**: System MUST publish Helm charts and Terraform modules
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
- **FR-038**: System MUST publish a Terraform provider exposing
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
- **FR-043**: In `selfhosted` and `airgapped` modes, the license
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
- **SC-007**: PII redaction achieves ≥98% recall on an industry-
  standard PII benchmark set (PII-NER eval corpus) with false-
  positive rate ≤2%.
- **SC-008**: Guardrail execution at p95 adds ≤50ms when
  synchronous-parallel with up to 5 guardrails active.
- **SC-009**: BYOK-enabled deployments can sustain full read/write
  throughput with data-key caching set to 15 minutes, with KMS call
  rate ≤ 1/minute per running instance.
- **SC-010**: Log export reliably delivers ≥99.9% of log records
  to the configured destination under a 1,000 RPS sustained load
  with target unreachable for up to 10 minutes (via retry + dead-
  letter queue).
- **SC-011**: Audit log query for a filtered range over 10M entries
  returns first page within 2 seconds p95.
- **SC-012**: User-lifecycle events via SCIM propagate to Bifrost
  within 10 minutes p95 of IdP event.
- **SC-013**: Zero plaintext secrets in any logstore record, metric
  label, trace attribute, or external export — validated by a
  continuous scanner in CI and in production.
- **SC-014**: Published Helm airgapped profile installs and passes
  smoke tests on a cluster with egress restricted to cluster-
  internal only.
- **SC-015**: Terraform provider achieves round-trip stability
  (apply → no-op plan) across 10 consecutive runs against a
  representative workspace configuration.
- **SC-016**: Offline license signature verification completes in
  <100ms on boot for a license file up to 4KB, measured on the
  reference container image. No network call is issued during
  verification (validated by eBPF socket monitor).
- **SC-017**: License-expiry grace period + UI banner + daily audit
  entries + graceful-degradation-at-grace-end all function as
  designed, validated by a time-travel fixture that steps the
  system clock through pre-expiry / expiry / grace / post-grace
  phases and asserts state transitions.
- **SC-018**: Cloud-tier billing accurately attributes ≥99.9% of
  requests to the correct organization with correct cost, validated
  by a reconciliation test that replays a metered fixture and
  compares Bifrost's rollup to an independently computed reference.
- **SC-019**: Customer self-service tier upgrade (Dev → Prod)
  completes in <5 minutes end-to-end (click Upgrade → Stripe
  checkout → confirmation email → tier features active).

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
