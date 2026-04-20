// Alert channel CRUD for the enterprise governance alert dispatcher
// (spec 004). Sibling to the upstream configstore methods in rdb.go.

package configstore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"gorm.io/gorm"
)

func (s *RDBConfigStore) ListAlertChannels(ctx context.Context) ([]tables_enterprise.TableAlertChannel, error) {
	var channels []tables_enterprise.TableAlertChannel
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("list alert channels: %w", err)
	}
	return channels, nil
}

func (s *RDBConfigStore) GetAlertChannelByID(ctx context.Context, id string) (*tables_enterprise.TableAlertChannel, error) {
	var ch tables_enterprise.TableAlertChannel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&ch).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get alert channel %s: %w", id, err)
	}
	return &ch, nil
}

func (s *RDBConfigStore) CreateAlertChannel(ctx context.Context, ch *tables_enterprise.TableAlertChannel) error {
	if ch.ID == "" {
		ch.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if ch.CreatedAt.IsZero() {
		ch.CreatedAt = now
	}
	ch.UpdatedAt = now
	if err := s.db.WithContext(ctx).Create(ch).Error; err != nil {
		return fmt.Errorf("create alert channel: %w", err)
	}
	return nil
}

func (s *RDBConfigStore) UpdateAlertChannel(ctx context.Context, ch *tables_enterprise.TableAlertChannel) error {
	ch.UpdatedAt = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(ch).Error; err != nil {
		return fmt.Errorf("update alert channel %s: %w", ch.ID, err)
	}
	return nil
}

func (s *RDBConfigStore) DeleteAlertChannel(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&tables_enterprise.TableAlertChannel{}).Error; err != nil {
		return fmt.Errorf("delete alert channel %s: %w", id, err)
	}
	return nil
}
