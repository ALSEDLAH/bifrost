// Integration test for OrgRepo + WorkspaceRepo.
//
// Constitution Principle VIII — real dependencies. Run with:
//
//	docker compose -f docker-compose.enterprise.yml up -d postgres
//	BIFROST_ENT_PG_DSN="host=localhost port=55432 user=bifrost password=bifrost_test dbname=bifrost_enterprise sslmode=disable" \
//	  go test -tags=integration ./framework/tenancy/...
//
//go:build integration

package tenancy_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/tenancy"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newOrgsDB(t *testing.T) *gorm.DB {
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
		for _, tbl := range []string{
			"ent_user_role_assignments", "ent_roles", "ent_users",
			"ent_workspaces", "ent_organizations",
			"ent_virtual_key_tenancy", "ent_team_tenancy",
			"ent_customer_tenancy", "ent_provider_tenancy",
			"ent_provider_key_tenancy", "ent_system_defaults",
			"migrations",
		} {
			_ = db.Migrator().DropTable(tbl)
		}
	})
	return db
}

// TestOrgWorkspaceRepo_DefaultSeedExists validates that after running
// the full enterprise migration suite, the default org + default
// workspace + 4 built-in roles exist.
func TestOrgWorkspaceRepo_DefaultSeedExists(t *testing.T) {
	db := newOrgsDB(t)
	ctx := context.Background()

	if err := configstore.RegisterEnterpriseMigrations(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	orgs := tenancy.NewOrgRepo(db)
	defaultOrg, err := orgs.GetDefault(ctx)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if !defaultOrg.IsDefault || defaultOrg.Name != "Default" {
		t.Fatalf("default org malformed: %+v", defaultOrg)
	}

	wsRepo := tenancy.NewWorkspaceRepo(db)
	list, err := wsRepo.List(ctx, defaultOrg.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Slug != "default" {
		t.Fatalf("expected exactly one default workspace; got %+v", list)
	}

	// Roles: exactly 4 built-ins.
	var roleCount int64
	if err := db.Model(&tables_enterprise.TableRole{}).
		Where("organization_id = ? AND is_builtin = ?", defaultOrg.ID, true).
		Count(&roleCount).Error; err != nil {
		t.Fatalf("role count: %v", err)
	}
	if roleCount != 4 {
		t.Fatalf("want 4 built-in roles; got %d", roleCount)
	}
}

// TestWorkspaceRepo_CRUD_SoftDeleteRestore walks the full lifecycle:
// create → list → get → patch → soft-delete → restore.
func TestWorkspaceRepo_CRUD_SoftDeleteRestore(t *testing.T) {
	db := newOrgsDB(t)
	ctx := context.Background()
	if err := configstore.RegisterEnterpriseMigrations(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	orgs := tenancy.NewOrgRepo(db)
	org, _ := orgs.GetDefault(ctx)
	wsRepo := tenancy.NewWorkspaceRepo(db)

	// Create
	ws, err := wsRepo.Create(ctx, org.ID, "Product", "product", "Product team workspace")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Slug conflict
	if _, err := wsRepo.Create(ctx, org.ID, "Product 2", "product", ""); !errors.Is(err, tenancy.ErrWorkspaceSlugConflict) {
		t.Fatalf("want ErrWorkspaceSlugConflict; got %v", err)
	}

	// Get
	got, err := wsRepo.Get(ctx, org.ID, ws.ID)
	if err != nil || got.Slug != "product" {
		t.Fatalf("Get: err=%v, got=%+v", err, got)
	}

	// Patch
	retention := 30
	patched, err := wsRepo.Patch(ctx, org.ID, ws.ID, map[string]any{
		"description":        "Product team — updated",
		"log_retention_days": retention,
	})
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if patched.Description != "Product team — updated" || patched.LogRetentionDays == nil || *patched.LogRetentionDays != 30 {
		t.Fatalf("patch did not apply: %+v", patched)
	}

	// Soft-delete hides from list
	if err := wsRepo.SoftDelete(ctx, org.ID, ws.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, err := wsRepo.Get(ctx, org.ID, ws.ID); !errors.Is(err, tenancy.ErrWorkspaceNotFound) {
		t.Fatalf("want ErrWorkspaceNotFound after soft-delete; got %v", err)
	}
	list, _ := wsRepo.List(ctx, org.ID)
	for _, w := range list {
		if w.ID == ws.ID {
			t.Fatalf("soft-deleted workspace still in List: %+v", w)
		}
	}

	// Restore
	if err := wsRepo.Restore(ctx, org.ID, ws.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := wsRepo.Get(ctx, org.ID, ws.ID); err != nil {
		t.Fatalf("Get after Restore: %v", err)
	}
}

// TestWorkspaceRepo_CrossOrgIsolation asserts that a workspace created
// in org A is invisible to org B. This is the core SC-001 guarantee
// (no cross-tenant reads).
func TestWorkspaceRepo_CrossOrgIsolation(t *testing.T) {
	db := newOrgsDB(t)
	ctx := context.Background()
	if err := configstore.RegisterEnterpriseMigrations(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	orgs := tenancy.NewOrgRepo(db)
	orgA, err := orgs.GetDefault(ctx)
	if err != nil {
		t.Fatalf("orgA: %v", err)
	}

	// Create org B for cross-org isolation. In single-org mode this is
	// outside the documented contract (CreateMultiOrg is cloud-only),
	// but the isolation semantics apply regardless of mode.
	orgB, err := orgs.CreateMultiOrg(ctx, "OrgB")
	if err != nil {
		t.Fatalf("orgB: %v", err)
	}

	wsRepo := tenancy.NewWorkspaceRepo(db)

	wsA, err := wsRepo.Create(ctx, orgA.ID, "A-ws", "a-ws", "")
	if err != nil {
		t.Fatalf("create A: %v", err)
	}

	// Try to read A's workspace from B's scope.
	if _, err := wsRepo.Get(ctx, orgB.ID, wsA.ID); !errors.Is(err, tenancy.ErrWorkspaceNotFound) {
		t.Fatalf("cross-org read leaked: err=%v", err)
	}

	// B's list must NOT include A's workspace.
	bList, _ := wsRepo.List(ctx, orgB.ID)
	for _, w := range bList {
		if w.ID == wsA.ID {
			t.Fatalf("orgB list leaked orgA workspace: %+v", w)
		}
	}
}
