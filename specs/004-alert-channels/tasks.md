# Tasks: Alert Channels

## Phase 1: Setup

- [ ] T001 New RBAC resource `AlertChannels` in `framework/tenancy/scopes.go`
      (alongside existing resources) + migration to seed default role grants
      in `framework/configstore/migrations_enterprise.go`.

## Phase 2: Foundational

- [ ] T002 Table `TableAlertChannel` in
      `framework/configstore/tables-enterprise/alert_channel.go` — fields per data-model.
- [ ] T003 Register migration in `framework/configstore/migrations_enterprise.go`.

## Phase 3: Backend (P1)

- [ ] T004 ConfigStore interface additions in `framework/configstore/store.go`:
      `ListAlertChannels`, `GetAlertChannelByID`, `CreateAlertChannel`,
      `UpdateAlertChannel`, `DeleteAlertChannel`.
- [ ] T005 RDB impl in `framework/configstore/alert_channels_enterprise.go`.
- [ ] T006 New package `framework/alertchannels/` with `Dispatcher` interface,
      `webhookDispatcher`, `slackDispatcher`, and `Send(channels, event)` helper
      that fans out fire-and-forget to all enabled channels. 5s HTTP timeout.
- [ ] T007 Wire dispatcher into governance plugin: `UsageTracker` holds a
      `dispatcher` and optional `channelsFetcher func() []TableAlertChannel`;
      `emitThreshold` calls `dispatcher.Send(fetcher(), event)`. New setter
      `SetAlertDispatcher(d, f)` in `plugins/governance/tracker_thresholds.go`.
- [ ] T008 Handlers at `transports/bifrost-http/handlers/alert_channels.go`:
      GET list, POST create, PATCH update, DELETE, POST /test — all gated by
      RBAC middleware.
- [ ] T009 Route registration in `transports/bifrost-http/server/server.go`.
- [ ] T010 Wire `SetAlertDispatcher` at server startup in `server.go`, passing
      a fetcher that calls ConfigStore.ListAlertChannels.

## Phase 4: UI (P1)

- [ ] T011 TypeScript types in `ui/lib/types/alertChannels.ts`
      (AlertChannel, AlertChannelType, WebhookConfig, SlackConfig).
- [ ] T012 RTK hooks in `ui/lib/store/apis/alertChannelsApi.ts`:
      list/create/update/delete/test queries + mutations.
- [ ] T013 Rewrite
      `ui/app/enterprise/components/alert-channels/alertChannelsView.tsx`
      — flip from FeatureStatusPanel to a real table. Row actions: edit
      dialog, Send test, Delete. Empty-state "New channel" CTA.
- [ ] T014 Add `RbacResource.AlertChannels` to the UI enum in
      `ui/app/enterprise/lib/index.ts` and `ui/lib/types/enterprise.ts`.

## Phase 5: Polish

- [ ] T015 Go unit test for the dispatcher: mocks an HTTP server, asserts
      Slack payload contains the budget_id + level, asserts failed 4xx/5xx
      are logged and don't panic. File: `framework/alertchannels/dispatcher_test.go`.
- [ ] T016 Go handler test for POST+GET+DELETE round-trip +
      `/test` endpoint. File: `transports/bifrost-http/handlers/alert_channels_test.go`.
- [ ] T017 Update spec 002 research.md row for alert-channels:
      `needs-own-spec` → `shipped in spec 004`. Amend spec 002 tasks.md
      T007 similarly.
- [ ] T018 Final verify: docker build green, /workspace/alert-channels
      renders real table, SC-020 scanner still green (no marketing string
      in alert-channels/*).
