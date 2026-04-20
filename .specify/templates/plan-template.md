# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: [e.g., Python 3.11, Swift 5.9, Rust 1.75 or NEEDS CLARIFICATION]  
**Primary Dependencies**: [e.g., FastAPI, UIKit, LLVM or NEEDS CLARIFICATION]  
**Storage**: [if applicable, e.g., PostgreSQL, CoreData, files or N/A]  
**Testing**: [e.g., pytest, XCTest, cargo test or NEEDS CLARIFICATION]  
**Target Platform**: [e.g., Linux server, iOS 15+, WASM or NEEDS CLARIFICATION]
**Project Type**: [e.g., library/cli/web-service/mobile-app/compiler/desktop-app or NEEDS CLARIFICATION]  
**Performance Goals**: [domain-specific, e.g., 1000 req/s, 10k lines/sec, 60 fps or NEEDS CLARIFICATION]  
**Constraints**: [domain-specific, e.g., <200ms p95, <100MB memory, offline-capable or NEEDS CLARIFICATION]  
**Scale/Scope**: [domain-specific, e.g., 10k users, 1M LOC, 50 screens or NEEDS CLARIFICATION]

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Assert compliance with each principle in `.specify/memory/constitution.md`.
Mark ✅ pass, ⚠ partial (requires Complexity Tracking entry below), or ❌ fail
(blocks merge).

| # | Principle | Status | Evidence / Notes |
|---|-----------|--------|------------------|
| I | Core Immutability — no edits under `core/**` | | |
| II | Non-Breaking — new config fields optional, hook signatures stable | | |
| III | Plugin-First — feature lives under `plugins/<name>/` or `framework/` | | |
| IV | Config-Driven Gating — no build tags, schema conditionals only | | |
| V | Multi-Tenancy First — `workspace_id` / `organization_id` present | | |
| VI | Observability — OTEL span + Prometheus metric + audit log covered | | |
| VII | Security by Default — secrets encrypted, TLS enforced, redaction hooked | | |
| VIII | Test Coverage — integration tests on real dependencies; Playwright for UI | | |
| IX | Docs & Schema Sync — `config.schema.json` + MDX + changelog in same PR | | |
| X | Dependency Hierarchy — no reverse imports; plugin modules independent | | |
| XI | Upstream-Mergeability — additive-by-sibling, E###_ migrations, schema overlay, hook points, drift-watched | | |
| XII | Code Quality — gofmt, golangci-lint, tsc, design system components, GoDoc | | |
| XIII | Testing Discipline — unit + integration (real deps) + E2E (Playwright), no flaky tests | | |
| XIV | UX Consistency — existing layout/components/RTK Query/RBAC gating/data-testid/loading states | | |
| XV | Performance Budget — <1ms p50 hot-path, <15s boot, <2s UI paint, no N+1, async observability | | |

Any ⚠ or ❌ row MUST have a corresponding row in **Complexity Tracking**
below with the reason it cannot be avoided and the simpler alternative that
was considered and rejected.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
# [REMOVE IF UNUSED] Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# [REMOVE IF UNUSED] Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# [REMOVE IF UNUSED] Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure: feature modules, UI flows, platform tests]
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
