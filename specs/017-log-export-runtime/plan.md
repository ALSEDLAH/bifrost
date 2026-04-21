# Plan: Log Export Runtime

## Architecture

```
inference request → provider call → response to client
                                          │
                                          ▼
                                  core/trace.Inject (async, post-response)
                                          │
                                          ▼
                         plugins/logexport.Inject(ctx, trace)
                                          │
                    ┌──────── list enabled connectors ─────────┐
                    │  (TTL 30s; Invalidate() on admin writes) │
                    └─────────────────┬────────────────────────┘
                                      │
                   ┌──────────────────┴──────────────────┐
                   ▼                                     ▼
         type=datadog                           type=bigquery
         POST /api/v2/logs                      skipped in v1
         (2s timeout)                           (spec 018)
```

## Data flow

1. Plugin loads `ent_log_export_connectors` rows where
   `enabled=true` into a `sync.Map` keyed by ID, refreshed every 30s
   OR when `Invalidate()` is called.
2. On `Inject(trace)`, clone the connector list (lock-free read),
   iterate, call the right adapter.
3. Datadog adapter parses the Config JSON for `{api_key, site,
   tags}`, constructs the intake URL, POSTs a single log event,
   logs errors.

## Data model

No new tables. Reuses spec 008's `ent_log_export_connectors`.

## Reused code

- Trace extraction: `trace.RequestID`, `trace.ModelRequested`,
  `trace.ProviderUsed`, `trace.Error`, `trace.Latency` are on the
  core `schemas.Trace` struct (already consumed by the telemetry
  + audit plugins).

## Constitution check
- Upstream-mergeability: brand-new plugin dir; no edits to upstream
  files beyond `go.work` (+ Dockerfile.local wiring). ✓
- SR-01: authorised net-new per 2026-04-20 loop directive. ✓
