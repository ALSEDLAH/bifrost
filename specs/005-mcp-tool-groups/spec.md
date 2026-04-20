# Feature Specification: MCP Tool Groups

**Feature Branch**: `005-mcp-tool-groups`
**Created**: 2026-04-20
**Status**: Draft

## User Scenarios & Testing

### User Story 1 — Admin creates a group and adds tools (P1)

**As a** platform admin
**I want** to create a named tool group and add a subset of tools from
one or more MCP clients into it
**So that** I can talk about "the read-only tools" or "the customer-facing
tools" as a single label across the admin UI.

**Acceptance**:
- Navigate to `/workspace/mcp-tool-groups`
- Click "New group" → name "read-only" + description → save
- Open the group → pick tools from a searchable list (grouped by MCP
  client) → save
- The group appears in the list with a count of its tool members

### User Story 2 — Admin edits or deletes a group (P2)

**As a** platform admin
**I want** to rename a group, add/remove tools, or delete it entirely
**So that** the label catalogue stays current as the tool inventory
changes.

**Acceptance**:
- Rename updates the group in the list
- Add/remove tools updates the member count
- Delete removes the group from the list

### User Story 3 — Admin scopes downstream access by group (P3 — future)

**As a** platform admin
**I want** to grant a virtual key or team access to a named tool group
**So that** the VK's tool list is filtered to only those tools.

*Explicitly out of scope for this spec — captured here so the data model
is forward-compatible.*

## Functional Requirements

- **FR-001**: A tool group has a unique `name` (case-insensitive), an
  optional `description`, and a list of tool references.
- **FR-002**: A tool reference is the pair `(mcp_client_id, tool_name)`
  where `mcp_client_id` is the upstream `TableMCPClient.ClientID`.
- **FR-003**: The CRUD API exposes: list groups, get group by id,
  create group, update group (name / description / tools), delete group.
- **FR-004**: Deleting an MCP client does not cascade — tool references
  pointing at the deleted client are tolerated and rendered as
  "(deleted)" in the UI.
- **FR-005**: The `/workspace/mcp-tool-groups` page renders a table of
  groups with columns `name`, `description`, `tools (count)`,
  `updated_at`, and per-row actions `Edit`, `Delete`. Empty state shows
  a "New group" CTA.
- **FR-006**: RBAC: new `MCPToolGroups` resource × the standard 6
  operations (Read, View, Create, Update, Delete, Download). Admin role
  gets all 5 mutating ops by default.

## Non-Functional Requirements

- **NFR-001**: The list endpoint returns in under 200 ms for 100 groups
  with 50 tool references each (typical catalogue).
- **NFR-002**: Concurrent edits from the same admin in two tabs do not
  silently overwrite each other — use last-write-wins with an
  `updated_at` timestamp shown in the UI so conflicts are visible.

## Success Criteria

- **SC-001**: An admin can create a 10-tool group and see it in the list
  in under 60 seconds from the empty state.
- **SC-002**: `/workspace/mcp-tool-groups` no longer renders
  `feature-status-panel` (spec 002 CI check passes).

## Out of Scope (v1)

- Virtual-key or team-level gating based on group membership (future
  spec; data model supports it).
- Tool-name glob patterns or regex membership (always explicit pairs).
- Group hierarchy / nested groups.
- Import/export of group definitions.

## Assumptions

- Upstream `config_mcp_clients` stays the single source of truth for
  MCP client metadata; this spec only references it by `client_id`.
- Tool names are stable; if a tool is renamed at the MCP server, the
  reference in the group becomes stale and is shown as "(missing)" in
  the UI. Admins must re-pick.

## Key Entities

- **ToolGroup**: persistent record of a named group of tool references.
- **ToolReference**: `(mcp_client_id, tool_name)` pair; stored as a JSON
  array on the group row.
