// Log export connector CRUD for the Datadog / BigQuery admin UI (spec 008).
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

func (s *RDBConfigStore) ListLogExportConnectors(ctx context.Context, typeFilter string) ([]tables_enterprise.TableLogExportConnector, error) {
	var rows []tables_enterprise.TableLogExportConnector
	q := s.db.WithContext(ctx)
	if typeFilter != "" {
		q = q.Where("type = ?", typeFilter)
	}
	if err := q.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list log_export_connectors: %w", err)
	}
	return rows, nil
}

func (s *RDBConfigStore) GetLogExportConnectorByID(ctx context.Context, id string) (*tables_enterprise.TableLogExportConnector, error) {
	var c tables_enterprise.TableLogExportConnector
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get log_export_connector %s: %w", id, err)
	}
	return &c, nil
}

func (s *RDBConfigStore) CreateLogExportConnector(ctx context.Context, c *tables_enterprise.TableLogExportConnector) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if err := s.db.WithContext(ctx).Create(c).Error; err != nil {
		return fmt.Errorf("create log_export_connector: %w", err)
	}
	return nil
}

func (s *RDBConfigStore) UpdateLogExportConnector(ctx context.Context, c *tables_enterprise.TableLogExportConnector) error {
	c.UpdatedAt = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(c).Error; err != nil {
		return fmt.Errorf("update log_export_connector %s: %w", c.ID, err)
	}
	return nil
}

func (s *RDBConfigStore) DeleteLogExportConnector(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&tables_enterprise.TableLogExportConnector{}).Error; err != nil {
		return fmt.Errorf("delete log_export_connector %s: %w", id, err)
	}
	return nil
}
