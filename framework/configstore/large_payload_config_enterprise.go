// Large payload singleton config for the enterprise settings UI (spec 006).
// Sibling file to upstream configstore methods in rdb.go.

package configstore

import (
	"context"
	"fmt"
	"time"

	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"gorm.io/gorm"
)

func (s *RDBConfigStore) GetLargePayloadConfig(ctx context.Context) (*tables_enterprise.TableLargePayloadConfig, error) {
	var cfg tables_enterprise.TableLargePayloadConfig
	if err := s.db.WithContext(ctx).Where("id = ?", tables_enterprise.LargePayloadConfigRowID).First(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get large_payload_config: %w", err)
	}
	return &cfg, nil
}

func (s *RDBConfigStore) UpsertLargePayloadConfig(ctx context.Context, c *tables_enterprise.TableLargePayloadConfig) error {
	c.ID = tables_enterprise.LargePayloadConfigRowID
	c.UpdatedAt = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(c).Error; err != nil {
		return fmt.Errorf("upsert large_payload_config: %w", err)
	}
	return nil
}
