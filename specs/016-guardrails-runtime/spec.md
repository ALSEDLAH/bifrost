# Feature Specification: Guardrails Runtime Enforcement

**Feature Branch**: `016-guardrails-runtime`
**Created**: 2026-04-20
**Status**: Draft

## Overview

Spec 010 shipped the config surface for guardrail providers + rules.
This spec ships the **enforcement plugin** — a Bifrost LLM plugin
that evaluates enabled rules on every inference request and either
blocks, flags, or logs the outcome. Closes the largest docs-vs-code
gap from the 2026-04-20 gap analysis.

## User Scenarios

### US1 — Block a credit-card prompt at the gateway (P1)

**As a** security admin
**I want** a regex rule matching credit-card patterns on input with
action=block to cause any matching request to fail with a clear error
**So that** PII stops at Bifrost instead of reaching the model.

### US2 — Flag a policy-violating output (P2)

**As a** compliance admin
**I want** an openai-moderation rule on output with action=flag to
mark responses in the audit trail without blocking the user
**So that** I can review without disrupting traffic.

### US3 — Log-only mode for rollout (P2)

**As an** operator piloting a new rule
**I want** action=log to only audit, not block, so I can see volume
before flipping to action=block
**So that** I don't cause outages while tuning.

## Functional Requirements

- **FR-001**: A new plugin `plugins/guardrailsruntime` registers as an
  `LLMPlugin` with `PreLLMHook` (input) and `PostLLMHook` (output).
- **FR-002**: On boot the plugin loads all `ent_guardrail_rules` +
  `ent_guardrail_providers` (where `enabled=true`), compiles regex
  patterns, and builds an in-memory index keyed by trigger
  (`input` / `output` / `both` maps both).
- **FR-003**: Rule evaluation per request extracts the plain text
  from `req.ChatRequest` / `req.ResponsesRequest` (concatenation of
  message contents). The extractor lives in a helper so future
  request types plug in cleanly.
- **FR-004**: Supported provider types (matching spec 010):
  - `regex`: `regexp.MustCompile(rule.Pattern).MatchString(text)`
  - `openai-moderation`: POST to `{base_url or "https://api.openai.com"}/v1/moderations`
    with `Authorization: Bearer <api_key>`; rule matches when any
    category is flagged (v1 treats all categories equally; category
    filters are phase 3).
  - `custom-webhook`: POST `{text, trigger, rule_id}` to the webhook's
    URL with a 2-second timeout; rule matches when response status
    is 2xx AND body contains `"match": true`.
- **FR-005**: Action semantics:
  - `block`: `PreLLMHook` returns `LLMPluginShortCircuit` with HTTP
    `451 Unavailable For Legal Reasons` + message `"guardrail
    blocked: <rule.name>"`; `PostLLMHook` blocking converts the
    upstream response to the same 451.
  - `flag`: attach `BifrostContextKeyGuardrailFlags` to context (JSON
    array of `{rule_id, rule_name, trigger, matched_at}`); request
    proceeds.
  - `log`: audit.Emit only; no context mutation.
- **FR-006**: Every block + flag fires an `audit.Emit` with
  `action=guardrail.block` or `guardrail.flag`, `resource_type=rule`,
  `resource_id=rule.id`, `outcome=denied` (block) or `allowed`
  (flag), `reason=<rule.name>`.
- **FR-007**: The admin CRUD handlers for rules + providers invalidate
  the plugin's cache on save/delete so edits take effect immediately.
- **FR-008**: Regex compilation failures at load time log a Warn
  and skip the rule (rest of the rule set still applies).
- **FR-009**: Moderation / webhook call failures do NOT block the
  request by default — they log a Warn and let the request through.
  An optional `fail_closed=true` config switch on the rule makes
  failures block (`block` action only). Default `false` (fail open).

## Non-Functional Requirements

- **NFR-001**: Regex evaluation hot-path cost < 50µs per rule for
  patterns without catastrophic backtracking. Patterns that time out
  (>10ms per evaluation) are disabled and logged Warn.
- **NFR-002**: Moderation / webhook external calls have a 2s timeout
  each. Total plugin latency budget per request = 2s max across all
  external rules.
- **NFR-003**: Rule cache refresh is O(N) over the rule count on
  every write; safe up to ~10k rules.

## Success Criteria

- **SC-001**: A regex rule with pattern `\b\d{16}\b` on input +
  action=block causes `POST /v1/chat/completions` with a 16-digit
  number in the prompt to return 451.
- **SC-002**: Without any guardrail rules configured, inference
  latency is unchanged (plugin short-circuits on empty rule set).
- **SC-003**: Audit verify (spec 015) includes every guardrail
  block/flag in the chain.

## Out of Scope (v1)

- CEL policy expressions (spec 010 mentioned CEL — v2).
- Streaming output scanning (first chunk only in v1; streaming
  support is phase 3).
- Per-category moderation filters (v1 flags any category).
- AWS Bedrock / Azure / GraySwan / Patronus provider types (docs
  mention them; plugin is extensible but v1 ships 3 provider types).
- Redaction actions (detect + modify content); v1 is detect-only.

## Assumptions

- The existing governance plugin's PreLLMHook runs BEFORE this one
  (governance decides routing; guardrails then evaluates content).
  Plugin ordering is configured in server.go plugin registration.
- Request text extraction covers chat and Responses shapes; other
  shapes (embeddings, speech input) are skipped — their content is
  not user prose.
