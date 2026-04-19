// Integration test: enterprise-gate against real PostgreSQL.
//
// Validates the full Init -> migrate -> resolve loop. Constitution
// Principle VIII (real dependencies, no mocks at integration tier).
//
//	docker compose -f docker-compose.enterprise.yml up -d postgres
//	BIFROST_ENT_PG_DSN="host=localhost port=55432 user=bifrost password=bifrost_test dbname=bifrost_enterprise sslmode=disable" \
//	  go test -tags=integration ./plugins/enterprise-gate/...
//
//go:build integration

package enterprisegate_test

import (
	"context"
	"os"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/tenancy"
	enterprisegate "github.com/maximhq/bifrost/plugins/enterprise-gate"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newPG(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BIFROST_ENT_PG_DSN")
	if dsn == "" {
		t.Skip("BIFROST_ENT_PG_DSN not set; skipping (expected outside CI)")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		// Wipe enterprise tables so reruns are clean.
		for _, tbl := range []string{
			"ent_audit_entries", "ent_log_tenancy",
			"ent_provider_key_tenancy", "ent_provider_tenancy",
			"ent_customer_tenancy", "ent_team_tenancy", "ent_virtual_key_tenancy",
			"ent_system_defaults", "migrations",
		} {
			_ = db.Migrator().DropTable(tbl)
		}
	})
	return db
}

func TestEnterpriseGate_InitAndResolve(t *testing.T) {
	db := newPG(t)
	ctx := context.Background()

	// Same DB serves as both configstore and logstore in this test.
	p, err := enterprisegate.Init(ctx, db, db, nil, nil)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer p.Cleanup()

	// Hit the gate with a virtual-key request; expect a populated
	// TenantContext with ResolverVirtualKey.
	bctx := schemas.NewBifrostContext(ctx)
	req := &schemas.HTTPRequest{
		Headers: map[string]string{
			"x-api-key": "sk-bf-IntegrationTest1234",
		},
	}
	resp, err := p.HTTPTransportPreHook(bctx, req)
	if err != nil {
		t.Fatalf("PreHook: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected continuation (resp == nil); got status=%d body=%s", resp.StatusCode, string(resp.Body))
	}

	// Validate the BifrostContext got populated.
	v := bctx.GetValue(tenancy.BifrostContextKeyTenantContext)
	if v == nil {
		t.Fatal("TenantContext key not set on bctx")
	}
	tc, ok := v.(tenancy.TenantContext)
	if !ok {
		t.Fatalf("TenantContext type wrong: %T", v)
	}
	if tc.ResolvedVia != tenancy.ResolverVirtualKey {
		t.Fatalf("ResolvedVia = %s; want virtual-key", tc.ResolvedVia)
	}

	// Hit the gate with no credentials; in single-org mode (default
	// for selfhosted), should fall through to the synthetic default.
	bctx2 := schemas.NewBifrostContext(ctx)
	if _, err := p.HTTPTransportPreHook(bctx2, &schemas.HTTPRequest{Headers: map[string]string{}}); err != nil {
		t.Fatalf("PreHook (no creds): %v", err)
	}
	v2 := bctx2.GetValue(tenancy.BifrostContextKeyTenantContext)
	if tc2, ok := v2.(tenancy.TenantContext); !ok || tc2.ResolvedVia != tenancy.ResolverDefault {
		t.Fatalf("expected ResolverDefault on no-creds; got %v", v2)
	}
}
