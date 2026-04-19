// Unit tests for resolver determinism.
//
// Validates research R-02: the resolution order is admin-key →
// service-account-key → session (registered later by sso) →
// virtual-key. Each auth-token type lands at the correct resolver
// regardless of header ordering, and unknown tokens fall through.

package enterprisegate

import (
	"context"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/tenancy"
)

func reqWith(headers map[string]string) *schemas.HTTPRequest {
	return &schemas.HTTPRequest{Headers: headers}
}

// helper: run a list of resolvers in priority order, returning the
// first matching TenantContext (mirrors plugin.resolveOne minus the
// fall-through).
func runResolvers(t *testing.T, resolvers []Resolver, req *schemas.HTTPRequest) (tenancy.TenantContext, string, error) {
	t.Helper()
	for _, r := range resolvers {
		tc, ok, err := r.Resolve(context.Background(), req)
		if err != nil {
			return tenancy.TenantContext{}, r.Name(), err
		}
		if ok {
			return tc, r.Name(), nil
		}
	}
	return tenancy.TenantContext{}, "none", nil
}

func TestResolutionOrder_AdminAPIKeyWinsOverVirtualKey(t *testing.T) {
	// Both an admin token and an x-api-key are present. Admin wins
	// because the admin resolver is registered first.
	req := reqWith(map[string]string{
		"authorization": "Bearer bf-admin-AbCdEfGhIjKl",
		"x-api-key":     "sk-bf-XXYYZZWW1122",
	})
	resolvers := []Resolver{
		newAdminAPIKeyResolver(nil),
		newServiceAccountKeyResolver(nil),
		newVirtualKeyResolver(nil),
	}
	_, by, err := runResolvers(t, resolvers, req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if by != "admin-api-key" {
		t.Fatalf("want admin-api-key wins; got %s", by)
	}
}

func TestResolutionOrder_ServiceAccountBeforeVirtualKey(t *testing.T) {
	req := reqWith(map[string]string{
		"authorization": "Bearer bf-svc-ServiceAcct1234",
		"x-api-key":     "sk-bf-XXYYZZWW1122",
	})
	resolvers := []Resolver{
		newAdminAPIKeyResolver(nil),
		newServiceAccountKeyResolver(nil),
		newVirtualKeyResolver(nil),
	}
	_, by, err := runResolvers(t, resolvers, req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if by != "service-account-key" {
		t.Fatalf("want service-account-key wins; got %s", by)
	}
}

func TestResolutionOrder_VirtualKeyOnly(t *testing.T) {
	req := reqWith(map[string]string{
		"x-api-key": "sk-bf-XXYYZZWW1122",
	})
	resolvers := []Resolver{
		newAdminAPIKeyResolver(nil),
		newServiceAccountKeyResolver(nil),
		newVirtualKeyResolver(nil),
	}
	tc, by, err := runResolvers(t, resolvers, req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if by != "virtual-key" {
		t.Fatalf("want virtual-key resolver; got %s", by)
	}
	if tc.ResolvedVia != tenancy.ResolverVirtualKey {
		t.Fatalf("ResolvedVia = %s; want virtual-key", tc.ResolvedVia)
	}
}

func TestResolutionOrder_NoCredentialsFallsThrough(t *testing.T) {
	req := reqWith(map[string]string{})
	resolvers := []Resolver{
		newAdminAPIKeyResolver(nil),
		newServiceAccountKeyResolver(nil),
		newVirtualKeyResolver(nil),
	}
	_, by, err := runResolvers(t, resolvers, req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if by != "none" {
		t.Fatalf("want fall-through (none); got %s", by)
	}
}

func TestResolutionOrder_MalformedAdminTokenIsAuthError(t *testing.T) {
	// Token has the right prefix but is too short to extract a usable suffix.
	req := reqWith(map[string]string{
		"authorization": "Bearer bf-admin-x",
	})
	resolvers := []Resolver{newAdminAPIKeyResolver(nil)}
	_, _, err := runResolvers(t, resolvers, req)
	if err == nil {
		t.Fatal("want auth error for malformed admin token; got nil")
	}
}

func TestExtractBearer_CaseInsensitiveScheme(t *testing.T) {
	tests := []struct {
		name string
		hdr  map[string]string
		want string
	}{
		{"lowercase", map[string]string{"authorization": "Bearer abc"}, "abc"},
		{"capitalized", map[string]string{"Authorization": "Bearer abc"}, "abc"},
		{"lowercase scheme", map[string]string{"authorization": "bearer abc"}, "abc"},
		{"missing", map[string]string{}, ""},
		{"non-bearer", map[string]string{"authorization": "Basic xxx"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractBearer(reqWith(tt.hdr)); got != tt.want {
				t.Fatalf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestSplitKeyPrefix(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"bf-admin-AbCdEfGhIjKl", "bf-admin-AbCdEfGh"},
		{"bf-svc-ServiceAcct1234", "bf-svc-ServiceA"},
		{"sk-bf-XXYYZZWW1122", "sk-bf-XXYYZZWW"},
		{"bf-admin-short", ""},     // suffix too short
		{"unknown-prefix-xxxx", ""}, // unknown prefix
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := splitKeyPrefix(tt.in); got != tt.want {
				t.Fatalf("got %q; want %q", got, tt.want)
			}
		})
	}
}
