# Tasks: Guardrails Runtime Enforcement

## Phase 1 — Plugin skeleton + regex path

- [X] T001 New package `plugins/guardrailsruntime/` with
      `plugin.go` (Plugin struct, PluginName, Init, GetName, Cleanup),
      `rules.go` (ruleEntry / providerEntry types), and `text.go`
      (request/response text extractors).
- [X] T002 Implement `loadRules(ctx, store)` — query providers + rules,
      build index. Regex compile errors → log + skip. Unknown provider
      types → log + skip.
- [X] T003 `PreLLMHook` — extract request text, evaluate rules whose
      trigger is `input` or `both`, apply action. Short-circuit on block.
- [X] T004 Expose `LLMPlugin` interface methods required by
      `lib.FindPluginAs` + plugin loader.

## Phase 2 — Providers

- [X] T005 Regex provider: reuse compiled regex in ruleEntry.
- [X] T006 OpenAI-moderation provider: HTTP client with 2s timeout,
      POST `/v1/moderations`, parse `results[0].flagged`.
- [X] T007 Custom-webhook provider: POST JSON body, parse
      `{"match": bool}` from response.

## Phase 3 — Output scanning + audit + invalidation

- [X] T008 `PostLLMHook` — extract response text, evaluate `output` +
      `both` rules, apply action. Block on post converts the response
      to the same 451.
- [X] T009 Every block + flag emits `audit.Emit` (spec 015 picks up
      HMAC automatically).
- [X] T010 Handler `handlers/guardrails.go` invalidates the plugin's
      cache on create/update/delete — add a setter `SetInvalidator`
      on the plugin, call from handler after a successful DB write.

## Phase 4 — Tests

- [X] T011 `plugins/guardrailsruntime/regex_test.go` — block/flag/log
      actions against a seeded rule set, table-driven.
- [X] T012 `plugins/guardrailsruntime/moderation_test.go` — httptest
      server simulating OpenAI moderation responses; assert match
      triggers block, failure fails open by default, fail_closed=true
      blocks on error.
- [X] T013 `plugins/guardrailsruntime/webhook_test.go` — same pattern.
- [X] T014 Integration: stand up a real plugin loaded into a mock
      Bifrost client, call ChatCompletion with a PII string, assert
      451 short-circuit + audit entry present.

## Phase 5 — E2E

- [X] T015 Playwright smoke at
      `ui/tests/e2e/enterprise/guardrails.spec.ts` — create a regex
      rule via the UI, POST a chat request with a matching prompt,
      assert 451 + rule name in the response body.

## Phase 6 — Polish

- [X] T016 Server wiring: register plugin at startup, install
      invalidator callback in guardrails handler.
- [X] T017 Update spec 002 research.md: guardrails row
      `SHIPPED-PHASE-1 → SHIPPED`.
