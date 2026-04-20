# Tasks: MCP Tool Groups

## Phase 1 — Backend

- [ ] T001 Add `MCPToolGroups` to `framework/tenancy/scopes.go` Resources list
      and to `ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx`
      enum. Grant the scope to the Admin role in
      `framework/configstore/migrations_enterprise.go`.
- [ ] T002 New table `TableMCPToolGroup` in
      `framework/configstore/tables-enterprise/mcp_tool_group.go`.
- [ ] T003 Migration `migrationE006MCPToolGroups` in
      `framework/configstore/migrations_enterprise.go` + register in
      `RegisterEnterpriseMigrations`.
- [ ] T004 ConfigStore interface additions in
      `framework/configstore/store.go`: `ListMCPToolGroups`,
      `GetMCPToolGroupByID`, `CreateMCPToolGroup`, `UpdateMCPToolGroup`,
      `DeleteMCPToolGroup`.
- [ ] T005 RDB impl at
      `framework/configstore/mcp_tool_groups_enterprise.go`.
- [ ] T006 Handler at
      `transports/bifrost-http/handlers/mcp_tool_groups.go`:
      GET/POST/PATCH/DELETE `/api/mcp/tool-groups`.
- [ ] T007 Register routes in
      `transports/bifrost-http/server/server.go` alongside alert
      channels.

## Phase 2 — UI

- [ ] T008 TypeScript types in `ui/lib/types/mcpToolGroups.ts`.
- [ ] T009 RTK hooks in `ui/lib/store/apis/mcpToolGroupsApi.ts`.
- [ ] T010 Add `MCPToolGroups` tag to `baseApi.ts` tagTypes.
- [ ] T011 Rewrite
      `ui/app/enterprise/components/mcp-tool-groups/mcpToolGroups.tsx`
      — flip from FeatureStatusPanel stub to a real table with create
      dialog, edit dialog, delete. Tool picker pulls from
      `useGetAllMCPClientsQuery` (existing RTK hook).

## Phase 3 — Polish

- [ ] T012 Update spec 002 `research.md` row for
      `mcp-tool-groups/mcpToolGroups.tsx`: `descoped` → `shipped in
      spec 005`.
