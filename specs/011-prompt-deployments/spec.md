# Feature Specification: Prompt Deployments

**Feature Branch**: `011-prompt-deployments`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Prompts already version through `TablePromptVersion` with `is_latest`
as the only built-in pointer — fine for dev, not for production where
you want to pin a specific version as "the one inference calls".
This spec adds **deployment labels** (production / staging) that
point a prompt at a specific version without modifying upstream
tables. v1 ships the label storage + admin UI; hooking the prompt
runtime so `promptName` resolves to the production label (instead of
`is_latest`) is phase 2.

## User Scenarios

### US1 — Prompt author promotes v7 to production (P1)

**As a** prompt author
**I want** to click "Promote to production" next to version 7
**So that** a permanent marker records that v7 is the production
version (even if v8 becomes `is_latest` later).

### US2 — Prompt author rolls back to an older version (P2)

**As a** prompt author
**I want** to re-promote v5 as production when v7 regresses
**So that** the rollback is one click and is audit-logged.

## Functional Requirements

- **FR-001**: New table `ent_prompt_deployments` with composite PK
  `(prompt_id, label)`, plus `{version_id, promoted_by,
  promoted_at}`. Labels: `production`, `staging` (extensible).
- **FR-002**: CRUD API:
  - `GET /api/prompts/:prompt_id/deployments` → list labels for this prompt.
  - `PUT /api/prompts/:prompt_id/deployments/:label` with body
    `{version_id}` → set/move the label.
  - `DELETE /api/prompts/:prompt_id/deployments/:label` → unset.
- **FR-003**: UI `PromptDeploymentView` renders:
  - Current production + staging labels (with "move" and "clear"
    actions).
  - Version history list with "Promote to production" / "Promote to
    staging" buttons.
- **FR-004**: `PromptDeploymentsAccordionItem` preserves the existing
  prop signature (`activeSection`) and embeds the same view inside
  the accordion pane of the prompt-detail sidebar.
- **FR-005**: Runtime prompt resolution using the labels is **out
  of scope** for v1 — the UI clearly says so.

## Success Criteria

- **SC-001**: An author can promote a version to production in under
  30 seconds from the prompt-detail page.
- **SC-002**: SC-020 scanner no longer matches
  `promptDeploymentView.tsx` or `promptDeploymentsAccordionItem.tsx`.
