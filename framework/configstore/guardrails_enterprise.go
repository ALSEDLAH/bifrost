// Guardrails CRUD — providers + rules (spec 010).
// Sibling file to upstream configstore methods.

package configstore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"gorm.io/gorm"
)

// ---- Providers --------------------------------------------------------

func (s *RDBConfigStore) ListGuardrailProviders(ctx context.Context) ([]tables_enterprise.TableGuardrailProvider, error) {
	var rows []tables_enterprise.TableGuardrailProvider
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list guardrail providers: %w", err)
	}
	return rows, nil
}

func (s *RDBConfigStore) GetGuardrailProviderByID(ctx context.Context, id string) (*tables_enterprise.TableGuardrailProvider, error) {
	var p tables_enterprise.TableGuardrailProvider
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get guardrail provider %s: %w", id, err)
	}
	return &p, nil
}

func (s *RDBConfigStore) CreateGuardrailProvider(ctx context.Context, p *tables_enterprise.TableGuardrailProvider) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if err := s.db.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("create guardrail provider: %w", err)
	}
	return nil
}

func (s *RDBConfigStore) UpdateGuardrailProvider(ctx context.Context, p *tables_enterprise.TableGuardrailProvider) error {
	p.UpdatedAt = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(p).Error; err != nil {
		return fmt.Errorf("update guardrail provider %s: %w", p.ID, err)
	}
	return nil
}

func (s *RDBConfigStore) DeleteGuardrailProvider(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&tables_enterprise.TableGuardrailProvider{}).Error; err != nil {
		return fmt.Errorf("delete guardrail provider %s: %w", id, err)
	}
	return nil
}

// ---- Rules ------------------------------------------------------------

func (s *RDBConfigStore) ListGuardrailRules(ctx context.Context) ([]tables_enterprise.TableGuardrailRule, error) {
	var rows []tables_enterprise.TableGuardrailRule
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list guardrail rules: %w", err)
	}
	return rows, nil
}

func (s *RDBConfigStore) GetGuardrailRuleByID(ctx context.Context, id string) (*tables_enterprise.TableGuardrailRule, error) {
	var r tables_enterprise.TableGuardrailRule
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&r).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get guardrail rule %s: %w", id, err)
	}
	return &r, nil
}

func (s *RDBConfigStore) CreateGuardrailRule(ctx context.Context, r *tables_enterprise.TableGuardrailRule) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	if err := s.db.WithContext(ctx).Create(r).Error; err != nil {
		return fmt.Errorf("create guardrail rule: %w", err)
	}
	return nil
}

func (s *RDBConfigStore) UpdateGuardrailRule(ctx context.Context, r *tables_enterprise.TableGuardrailRule) error {
	r.UpdatedAt = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(r).Error; err != nil {
		return fmt.Errorf("update guardrail rule %s: %w", r.ID, err)
	}
	return nil
}

func (s *RDBConfigStore) DeleteGuardrailRule(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&tables_enterprise.TableGuardrailRule{}).Error; err != nil {
		return fmt.Errorf("delete guardrail rule %s: %w", id, err)
	}
	return nil
}

func (s *RDBConfigStore) CountGuardrailRulesByProvider(ctx context.Context, providerID string) (int64, error) {
	var n int64
	if err := s.db.WithContext(ctx).Model(&tables_enterprise.TableGuardrailRule{}).Where("provider_id = ?", providerID).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count rules for provider %s: %w", providerID, err)
	}
	return n, nil
}
