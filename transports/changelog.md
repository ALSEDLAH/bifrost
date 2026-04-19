# Transports Changelog

## [Unreleased] — enterprise-parity branch

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
