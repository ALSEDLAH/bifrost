# Plan: Alert Channels

## Technical Context

- **Language**: Go 1.26 (framework + plugin + HTTP handler), TypeScript 5.9 / React 19 (UI)
- **DB**: GORM over SQLite/Postgres (via existing configstore)
- **HTTP**: fasthttp routes under `transports/bifrost-http/handlers/`
- **UI**: RTK Query + shadcn table components, same patterns as access-profiles / rbac pages

## Architecture

```
┌─── UI: /workspace/alert-channels ────┐
│ alertChannelsView.tsx (list+dialog)  │
└─── RTK: useGet/Create/Update/Delete ─┘
                │
                ▼
┌─── HTTP: /api/alert-channels ────────┐
│ alert_channels.go (handlers)         │
└─── ConfigStore.AlertChannels* ───────┘
                │
                ▼
┌─── DB: ent_alert_channels ───────────┐

┌─── Plugin: governance/tracker ───────┐
│ emitThreshold() calls                │
│   dispatcher.Send(channels, event)   │
└─── Dispatcher (new package) ─────────┘
                │
      ┌─────────┴─────────┐
      ▼                   ▼
   webhook              slack
   POST JSON            POST formatted text
```

## Data Model

New GORM table `tables_enterprise.TableAlertChannel`:
```go
type TableAlertChannel struct {
    ID        string    `gorm:"primaryKey;type:varchar(36)"`
    Name      string    `gorm:"type:varchar(255);not null"`
    Type      string    `gorm:"type:varchar(50);not null"` // "webhook" | "slack"
    ConfigRaw string    `gorm:"type:text;column:config"`    // JSON
    Enabled   bool      `gorm:"not null;default:true"`
    CreatedAt time.Time `gorm:"not null"`
    UpdatedAt time.Time `gorm:"not null"`
}
```
Registered in `framework/configstore/migrations_enterprise.go`.

## Constitution Check

- **I. Upstream-Mergeability**: Table + handler live under
  `*-enterprise` sibling files; dispatcher lives in a new
  `framework/alertchannels/` package so the governance plugin just
  calls one function. Minimal change to existing governance code:
  one call site + one setter. ✓
- **V. RBAC**: New RbacResource.AlertChannels × all 6 operations. ✓
- **XI. SR-01 Reuse-over-new**: Alert channels are genuinely net-new
  — no upstream plumbing exists. This spec is authorized by the user
  explicitly (session 2026-04-20: "go and do them, except PII"). ✓

## Phases

- **Phase 0 — Research** (this file)
- **Phase 1 — Design & Contracts** (below)
- **Phase 2 — Tasks** (tasks.md)
- **Phase 3 — Implementation** (per tasks)
