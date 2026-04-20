# Plan: Guardrails Runtime Enforcement

## Technical Context

- **Language**: Go 1.26 (new `plugins/guardrailsruntime` module)
- **Hooks**: `PreLLMHook` + `PostLLMHook` from `core/schemas` plugin API
- **Config source**: `configstore.ListGuardrailProviders` + `ListGuardrailRules` (spec 010)
- **Audit**: `audit.Emit` (spec 001 + HMAC chain from spec 015)

## Architecture

```
inference request
   │
   ▼
governance.PreLLMHook  (routing, tenant resolution)
   │
   ▼
guardrailsruntime.PreLLMHook   ──► scan input vs rules[trigger=input|both]
   │    matches + action=block  ──► short-circuit 451
   │    matches + action=flag   ──► ctx.SetValue(GuardrailFlags, ...)
   │    matches + action=log    ──► audit.Emit only
   ▼
provider call (hot path untouched)
   │
   ▼
guardrailsruntime.PostLLMHook  ──► scan output vs rules[trigger=output|both]
   │                             same action semantics
   ▼
response to client
```

## Data model

Reuses spec 010 tables — no schema change:
- `ent_guardrail_providers` — `{id, name, type, config JSON, enabled}`
- `ent_guardrail_rules` — `{id, name, provider_id, trigger, action, pattern, enabled}`

Plugin's in-memory index:
```go
type ruleEntry struct {
    id        string
    name      string
    trigger   string // "input" | "output" | "both"
    action    string // "block" | "flag" | "log"
    provider  *providerEntry // nil for type=regex
    pattern   string
    regex     *regexp.Regexp // compiled for type=regex
    failClosed bool
}

type providerEntry struct {
    id     string
    typ    string // "openai-moderation" | "custom-webhook" | "regex"
    apiKey string // openai
    baseURL string // openai
    webhookURL string // custom-webhook
}
```

## Constitution check
- Upstream-mergeability: new plugin dir, no upstream file edits. ✓
- Auditability: every block + flag emits to audit. ✓
- SR-01: authorised net-new per 2026-04-20 "do them all" directive. ✓

## Phases

- Phase 1: package skeleton + PluginName + Init + in-memory index + PreLLMHook regex-only path
- Phase 2: PostLLMHook + openai-moderation provider + custom-webhook provider
- Phase 3: handler invalidation hooks + audit.Emit wiring
- Phase 4: unit tests per provider + an integration test that wires the plugin to a fake PreLLMHook fixture
- Phase 5: e2e test via Playwright — configure a rule in the UI, send a blocked request, assert 451
