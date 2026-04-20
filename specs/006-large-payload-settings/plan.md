# Plan: Large Payload Settings

## Architecture

```
┌─── UI: clientSettingsView (existing) ──┐
│   LargePayloadSettingsFragment (NEW)   │  (6 number inputs + toggle)
└── useGet/UpdateLargePayloadConfig API ─┘
                │
                ▼
┌─── HTTP: /api/config/large-payload ────┐
│ large_payload_config.go handlers       │
└─── ConfigStore.GetLargePayload... ─────┘
                │
                ▼
┌─── DB: ent_large_payload_config ───────┐

┌─── Middleware: injectLargePayloadCtx ──┐  (TTL cache 30s)
│ Sets BifrostContextKey* on every       │
│ inference request                      │
└────────────────────────────────────────┘

┌─── Server startup hook ────────────────┐
│ Reads config + sets                    │
│ lib.Config.StreamingDecompressThreshold│
└────────────────────────────────────────┘
```

## Data Model

```go
package tables_enterprise

type TableLargePayloadConfig struct {
    ID                    string `gorm:"primaryKey;type:varchar(36)"` // always "default"
    Enabled               bool
    RequestThresholdBytes int64
    ResponseThresholdBytes int64
    PrefetchSizeBytes     int64
    MaxPayloadBytes       int64
    TruncatedLogBytes     int64
    UpdatedAt             time.Time
}
```

Singleton: handler upserts via ID="default"; GET returns the row or a
zero-value in-memory struct if no row exists.

## Constitution check
- Upstream-mergeability: sibling-file only; new middleware lives next
  to alert-channel wiring. No upstream file changes. ✓
- SR-01 reuse: exposing pre-existing context keys that consumers
  already respect — this IS an "expose hidden logic" win. ✓
