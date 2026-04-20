<!--
Sync Impact Report
==================
Version change: 1.1.0 → 1.2.0
Bump rationale: MINOR — added 4 new principles (XII-XV) for code quality,
                testing standards, UX consistency, and performance.
                Additive only; no existing principle modified or removed.

Modified principles: (none)

Added principles:
  XII.  Code Quality Standards (NEW)
  XIII. Testing Discipline (NEW)
  XIV.  UX Consistency (NEW)
  XV.   Performance Budget (NEW)

Added sections: (none)
Removed sections: (none)

Templates requiring updates:
- .specify/templates/plan-template.md  ✅ updated — Constitution Check
  table now has XII-XV rows.

------------------------------------------------------------------------
Previous history preserved below:
------------------------------------------------------------------------

Version change: 1.0.0 → 1.1.0
Bump rationale: MINOR — added Principle XI (Upstream-Mergeability).

Version change: (uninitialized template) → 1.0.0
Bump rationale: Initial ratification (Principles I-X).
-->

# Bifrost Constitution

Bifrost is a production AI gateway consumed as (a) a Go library (`core/`,
`framework/`, `plugins/*`) by external applications, and (b) a self-contained
HTTP server and UI built on top of those libraries (`transports/`, `ui/`,
`cli/`, `helm-charts/`). This constitution governs how enterprise-grade
features — targeting parity with Portkey's enterprise tier — are added to
Bifrost without destabilizing the library contract that downstream users
depend on.

## Core Principles

### I. Core Immutability (NON-NEGOTIABLE)

The `core/` Go module MUST NOT be modified to deliver enterprise features.
This includes `core/schemas/` (plugin, provider, context key definitions),
`core/bifrost.go`, `core/mcp/`, and all provider implementations under
`core/providers/`.

Enterprise functionality is delivered exclusively by adding to:
`plugins/<new-plugin>/`, `framework/<new-subsystem>/`, new handlers under
`transports/bifrost-http/handlers/`, middleware under
`transports/bifrost-http/lib/middleware.go`, new UI pages under `ui/`, and
Helm / Terraform assets under `helm-charts/` and `terraform/`.

**Rationale**: `core` is imported as a library by external consumers. Any
change to `core/schemas/Plugin`, `Provider`, or `ProviderConfig` propagates
across 20+ provider implementations and every downstream user. A breaking
change in `core` breaks every consumer simultaneously. Enterprise parity
must be purely additive at the boundary layers.

**Escape hatch**: If a required capability is provably impossible without a
core change, the PR MUST include (a) a written analysis of why extension
points (Principles III, IV) are insufficient, (b) a migration plan for
downstream consumers, and (c) approval recorded in `specs/<feature>/
complexity.md`. This is expected to be exceptional, not routine.

### II. Non-Breaking by Default

Every change to a public surface MUST preserve backward compatibility within
a major version:

- New fields in `transports/config.schema.json` MUST be optional with a safe
  default. Required fields are a MAJOR-version change.
- Existing `LLMPlugin` / `HTTPTransportPlugin` / `MCPPlugin` /
  `ObservabilityPlugin` hook signatures MUST remain stable. New capability
  goes into new hooks or new interfaces, never mutated signatures.
- New plugins live alongside existing ones under `plugins/<name>/` with
  their own `go.mod`; they do not replace existing plugins in-place.
- Deprecations require two full minor-version cycles: mark deprecated in
  version N, emit runtime warning in N+1, remove no earlier than N+2 (and
  only on a MAJOR boundary).
- Default behavior with a stock `config.json` from version N MUST continue
  to work unchanged on version N+1.

**Rationale**: Bifrost advertises itself as a drop-in SDK-compatible gateway
(OpenAI, Anthropic, Bedrock, GenAI, LangChain, LiteLLM). Breaking a plugin
contract or a config schema shape strands every production deployment.

### III. Plugin-First Architecture

Enterprise features MUST be implemented using the existing extension model:

- Request-path logic → implement `LLMPlugin`, `HTTPTransportPlugin`, or
  `MCPPlugin` under `plugins/<name>/` following the pattern in
  `plugins/governance/main.go` and `plugins/logging/main.go`.
- Async emission (traces, metrics, external export) → implement
  `ObservabilityPlugin`.
- Cross-cutting HTTP concerns (auth, CORS, request-ID, tenant resolution)
  → add middleware to `transports/bifrost-http/lib/middleware.go` and
  chain per-route.
- Persistence backends (configstore, logstore, vectorstore) → implement
  the framework interface under `framework/<name>/` rather than embedding
  storage in a plugin.

Plugins MUST declare `PluginPlacement` (`pre_builtin` | `builtin` |
`post_builtin`) and `Order` explicitly in their default registration to
make hook ordering auditable.

**Rationale**: The plugin system is the documented extension point with
symmetric pre/post hooks, short-circuit support, and hot-reload via atomic
pointer swap. Bypassing it (e.g., forking `core/bifrost.go`) loses
observability hooks, hot-reload, and placement ordering guarantees.

### IV. Config-Driven Gating

Enterprise-only features are gated by configuration, not by build flags or
separate binaries:

- Feature toggles live in `transports/config.schema.json` as plugin-level
  or global booleans (e.g., `governance.config.is_enterprise`,
  `workspaces.enabled`) with a default that preserves OSS behavior.
- Schema conditionals use JSON Schema `allOf` / `if` / `then` blocks to
  require additional fields only when a feature is enabled.
- No Go build tags (`//go:build enterprise`) for feature gating. No
  compiled-in license checks that diverge the binary.
- Runtime license validation (if introduced) runs as a plugin
  (`plugins/license/`) that sets context keys consumed by gated features;
  it MUST NOT live in `core`.

**Rationale**: A single binary with config-driven gating avoids the
"enterprise fork rot" problem where OSS and enterprise codebases drift. It
also keeps downstream library users from accidentally depending on
enterprise-only code paths.

### V. Multi-Tenancy First

Every new feature — configstore tables, logstore schemas, API handlers, UI
views, plugin state — MUST carry workspace/organization scoping from the
first commit:

- New framework persistence tables include `organization_id` and
  `workspace_id` columns (or equivalent document fields), both indexed.
- Context keys carry tenant identifiers through the plugin chain; plugins
  read them from `BifrostContext` rather than from global state.
- API handlers MUST filter by the caller's resolved tenant before returning
  data; cross-tenant reads require an explicit admin scope check.
- UI components accept a workspace context prop and never display data
  across tenant boundaries.

Backfilling multi-tenancy after a feature ships is explicitly disallowed:
it has historically been the dominant source of data-leak regressions in
gateway products.

**Rationale**: Portkey's enterprise differentiation is built on workspace
isolation. Retrofitting tenancy onto single-tenant tables requires a
destructive migration for every existing Bifrost deployment.

### VI. Observability Mandatory

Every enterprise feature MUST emit all three signals:

1. **OpenTelemetry spans** via the `plugins/otel` infrastructure — one span
   per request-scoped operation, attributes include `tenant.id`,
   `workspace.id`, `virtual_key.id`.
2. **Prometheus metrics** via the `plugins/telemetry` registry — at
   minimum a counter and a histogram (latency) labeled with tenant +
   feature name. Metrics register through the Push Gateway path when
   clustering is enabled.
3. **Audit log entries** via `plugins/logging` for every administrative or
   governance action (create/update/delete of keys, budgets, workspaces,
   roles, guardrails, configs). Audit records MUST include actor identity,
   action, resource, before/after state, and request-id.

Features that cannot emit one of the three MUST justify the gap in the
feature's `plan.md` under a "Deferred Observability" section.

**Rationale**: Enterprise buyers evaluate gateways on observability
completeness. A feature that does not appear in traces, metrics, and audit
logs is invisible to SecOps and FinOps buyers.

### VII. Security by Default

- Provider API keys, OAuth client secrets, SMTP passwords, webhook secrets,
  and customer-supplied credentials MUST be written through the
  configstore's `encryption_key`-backed encryption. Plaintext at rest is
  prohibited.
- All external integrations (SSO IdPs, webhook targets, log exports,
  vector DBs, partner guardrail APIs) require TLS 1.2+; HTTP is rejected
  at config-validation time for production profiles.
- Secrets MUST NOT appear in logstore records, OTEL attributes, or
  Prometheus labels. The logstore write path runs a PII/secret redaction
  hook before persist; new fields containing sensitive data register with
  the redactor at plugin init.
- Admin API keys, workspace API keys, and virtual keys MUST be stored
  hashed (argon2id or equivalent) with only a prefix displayed in the UI
  after creation.
- Time-window-limited, scoped signing is preferred over long-lived bearer
  tokens for service-to-service calls (e.g., webhook signatures).

**Rationale**: SOC 2, ISO 27001, and HIPAA attestations — which are the
explicit target of the enterprise parity effort — hinge on demonstrable
encryption at rest, transport security, and secret-handling discipline.

### VIII. Test Coverage Required

- Plugin unit tests live under `plugins/<name>/*_test.go` and run in CI.
- Integration tests hit real dependencies: PostgreSQL for configstore and
  logstore, a real vectorstore (Weaviate/Qdrant/Redis) for semantic cache
  paths, and real provider APIs under `make test-core` where a key is
  available. Mocking these layers is prohibited for integration tests
  because mocked behavior has historically diverged from production
  (see AGENTS.md "Gotchas & Conventions").
- UI additions include Playwright E2E tests under
  `ui/tests/e2e/` using the `data-testid="<entity>-<element>-<qualifier>"`
  naming convention.
- Every new HTTP handler registered in `transports/bifrost-http/handlers/`
  has at least one happy-path integration test and one authorization-
  failure test (wrong tenant / missing scope).

**Rationale**: Bifrost's correctness claims depend on behavior under real
provider quirks, real SQL constraints, and real vectorstore semantics. CI
time spent on integration tests has repeatedly caught regressions that
mocks would have hidden.

### IX. Documentation & Schema Sync

Every feature-delivering PR MUST, in the same PR:

1. Update `transports/config.schema.json` with new fields, descriptions,
   and conditionals.
2. Add or update Mintlify MDX docs under `docs/enterprise/<feature>.mdx`
   (or the appropriate subtree) including configuration reference and at
   least one runnable example.
3. Append a `changelog.md` entry in every module touched
   (`core/`, `framework/`, `plugins/<name>/`, `transports/`) respecting
   the core→framework→plugins→transport version hierarchy.
4. Add UI-facing strings to the translations manifest if applicable.
5. Link to the spec at `specs/<###-feature>/spec.md` from the docs page.

A PR without these artifacts MUST NOT merge, regardless of code quality.

**Rationale**: `config.schema.json` is the source of truth enforced by
UI validation, CI schema diff, and downstream tooling (Terraform provider,
Helm values validation). Out-of-sync docs are the single largest reported
enterprise onboarding friction per `docs/enterprise/` issue history.

### X. Dependency Hierarchy Respected

Import direction is strictly `core` → `framework` → `plugins/*` →
`transports` → `ui`. Reverse imports are prohibited:

- `core/` does not import `framework/`, `plugins/`, or `transports/`.
- `framework/` does not import `plugins/` or `transports/`.
- `plugins/<X>/` does not import `plugins/<Y>/`; cross-plugin coordination
  goes through context keys or framework services.
- Plugin modules (`plugins/*/go.mod`) remain independently versioned so a
  plugin can ship a patch without a core release.

**Rationale**: The workspace (`go.work`) silently resolves cycles at
development time but they surface as import errors at release tagging.
Keeping plugins independently versioned also enables targeted security
patches without re-releasing the whole gateway.

### XI. Upstream-Mergeability (NON-NEGOTIABLE for enterprise fork)

Bifrost is a living upstream (`maximhq/bifrost`). Our enterprise fork
MUST remain mergeable with upstream on a weekly cadence without
"rebase hell." Every enterprise change MUST follow these rules:

1. **Additive-by-sibling-file**: When extending an upstream plugin or
   transport file (e.g., `plugins/governance/main.go`,
   `transports/bifrost-http/lib/middleware.go`), add a NEW file
   alongside (e.g., `plugins/governance/budgets.go`,
   `transports/bifrost-http/lib/middleware_enterprise.go`) instead of
   inlining changes. Upstream files in our fork MUST be touched only
   for (a) a single-line hook-registration call, or (b) a trivially-
   merged append. No reordering, no reformatting, no interleaved
   additions.

2. **Migration namespace separation**: Bifrost uses GORM struct
   tables in `framework/<store>/tables/` (configstore) or
   `framework/<store>/tables.go` (logstore), with migrations
   registered as Go code via the `framework/migrator` package
   (gormigrate-based; each migration has a string `ID`, a
   `Migrate` function, and a `Rollback` function). Enterprise
   conventions:
   - **New enterprise tables** live in
     `framework/<store>/tables-enterprise/*.go` (configstore) or
     `framework/<store>/tables_enterprise.go` (logstore) —
     sibling location, never edits upstream's `tables/`.
   - **Enterprise migrations** are registered in a sibling
     `framework/<store>/migrations_enterprise.go` file. Migration
     IDs use the prefix `E###_<descriptive_name>` (e.g.,
     `E001_orgs_workspaces`). Upstream IDs are descriptive
     strings without that prefix; the namespaces never collide
     in the migration tracking table.
   - **Adding tenancy / enterprise columns to an existing
     upstream table** MUST NOT modify the upstream GORM struct.
     Instead, create a 1:1 sidecar table (e.g.,
     `virtual_key_tenancy(virtual_key_id PK FK, organization_id,
     workspace_id)`) in `tables-enterprise/`. Reads JOIN the
     sidecar; writes update both. If the columns are later
     upstreamed (Principle XI rule 7), the sidecar can be
     removed in a v1.7+ release with a one-time migration that
     copies values from sidecar into the main table.
   - The enterprise migration runner is invoked from the
     enterprise plugin's `Init()` via a single
     `RegisterEnterpriseMigrations(db)` call — no upstream file
     edit is required to wire it.

3. **Schema overlay, not patch**:
   `transports/config.schema.json` is hot territory. Enterprise
   fields live in `transports/config.schema.enterprise.json` and are
   composed at boot time via JSON Schema `$ref`. The upstream file
   receives at most a single
   `allOf: [{ $ref: "config.schema.enterprise.json" }]` entry.

4. **Middleware / router hook points**: Upstream middleware chains
   and UI route registrations MUST expose a single extension hook
   that the enterprise fork fills. Upstream's registration code
   stays unchanged; enterprise work happens in
   `*_enterprise.go` / `enterpriseRoutes.ts` files upstream never
   touches.

5. **Weekly merge discipline**: A maintainer runs
   `git fetch upstream && git merge upstream/main --no-ff` at least
   weekly, resolves conflicts, runs the full integration + golden-
   set suite, and commits. A missed week is maintenance debt; two
   missed weeks is an incident.

6. **Drift watcher**: CI computes the line-diff size against
   `upstream/main` for a watch-list of shared files (six extended
   plugins, `middleware.go`, `config.schema.json`, `AGENTS.md`,
   `CLAUDE.md`) and fails any PR that grows the diff beyond the
   agreed ceiling without a `drift:` commit prefix.

7. **Upstreaming bias**: Primitives that are not enterprise-gated
   (e.g., the multi-tenancy framework, audit sink, crypto primitive)
   SHOULD be offered upstream via PR to `maximhq/bifrost`. Accepted
   PRs shrink our long-term delta; rejected ones stay in our fork
   documented as "rejected upstream: <reason>".

**Rationale**: A fork that ships enterprise features becomes
worthless the moment merging upstream takes a full week per cycle.
Principle I keeps `core/**` clean; Principle XI keeps the *rest* of
the tree mergeable by forcing additive patterns where upstream
evolves.

**Escape hatch**: If a genuine upstream-file edit is unavoidable
(e.g., a bug fix that blocks enterprise work and isn't yet upstream),
the PR MUST (a) cite the upstream issue/PR number, (b) add the file
to the "upstream-carried patches" list in `UPSTREAM-SYNC.md`, (c)
include a plan to drop the patch once upstream merges or rejects.

**Operator runbook**: See `UPSTREAM-SYNC.md` at repo root for the
weekly merge flow, conflict-resolution playbook, and CI drift-
watcher configuration.

### XII. Code Quality Standards

All Go code MUST pass `gofmt`, `go vet`, and `golangci-lint` with the
project's `.golangci.yml` configuration before merge. All TypeScript
code MUST pass `tsc --noEmit` and the project's ESLint configuration.

- Functions exceeding 80 lines MUST be split or justified in PR review.
- Exported functions and types MUST have GoDoc comments. Unexported
  helpers need comments only when the intent is non-obvious.
- Magic numbers and hardcoded strings MUST be extracted to named
  constants.
- Error messages MUST include enough context to diagnose without
  reading source (resource type, ID, operation attempted).
- No `//nolint` directives without an adjacent comment explaining why
  the lint rule does not apply.
- UI components MUST use the project's existing design system
  (`@/components/ui/*`) — no raw HTML elements for interactive
  controls.

**Rationale**: Consistent code quality reduces review friction, makes
the codebase navigable by new contributors, and prevents style debates
from consuming review cycles.

### XIII. Testing Discipline

Beyond Principle VIII's coverage requirements, testing MUST follow
these standards:

- **Unit tests**: Every exported function with branching logic has at
  least one happy-path and one error-path test.
- **Integration tests**: Run against real PostgreSQL (configstore +
  logstore) via `make test-enterprise`. Mocks are prohibited for
  database interactions.
- **E2E tests**: Every new UI page or dialog has a Playwright test
  under `ui/tests/e2e/` using `data-testid` attributes. Tests MUST
  cover the golden path and at least one error state.
- **Test isolation**: Each test MUST set up and tear down its own
  data. Tests MUST NOT depend on execution order or shared state
  from other tests.
- **Flaky test policy**: A test that fails intermittently is
  immediately quarantined (skipped with `t.Skip("flaky: <issue>")`),
  investigated within 48 hours, and either fixed or deleted.

**Rationale**: Untested code is unshippable code. Flaky tests erode
CI trust and cause engineers to ignore real failures.

### XIV. UX Consistency

Every enterprise UI page MUST follow existing Bifrost design patterns:

- **Layout**: Use the existing workspace layout
  (`ui/app/workspace/*/layout.tsx` + `page.tsx` pattern) with
  TanStack Router file-based routing.
- **Components**: Use the project's Radix UI + Tailwind design system
  (`@/components/ui/*`). No new UI libraries without explicit
  justification.
- **Data fetching**: Use RTK Query (`ui/lib/store/apis/*.ts`) with
  proper cache tag invalidation. No raw `fetch()` calls from
  components.
- **Enterprise stubs**: Replace `ContactUsView` fallbacks by
  implementing real components at
  `ui/app/enterprise/components/<feature>/<view>.tsx`. The file MUST
  export a default function component.
- **data-testid convention**: All interactive elements carry
  `data-testid="<entity>-<element>-<qualifier>"` for Playwright.
- **Loading/error/empty states**: Every data-fetching view MUST
  handle loading (skeleton or spinner), error (message with retry),
  and empty (helpful prompt to create first item) states.
- **Toast notifications**: Use `sonner` for success/error feedback.
  No `alert()` or `console.log()` for user-facing messages.
- **RBAC gating**: Pages MUST check `useRbac(resource, operation)`
  and render `NoPermissionView` when access is denied.

**Rationale**: Inconsistent UI erodes user trust. Enterprise buyers
evaluate polish during procurement demos — a single broken or
unstyled page can stall a deal.

### XV. Performance Budget

Every feature operates within a strict performance budget:

- **Hot-path overhead**: Enterprise plugins MUST NOT add more than
  1ms p50 / 3ms p99 to request latency at 5k RPS (SC-005). Measured
  by `make perf` benchmark suite.
- **Cold-start**: Server boot time MUST NOT exceed 15 seconds with
  all enterprise plugins enabled (including migration checks).
- **UI page load**: Enterprise pages MUST render first meaningful
  paint within 2 seconds on a standard connection. Use code-splitting
  (`autoCodeSplitting: true` in TanStack Router config).
- **Database queries**: No N+1 queries. List endpoints MUST use
  single queries with JOINs or preloads. Audit log queries over 10M
  rows MUST return first page within 2 seconds (SC-011).
- **Memory**: Plugins MUST NOT hold unbounded in-memory state. Caches
  MUST have TTL or size bounds. Streaming responses MUST NOT buffer
  the full body.
- **Async by default**: Non-critical operations (audit writes,
  metric emissions, export flushes) MUST run asynchronously via
  channels or background workers. The request path MUST NOT block
  on observability.

**Rationale**: Performance regressions are silent — they accumulate
until a customer reports latency spikes. Explicit budgets make them
visible before merge.

## Additional Constraints

**Performance**: New plugins MUST NOT add more than 1ms p50 overhead to the
hot request path measured at 5k RPS on the reference load profile. Plugins
that need heavier work run async via `ObservabilityPlugin` or background
workers driven off a channel.

**Hot-reload discipline**: Any configurable list a plugin reads at request
time MUST be held behind an `atomic.Pointer` (see governance plugin's
`requiredHeaders` handling); in-place mutation of config slices is
prohibited because it races with concurrent readers.

**Pooling**: Pooled objects (request, response, context) MUST have all
fields reset before return to pool. New fields on pooled types register
with the reset function in the same commit.

**Compliance posture**: Features intended to support SOC 2 / ISO 27001 /
HIPAA / GDPR claims MUST name the specific control they address in
`specs/<feature>/spec.md` under a "Compliance Mapping" section.

## Development Workflow & Quality Gates

**Spec-driven development**: Every enterprise feature follows the Spec Kit
flow — `/speckit-constitution` (this document) → `/speckit-specify` →
`/speckit-clarify` → `/speckit-plan` → `/speckit-tasks` → `/speckit-analyze`
→ `/speckit-implement`. Artifacts live under `specs/<###-feature>/`.

**Feature branching**: Work happens on `feature/<###-name>` branches cut
by `/speckit-git-feature`. Merges to `main` require (a) spec + plan +
tasks committed, (b) all principles in this constitution satisfied, (c)
green CI including integration tests, (d) a reviewer outside the author's
immediate team.

**Constitution Check gate**: The `/speckit-plan` output includes an
explicit "Constitution Check" section listing each of Principles I–X and
asserting compliance. Violations require a "Complexity Tracking" entry in
`plan.md` with justification and the simpler alternative that was
rejected.

**Changelog discipline**: Module version bumps respect the
core→framework→plugins→transport hierarchy. A core bump cascades. A
plugin bump is self-contained.

**Code review**: Reviewers verify principle compliance explicitly in PR
description; a boilerplate principle-check block lives in
`.github/pull_request_template.md` (to be added alongside the first
enterprise feature).

## Governance

**Supremacy**: This constitution supersedes individual team preferences,
expedience arguments, and "temporary" shortcuts. When a conflict exists
between this document and another practice document, this document wins
unless the other is a superseding constitutional amendment.

**Amendment procedure**:

1. Draft amendment as a PR touching `.specify/memory/constitution.md`.
2. Commit message MUST begin with `constitution:` (enforced by
   `.pre-commit-config.yaml` hook — to be added).
3. Bump `**Version**` field per semantic rules:
   - **MAJOR**: Principle removal, principle redefinition that inverts
     intent, or governance rule change that removes a check.
   - **MINOR**: Principle addition or materially expanded guidance.
   - **PATCH**: Wording clarification, typo, reordering that does not
     change semantics.
4. Update `**Last Amended**` to the PR merge date (ISO YYYY-MM-DD).
5. Sync Impact Report at the top of this file is updated in the same PR.
6. Dependent templates (`.specify/templates/*.md`) are updated in the
   same PR if the amendment affects them.
7. Approval: two maintainers, at least one from core/framework and one
   from transports/plugins.

**Compliance review**: On every `/speckit-analyze` invocation, the
analysis cross-checks `spec.md`, `plan.md`, and `tasks.md` against each
of Principles I–XV and flags gaps. Unresolved flags block
`/speckit-implement`.

**Runtime guidance**: For day-to-day development conventions
(fasthttp vs net/http, sonic vs encoding/json, pool reset discipline,
fallback pipeline semantics, and the full "Gotchas & Conventions"
catalogue), engineers consult [AGENTS.md](../../AGENTS.md). This
constitution governs *what* must be true; AGENTS.md governs *how* to
write code such that it stays true.

**Version**: 1.2.0 | **Ratified**: 2026-04-19 | **Last Amended**: 2026-04-19
