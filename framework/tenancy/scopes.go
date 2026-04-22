// Scope definitions for RBAC (US2).
//
// Aligned to the frontend's RbacResource and RbacOperation enums
// defined in ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx.
// The exact counts evolve as the catalog grows — code that needs the
// numbers should call len(Resources) / len(Operations) rather than
// hardcode a literal.

package tenancy

// Resources enumerates every RBAC resource matching the frontend enum.
var Resources = []string{
	"GuardrailsConfig",
	"GuardrailsProviders",
	"GuardrailRules",
	"UserProvisioning",
	"Cluster",
	"Settings",
	"Users",
	"Logs",
	"Observability",
	"VirtualKeys",
	"ModelProvider",
	"Plugins",
	"MCPGateway",
	"AdaptiveRouter",
	"AuditLogs",
	"Customers",
	"Teams",
	"RBAC",
	"Governance",
	"RoutingRules",
	"PIIRedactor",
	"PromptRepository",
	"PromptDeploymentStrategy",
	"AccessProfiles",
	"AlertChannels",
	"MCPToolGroups",
}

// Operations enumerates the 6 RBAC operations matching the frontend enum.
var Operations = []string{
	"Read",
	"View",
	"Create",
	"Update",
	"Delete",
	"Download",
}

// AllScopes returns every valid scope string (Resource.Operation).
func AllScopes() []string {
	scopes := make([]string, 0, len(Resources)*len(Operations))
	for _, r := range Resources {
		for _, o := range Operations {
			scopes = append(scopes, r+"."+o)
		}
	}
	return scopes
}

// HasScope checks if a list of granted scopes contains the required scope.
// Wildcard "*" in the granted list satisfies any scope.
func HasScope(granted []string, resource, operation string) bool {
	required := resource + "." + operation
	for _, s := range granted {
		if s == "*" || s == required {
			return true
		}
	}
	return false
}

// HasAnyScope checks if the granted scopes include any operation on the resource.
func HasAnyScope(granted []string, resource string) bool {
	for _, s := range granted {
		if s == "*" {
			return true
		}
		// Check if scope starts with "Resource."
		if len(s) > len(resource)+1 && s[:len(resource)+1] == resource+"." {
			return true
		}
	}
	return false
}
