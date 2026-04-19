// Package tenancy implements the multi-tenant primitive that every
// enterprise feature scopes against.
//
// Constitution Principle V — multi-tenancy first. Every tenant-scoped
// table carries `organization_id` + `workspace_id`; every request-path
// plugin reads tenant context from this package's BifrostContext keys.
//
// Constitution Principle XI rule 1 — sibling-file extension; nothing in
// upstream's core/, framework/configstore/, or transports/ is modified.
//
// The TenantContext is populated by the enterprise-gate plugin's HTTP
// pre-hook and carried through the plugin chain via BifrostContext keys
// defined in keys.go.
package tenancy

import (
	"github.com/maximhq/bifrost/framework/deploymentmode"
)

// TenantContext is the canonical per-request tenant identity. It is
// constructed by the enterprise-gate plugin from the request's auth
// material (admin API key, service account key, session cookie, or
// virtual key) and stored in BifrostContext for downstream plugins.
type TenantContext struct {
	OrganizationID string   // UUID v7 of the resolved organization
	WorkspaceID    string   // UUID v7 of the resolved workspace (may be empty for org-level admin actions)
	UserID         string   // UUID v7 of the resolved user (may be empty for service-account / admin-key flows)
	RoleScopes     []string // resolved scopes (e.g., "workspaces:write", "metrics:read")
	ResolvedVia    Resolver // which auth path produced this context
}

// Resolver names the auth-resolution path that produced a TenantContext.
// Used in audit entries and to pick policy variants per resolution
// origin (e.g., admin-API-key requests bypass per-key budget enforcement).
type Resolver string

const (
	// ResolverSession means a UI session cookie produced the context.
	ResolverSession Resolver = "session"
	// ResolverAdminAPIKey means an org-level admin API key (bf-admin-...).
	ResolverAdminAPIKey Resolver = "admin-api-key"
	// ResolverServiceAccountKey means a workspace-scoped service account key (bf-svc-...).
	ResolverServiceAccountKey Resolver = "service-account-key"
	// ResolverVirtualKey means a per-workspace virtual key (sk-bf-...).
	ResolverVirtualKey Resolver = "virtual-key"
	// ResolverDefault means single-org synthetic default (no enterprise auth in play).
	ResolverDefault Resolver = "default"
)

// IsZero reports whether the TenantContext was never resolved.
func (t TenantContext) IsZero() bool {
	return t.OrganizationID == ""
}

// IsDefault reports whether this is the synthetic default-org context
// used by single-org deployments before any explicit tenant is created.
func (t TenantContext) IsDefault() bool {
	return t.ResolvedVia == ResolverDefault
}

// HasScope reports whether the resolved scopes include `<resource>:<verb>`.
// Verb checks are exact-match: a holder of "workspaces:read" cannot
// "workspaces:write".
func (t TenantContext) HasScope(resourceVerb string) bool {
	for _, s := range t.RoleScopes {
		if s == resourceVerb {
			return true
		}
	}
	return false
}

// MultiOrgEnabled is a convenience that combines the deployment mode's
// default with any per-tenancy override. v1 single-org-mode deployments
// always return false; cloud mode returns true.
func MultiOrgEnabled() bool {
	return deploymentmode.CurrentDefaults().MultiOrgEnabled
}
