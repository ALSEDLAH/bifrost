// Prompt deployment CRUD (spec 011).

package configstore

import (
	"context"
	"fmt"
	"time"

	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

func (s *RDBConfigStore) ListPromptDeployments(ctx context.Context, promptID string) ([]tables_enterprise.TablePromptDeployment, error) {
	var rows []tables_enterprise.TablePromptDeployment
	if err := s.db.WithContext(ctx).Where("prompt_id = ?", promptID).Order("label ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list prompt deployments for %s: %w", promptID, err)
	}
	return rows, nil
}

func (s *RDBConfigStore) UpsertPromptDeployment(ctx context.Context, d *tables_enterprise.TablePromptDeployment) error {
	d.PromotedAt = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(d).Error; err != nil {
		return fmt.Errorf("upsert prompt deployment: %w", err)
	}
	return nil
}

func (s *RDBConfigStore) DeletePromptDeployment(ctx context.Context, promptID, label string) error {
	if err := s.db.WithContext(ctx).
		Where("prompt_id = ? AND label = ?", promptID, label).
		Delete(&tables_enterprise.TablePromptDeployment{}).Error; err != nil {
		return fmt.Errorf("delete prompt deployment %s/%s: %w", promptID, label, err)
	}
	return nil
}
