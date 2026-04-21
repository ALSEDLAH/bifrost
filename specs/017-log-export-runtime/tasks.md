# Tasks: Log Export Runtime

## Phase 1 — Plugin skeleton + Datadog adapter

- [X] T001 New package `plugins/logexport/` — go.mod, plugin.go
      (Plugin struct, Init, GetName, Cleanup), connectors.go
      (TTL-cached connector loader from configstore).
- [X] T002 Implement `ObservabilityPlugin.Inject(ctx, trace) error`
      that iterates enabled datadog connectors and POSTs a log record.
- [X] T003 Datadog payload builder — extract trace fields into the
      Logs-API JSON shape. Best-effort; skip missing fields.
- [X] T004 BigQuery connector type: one-time Info log, skip.
      Wired so the skip path is covered in tests.

## Phase 2 — Wire-up + cache invalidation

- [X] T005 Server startup (transports/bifrost-http/server/server.go):
      Init plugin, register via Config.ReloadPlugin, install
      plugin.Invalidate as an invalidator on the log-export CRUD
      handlers from spec 008.
- [X] T006 handlers/log_export_connectors.go: SetInvalidator +
      invocation on every successful create/update/delete.

## Phase 3 — Tests

- [X] T007 Unit test: Inject with httptest.Server acting as Datadog
      intake, assert POST body contains {ddsource, service, message,
      request_id, provider, model} and correct Datadog `DD-API-KEY`
      auth header.
- [X] T008 Unit test: BigQuery connector type is skipped cleanly
      with no HTTP call issued.
- [X] T009 Unit test: Invalidate() reloads the cache — add a row,
      invalidate, assert next Inject fires the new connector.
- [X] T010 Integration test: full plugin pipeline with a real
      SQLite configstore + fake Datadog server, seed one connector,
      drive Inject with a synthetic Trace, assert intake POST lands.

## Phase 4 — Polish

- [X] T011 Update spec 002 research.md rows 6+7 (BigQuery + Datadog):
      `descope → FeatureStatusPanel` → `config in spec 008, runtime
      (datadog) in spec 017, bigquery deferred to spec 018`.
- [X] T012 Update spec 017 tasks.md checklist on completion, merge
      to main, push.
