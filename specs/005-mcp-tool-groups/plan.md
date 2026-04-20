# Plan: MCP Tool Groups

## Technical Context

- **Language**: Go 1.26 (framework + handlers), TypeScript 5.9 / React 19 (UI)
- **DB**: GORM over SQLite/Postgres (via existing configstore)
- **HTTP**: fasthttp routes under `transports/bifrost-http/handlers/`
- **UI**: RTK Query + shadcn Table / Dialog / MultiSelect, same patterns as alert-channels

## Architecture

```
┌─── UI: /workspace/mcp-tool-groups ────┐
│ mcpToolGroups.tsx (list+edit dialog)  │
└─── RTK: useGet/Create/Update/Delete ──┘
                │
                ▼
┌─── HTTP: /api/mcp/tool-groups ────────┐
│ mcp_tool_groups.go handlers           │
└─── ConfigStore CRUD methods ──────────┘
                │
                ▼
┌─── DB: ent_mcp_tool_groups ───────────┐
│ {id, name, description,               │
│  tools_json, created_at, updated_at}  │
```

Tool discovery for the UI picker reuses the existing `TableMCPClient`
reads — the handler does not re-fetch from the MCP server itself.

## Data Model

New GORM table `tables_enterprise.TableMCPToolGroup`:
```go
type TableMCPToolGroup struct {
    ID          string    `gorm:"primaryKey;type:varchar(36)"`
    Name        string    `gorm:"type:varchar(255);not null;uniqueIndex"`
    Description string    `gorm:"type:text"`
    ToolsJSON   string    `gorm:"type:text;column:tools"` // JSON []ToolRef
    CreatedAt   time.Time `gorm:"not null"`
    UpdatedAt   time.Time `gorm:"not null"`
}

// ToolRef (not stored as its own table) — JSON shape on the row:
// {"mcp_client_id": "<client-id>", "tool_name": "<tool>"}
```
Registered in `framework/configstore/migrations_enterprise.go` as
`migrationE006MCPToolGroups`.

## Constitution Check

- **I. Upstream-Mergeability**: Table + handler live under
  `*-enterprise` sibling files; single JSON column avoids modifying
  upstream `config_mcp_clients`. ✓
- **V. RBAC**: New `RbacResource.MCPToolGroups` × 6 ops. ✓
- **XI. SR-01 Reuse-over-new**: Net-new feature explicitly authorized
  by the user (session 2026-04-20). ✓

## Phases

- Phase 0 — Research (implicit; data model + arch captured above)
- Phase 1 — Tasks (tasks.md)
- Phase 2 — Implementation (per tasks)
