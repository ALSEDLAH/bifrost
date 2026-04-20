// SCIM singleton config CRUD (spec 009).

package configstore

import (
	"context"
	"fmt"
	"time"

	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"gorm.io/gorm"
)

func (s *RDBConfigStore) GetSCIMConfig(ctx context.Context) (*tables_enterprise.TableSCIMConfig, error) {
	var c tables_enterprise.TableSCIMConfig
	if err := s.db.WithContext(ctx).Where("id = ?", tables_enterprise.SCIMConfigRowID).First(&c).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get scim_config: %w", err)
	}
	return &c, nil
}

func (s *RDBConfigStore) UpsertSCIMConfig(ctx context.Context, c *tables_enterprise.TableSCIMConfig) error {
	c.ID = tables_enterprise.SCIMConfigRowID
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if err := s.db.WithContext(ctx).Save(c).Error; err != nil {
		return fmt.Errorf("upsert scim_config: %w", err)
	}
	return nil
}
