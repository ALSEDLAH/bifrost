# Feature Specification: Guardrails (config surface)

**Feature Branch**: `010-guardrails`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Guardrails inspect inference input or output and take an action
(block / flag / log) when a policy violates. Phase 1 ships the admin
**config surface** — Provider records (e.g., OpenAI Moderation
credentials, a regex engine) and Rule records that reference them.
The enforcement plugin that actually runs on each request is phase 2.

Shipping config first lets customers script their policy before the
runtime engine lands and gives the enforcement plugin a real data
model to plug into.

## User Scenarios

### US1 — Security admin adds an OpenAI Moderation provider (P1)

**As a** security admin
**I want** to register an OpenAI Moderation provider with my API key
**So that** I can reference it from rules that check inference inputs.

### US2 — Security admin writes a "block credit-card numbers" rule (P1)

**As a** security admin
**I want** to create a regex-based rule that blocks any inference
call whose input matches a credit-card pattern
**So that** PII is rejected at the gateway.

## Functional Requirements

- **FR-001**: New table `ent_guardrail_providers` with
  `{id, name, type, config JSON, enabled, updated_at}`. Supported
  types: `openai-moderation`, `regex`, `custom-webhook`.
- **FR-002**: New table `ent_guardrail_rules` with
  `{id, name, provider_id (nullable for regex), trigger, action,
  pattern, enabled, updated_at}`. `trigger ∈ {input, output, both}`,
  `action ∈ {block, flag, log}`.
- **FR-003**: CRUD APIs at `/api/guardrails/providers` and
  `/api/guardrails/rules` with the standard
  GET-list / POST / PATCH / DELETE shape.
- **FR-004**: `/workspace/guardrails/providers` and
  `/workspace/guardrails/rules` render real tables with create /
  edit dialogs. Current FeatureStatusPanel stubs are replaced.
- **FR-005**: RBAC uses existing resources
  `GuardrailsProviders`, `GuardrailRules`, `GuardrailsConfig`.
- **FR-006**: Rules reference providers by id. Deleting a provider
  that has rules is rejected with a 409 + error message listing the
  referencing rule names.

## Out of Scope (v1)

- Runtime enforcement plugin — phase 2 spec.
- Rule ordering / precedence semantics (all rules evaluated).
- Compound rules (AND / OR across providers).

## Success Criteria

- **SC-001**: An admin can register a provider + one regex rule in
  under 2 minutes starting from empty state.
- **SC-002**: SC-020 scanner no longer matches
  `guardrailsProviderView.tsx` or `guardrailsConfigurationView.tsx`.
