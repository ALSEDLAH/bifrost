// Built-in tenant resolvers — admin API key, service account key,
// virtual key. Session resolver lives in plugins/sso (registered via
// RegisterResolver to avoid a cross-plugin import).
//
// Each resolver inspects the HTTPRequest, looks up the credential in
// configstore (when needed), and returns a fully-populated
// TenantContext. Unknown credential prefixes are no-match (false, nil)
// so the next resolver gets a chance.

package enterprisegate

import (
	"context"
	"errors"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/tenancy"
	"gorm.io/gorm"
)

const (
	prefixAdminAPIKey       = "bf-admin-"
	prefixServiceAccountKey = "bf-svc-"
	prefixVirtualKey        = "sk-bf-"
)

// extractBearer pulls the token after "Bearer " from the Authorization
// header, normalized to lowercase scheme. Returns "" on miss.
func extractBearer(req *schemas.HTTPRequest) string {
	if req == nil {
		return ""
	}
	v, ok := req.Headers["authorization"]
	if !ok {
		v = req.Headers["Authorization"]
	}
	if v == "" {
		return ""
	}
	const scheme = "bearer "
	if len(v) > len(scheme) && strings.EqualFold(v[:len(scheme)], scheme) {
		return strings.TrimSpace(v[len(scheme):])
	}
	return ""
}

// ─── Admin API Key resolver ────────────────────────────────────────

type adminAPIKeyResolver struct {
	db *gorm.DB
}

func newAdminAPIKeyResolver(db *gorm.DB) *adminAPIKeyResolver {
	return &adminAPIKeyResolver{db: db}
}

func (r *adminAPIKeyResolver) Name() string { return "admin-api-key" }

func (r *adminAPIKeyResolver) Resolve(ctx context.Context, req *schemas.HTTPRequest) (tenancy.TenantContext, bool, error) {
	tok := extractBearer(req)
	if tok == "" || !strings.HasPrefix(tok, prefixAdminAPIKey) {
		return tenancy.TenantContext{}, false, nil
	}

	// In Phase 2 the admin_api_keys table doesn't exist yet (it lands
	// in T086 / Train A US5). For the gate's foundational role, accept
	// any well-formed admin token in single-org mode and fail-open to
	// the default tenant. Phase 3.5 (US5 implementation) replaces this
	// stub with a real lookup against ent_admin_api_keys.
	prefix := splitKeyPrefix(tok)
	if prefix == "" {
		return tenancy.TenantContext{}, true, errors.New("admin api key malformed")
	}
	// Default-org for now; once US5 lands the lookup populates a real
	// org from the key's row.
	return tenancy.TenantContext{
		// OrganizationID is filled in by the gate caller when this
		// resolver returns; the resolver itself doesn't know the
		// default. We stash the prefix in UserID for tracing.
		UserID:      prefix,
		ResolvedVia: tenancy.ResolverAdminAPIKey,
		RoleScopes:  []string{"*"}, // admin keys are full-scope by default until US2 lands per-key scope storage
	}, true, nil
}

// ─── Service Account Key resolver ──────────────────────────────────

type serviceAccountKeyResolver struct {
	db *gorm.DB
}

func newServiceAccountKeyResolver(db *gorm.DB) *serviceAccountKeyResolver {
	return &serviceAccountKeyResolver{db: db}
}

func (r *serviceAccountKeyResolver) Name() string { return "service-account-key" }

func (r *serviceAccountKeyResolver) Resolve(ctx context.Context, req *schemas.HTTPRequest) (tenancy.TenantContext, bool, error) {
	tok := extractBearer(req)
	if tok == "" || !strings.HasPrefix(tok, prefixServiceAccountKey) {
		return tenancy.TenantContext{}, false, nil
	}
	prefix := splitKeyPrefix(tok)
	if prefix == "" {
		return tenancy.TenantContext{}, true, errors.New("service-account key malformed")
	}
	// US17 lookup (T235+) replaces this with a real ent_service_account_keys query.
	return tenancy.TenantContext{
		UserID:      prefix,
		ResolvedVia: tenancy.ResolverServiceAccountKey,
		RoleScopes:  []string{"completions:write", "metrics:read"},
	}, true, nil
}

// ─── Virtual Key resolver ──────────────────────────────────────────

type virtualKeyResolver struct {
	db *gorm.DB
}

func newVirtualKeyResolver(db *gorm.DB) *virtualKeyResolver {
	return &virtualKeyResolver{db: db}
}

func (r *virtualKeyResolver) Name() string { return "virtual-key" }

func (r *virtualKeyResolver) Resolve(ctx context.Context, req *schemas.HTTPRequest) (tenancy.TenantContext, bool, error) {
	if req == nil {
		return tenancy.TenantContext{}, false, nil
	}

	// Virtual keys arrive on x-api-key (Bifrost convention); fall back
	// to Authorization: Bearer if the prefix matches.
	apiKey := req.Headers["x-api-key"]
	if apiKey == "" {
		apiKey = req.Headers["X-API-Key"]
	}
	if apiKey == "" {
		bearer := extractBearer(req)
		if strings.HasPrefix(bearer, prefixVirtualKey) {
			apiKey = bearer
		}
	}
	if apiKey == "" || !strings.HasPrefix(apiKey, prefixVirtualKey) {
		return tenancy.TenantContext{}, false, nil
	}

	// Real lookup: JOIN governance_virtual_keys -> ent_virtual_key_tenancy.
	// In Phase 2 we only have the sidecar table; the actual VK table is
	// upstream's governance_virtual_keys. Real lookups land in T028 once
	// the middleware integration test establishes the data path.
	// For now: claim the default tenant and attach a dummy VK marker so
	// downstream plugins (governance, metering) see a virtual-key
	// resolution.
	return tenancy.TenantContext{
		UserID:      "vk:" + splitKeyPrefix(apiKey),
		ResolvedVia: tenancy.ResolverVirtualKey,
		RoleScopes:  []string{"completions:write"},
	}, true, nil
}

// splitKeyPrefix returns the first 8 characters after the recognised
// prefix, or "" if the token is too short. Used for audit / log
// attribution without revealing the full secret.
func splitKeyPrefix(tok string) string {
	for _, p := range []string{prefixAdminAPIKey, prefixServiceAccountKey, prefixVirtualKey} {
		if strings.HasPrefix(tok, p) {
			rem := tok[len(p):]
			if len(rem) < 8 {
				return ""
			}
			return p + rem[:8]
		}
	}
	return ""
}
