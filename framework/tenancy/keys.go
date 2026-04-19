// Tenancy context keys carried in BifrostContext.
//
// These are NEW keys added by the enterprise fork; they do not appear in
// upstream's reserved-keys list (core/schemas/context.go). Plugins —
// specifically enterprise-gate — write them via the standard
// BifrostContext.SetValue() API.
//
// Constitution Principle XI rule 1 — sibling-file extension.

package tenancy

import "github.com/maximhq/bifrost/core/schemas"

// Context keys carried through the plugin chain. Authoritative names —
// downstream plugins (audit, license, governance, guardrails, metering,
// billing) read them via these constants.
const (
	// BifrostContextKeyOrganizationID stores the resolved organization
	// UUID v7 string for the current request.
	BifrostContextKeyOrganizationID schemas.BifrostContextKey = "ent-organization-id"

	// BifrostContextKeyWorkspaceID stores the resolved workspace UUID v7
	// string. Empty for org-level admin actions.
	BifrostContextKeyWorkspaceID schemas.BifrostContextKey = "ent-workspace-id"

	// BifrostContextKeyUserID stores the resolved user UUID v7 string.
	// Empty for service-account-key and admin-api-key flows.
	BifrostContextKeyUserID schemas.BifrostContextKey = "ent-user-id"

	// BifrostContextKeyRoleScopes stores []string of resolved scopes
	// in `<resource>:<verb>` form (e.g., "audit_logs:read").
	BifrostContextKeyRoleScopes schemas.BifrostContextKey = "ent-role-scopes"

	// BifrostContextKeyResolvedVia stores the Resolver value naming the
	// auth path that produced this context.
	BifrostContextKeyResolvedVia schemas.BifrostContextKey = "ent-resolved-via"

	// BifrostContextKeyTenantContext stores the entire TenantContext
	// struct for downstream plugins that prefer the typed accessor over
	// individual key reads.
	BifrostContextKeyTenantContext schemas.BifrostContextKey = "ent-tenant-context"

	// BifrostContextKeyLicense stores the license plugin's per-request
	// state (entitlements, expiry days, in-grace bool) so gated features
	// can call IsEntitled() without a global plugin lookup.
	BifrostContextKeyLicense schemas.BifrostContextKey = "ent-license"

	// BifrostContextKeyTier stores the cloud-mode tier (Dev/Prod/Enterprise)
	// resolved by the billing plugin for cloud deployments.
	BifrostContextKeyTier schemas.BifrostContextKey = "ent-tier"
)
