# Framework Changelog

## [Unreleased] — enterprise-parity branch

### Added (US2 — Granular RBAC, T020–T022 + T032/T033, 2026-04-20)

- `framework/tenancy/scopes.go` + `roles.go` back the new RBAC HTTP
  handler (`transports/bifrost-http/handlers/rbac.go`). 24 resources ×
  6 operations; 4 built-in roles (Owner / Admin / Manager / Member)
  seeded by the existing `migrationE004UsersAndRoles`.

### Descoped 2026-04-20 (per SR-01 reuse-over-new)

- `tables-enterprise/admin_api_key.go` + `migrationE005AdminAPIKeys`
  removed — upstream `auth_config` basic-auth already provides admin
  auth. Any `ent_admin_api_keys` table left in existing deployments
  from pre-revert images is harmless (orphan, no handler reads it).

### Added (US1 — Organizations & Workspaces, T033–T036)

- `framework/tenancy/orgs.go` — `OrgRepo` with `GetDefault`, `GetByID`,
  `Update`, `CreateMultiOrg`.
- `framework/tenancy/workspaces.go` — `WorkspaceRepo` with full CRUD,
  soft-delete (30-day grace per US1 edge case), `Restore`.
- `framework/configstore/tables-enterprise/{organization,workspace,user,role,user_role_assignment}.go`
  — 5 new GORM structs per data-model §1.
- `framework/configstore/migrations_enterprise.go` — `E004_orgs_workspaces_users_roles`
  migration creating the 5 tables, seeding the default org + default
  workspace (pointing at the UUIDs persisted by `E001` in
  `ent_system_defaults`), and seeding 4 built-in roles (Owner, Admin,
  Member, Manager).

### Added (Phase 2 — Foundational, T011–T020)

- `framework/tenancy/context.go` — `TenantContext` struct + Resolver
  enum carried through the plugin chain via BifrostContext keys.
- `framework/tenancy/keys.go` — authoritative context-key names
  (`BifrostContextKeyOrganizationID`, `WorkspaceID`, `RoleScopes`, etc.).
- `framework/tenancy/repository.go` — `ScopedDB(bctx, db, workspaceScoped)`
  helper that pre-filters every query by org + optional workspace.
- `framework/crypto/configkey.go` + `envelope.go` — unified at-rest
  encryption with two backends (configstore key default + BYOK envelope
  layout per research R-05).
- `framework/deploymentmode/mode.go` — deployment-mode enum
  (cloud / selfhosted / airgapped) + opinionated Defaults table.
- `framework/telemetry/phonehome.go` — phone-home gate enforcing
  airgapped=off and selfhosted=opt-in.
- `framework/configstore/tables-enterprise/system_defaults.go` +
  5 sidecar tenancy tables (virtual_key / team / customer / provider /
  provider_key).
- `framework/configstore/migrations_enterprise.go` — `E001` (seed
  default org UUIDs), `E002` (create sidecars + backfill), plus the
  `RegisterEnterpriseMigrations(ctx, db)` entry-point.
- `framework/logstore/tables_enterprise.go` — `TableLogTenancy` (1:1
  sidecar for upstream logs) + `TableAuditEntry` (audit table,
  foundational so every enterprise plugin can emit from day 1).
- `framework/logstore/migrations_enterprise.go` — `E003` creates both
  logstore tables and backfills log-tenancy for pre-existing rows.

### Notes

- No edits under `core/**` (Constitution Principle I).
- No edits to upstream `framework/configstore/tables/*.go` or
  `framework/logstore/tables.go` (Principle XI rule 1); enterprise
  tenancy attaches via sibling `tables-enterprise/` files and 1:1
  sidecar tables.
- Enterprise migration IDs use the `E###_<name>` prefix to sort
  disjoint from upstream's descriptive migration IDs in the shared
  `migrations` tracking table (Principle XI rule 2).
