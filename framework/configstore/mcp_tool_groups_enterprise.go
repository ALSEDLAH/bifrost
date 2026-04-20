// MCP tool-group CRUD for the enterprise admin UI (spec 005).
// Sibling file to the upstream configstore methods in rdb.go.

package configstore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"gorm.io/gorm"
)

func (s *RDBConfigStore) ListMCPToolGroups(ctx context.Context) ([]tables_enterprise.TableMCPToolGroup, error) {
	var groups []tables_enterprise.TableMCPToolGroup
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("list mcp tool groups: %w", err)
	}
	return groups, nil
}

func (s *RDBConfigStore) GetMCPToolGroupByID(ctx context.Context, id string) (*tables_enterprise.TableMCPToolGroup, error) {
	var g tables_enterprise.TableMCPToolGroup
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&g).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get mcp tool group %s: %w", id, err)
	}
	return &g, nil
}

func (s *RDBConfigStore) CreateMCPToolGroup(ctx context.Context, g *tables_enterprise.TableMCPToolGroup) error {
	if g.ID == "" {
		g.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	g.UpdatedAt = now
	if g.Tools == "" {
		g.Tools = "[]"
	}
	if err := s.db.WithContext(ctx).Create(g).Error; err != nil {
		return fmt.Errorf("create mcp tool group: %w", err)
	}
	return nil
}

func (s *RDBConfigStore) UpdateMCPToolGroup(ctx context.Context, g *tables_enterprise.TableMCPToolGroup) error {
	g.UpdatedAt = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(g).Error; err != nil {
		return fmt.Errorf("update mcp tool group %s: %w", g.ID, err)
	}
	return nil
}

func (s *RDBConfigStore) DeleteMCPToolGroup(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&tables_enterprise.TableMCPToolGroup{}).Error; err != nil {
		return fmt.Errorf("delete mcp tool group %s: %w", id, err)
	}
	return nil
}
