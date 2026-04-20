# Transports Changelog

## [Unreleased] — enterprise-parity branch

### Added (US2 — Granular RBAC, T023–T025 + T032/T033, 2026-04-20)

- `transports/bifrost-http/handlers/rbac.go` — `/api/rbac/{meta,me}`
  plus CRUD on `/api/rbac/roles`, `/api/rbac/users`, `/api/rbac/users
  /:id/assignments`, `/api/rbac/assignments`. Backed by existing
  `tenancy.RoleRepo` and `ent_users` / `ent_roles` / `ent_user_role_
  assignments` tables (no new storage). v1 single-org mode uses
  `ent_system_defaults.default_organization_id` as the implicit org.
- `server/server.go` registers the handler via
  `NewRBACHandler(s.Config.ConfigStore.DB(), logger)`.

### Added (US4 — Audit Logs, T035–T037, 2026-04-20)

- No backend change — the existing `handlers/audit_logs.go` (list +
  CSV/JSON export under `/api/audit-logs*`) already covers US4.

### Added (US30 — MCP Auth Config, T075, 2026-04-20)

- No backend change — reuse of upstream `/api/oauth/config/:id/status`
  + DELETE endpoints.

### Descoped 2026-04-20 (per SR-01 reuse-over-new scope rule)

- US5 admin API keys: removed an earlier speculative
  `handlers/admin_api_keys.go` + `ent_admin_api_keys` table.
  Upstream `auth_config` basic-auth already provides admin auth.
- US30 T074 MCP Tool Groups: no upstream grouping column on
  `TableMCPClient`, no endpoint; net-new per SR-01. Fallback stub
  retained.
- US30 T076 Large Payload settings: no persistence endpoint exists;
  fallback stub retained.

### Added (US1 — Organizations & Workspaces, T037–T039)

- `transports/bifrost-http/handlers/organizations.go` — GET/PATCH
  `/v1/admin/organizations/current`. PATCH requires `team_mgmt:write`
  scope; emits `organization.update` audit entry with before/after
  snapshots.
- `transports/bifrost-http/handlers/workspaces.go` — full CRUD on
  `/v1/admin/workspaces/*`:
  - `GET /v1/admin/workspaces` — list (scope: `workspaces:read`)
  - `POST /v1/admin/workspaces` — create (scope: `workspaces:write`);
    409 on slug conflict
  - `GET /v1/admin/workspaces/{id}` — get (`workspaces:read`); returns
    404 for cross-org access (no existence-leak)
  - `PATCH /v1/admin/workspaces/{id}` — update (`workspaces:write`)
  - `DELETE /v1/admin/workspaces/{id}` — soft-delete with 30-day grace
    (`workspaces:delete`); emits `workspace.delete` audit
- `transports/bifrost-http/handlers/enterprise_helpers.go` — shared
  `writeJSON`/`writeJSONError`/`newAuditBctxFromFasthttp` helpers for
  enterprise handlers.
- `transports/config.schema.enterprise.json` — `enterprise.orgs_workspaces`
  block (enabled: bool).

### Added (Phase 1.5 — Upstream Sync Tooling, T324/T326)

- `transports/bifrost-http/lib/schema_loader_enterprise.go` — overlay
  loader composing `config.schema.enterprise.json` into upstream's
  `config.schema.json` at boot via single `$ref` anchor
  (Principle XI rule 3).
- `transports/bifrost-http/lib/middleware_enterprise.go` — extension
  registry (`EnterpriseMiddlewareProvider`,
  `RegisterEnterpriseMiddlewareProvider`,
  `EnterpriseMiddlewares()`) plus concrete `NewTenantResolveProvider`
  and `NewRBACEnforceProvider` producing 401/403 on auth/scope
  failures. `RequireScope("resource:verb")` marks a route as
  requiring a specific scope.

### Notes

- No edits to upstream `transports/bifrost-http/lib/middleware.go` body;
  enterprise middleware lives in the sibling `middleware_enterprise.go`
  (Constitution Principle XI rule 1 + 4).
- The single upstream touch on `transports/config.schema.json` is a
  4-line `enterprise` property with `$ref` to the overlay file;
  everything else lives in the overlay.
