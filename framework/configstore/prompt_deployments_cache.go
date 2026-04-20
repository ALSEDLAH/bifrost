// Cached production-deployment lookup used by the prompts plugin
// (spec 014). Keyed by prompt_id → version_id. TTL=30s with
// explicit Invalidate() from the admin PUT/DELETE handlers so a
// rollback takes effect immediately.
//
// Lives in configstore (not the prompts plugin) to avoid pulling
// configstore into the plugins/prompts go.mod.

package configstore

import (
	"context"
	"sync"
	"time"

	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

// ProductionDeploymentCache keeps a prompt_id → version_id map
// refreshed from ent_prompt_deployments where label='production'.
type ProductionDeploymentCache struct {
	store   ConfigStore
	mu      sync.RWMutex
	expires time.Time
	lookup  map[string]uint
}

const productionDeploymentCacheTTL = 30 * time.Second

// NewProductionDeploymentCache builds a cache bound to the store.
// The first Lookup triggers a synchronous refresh.
func NewProductionDeploymentCache(store ConfigStore) *ProductionDeploymentCache {
	return &ProductionDeploymentCache{
		store:  store,
		lookup: make(map[string]uint),
	}
}

// Lookup returns the production version id for the prompt, or 0.
// Refreshes the table scan when the TTL has elapsed.
func (c *ProductionDeploymentCache) Lookup(promptID string) uint {
	if c == nil || c.store == nil {
		return 0
	}
	c.mu.RLock()
	fresh := time.Now().Before(c.expires)
	if fresh {
		id := c.lookup[promptID]
		c.mu.RUnlock()
		return id
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.expires) {
		return c.lookup[promptID]
	}
	rows, err := c.listAllProduction(context.Background())
	if err != nil {
		// Keep the stale map; return whatever we had.
		return c.lookup[promptID]
	}
	next := make(map[string]uint, len(rows))
	for _, r := range rows {
		if r.Label == "production" {
			next[r.PromptID] = r.VersionID
		}
	}
	c.lookup = next
	c.expires = time.Now().Add(productionDeploymentCacheTTL)
	return c.lookup[promptID]
}

// Invalidate drops the cache so the next Lookup triggers a refresh.
// Called by the admin PUT / DELETE /api/prompts/:id/deployments/:label
// handlers so rollbacks are visible in <1 second (spec 014 FR-005).
func (c *ProductionDeploymentCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.expires = time.Time{}
	c.mu.Unlock()
}

// listAllProduction is a scan across every prompt's production row.
// The table is small (one row per labeled prompt), so this is fine
// at our TTL cadence.
func (c *ProductionDeploymentCache) listAllProduction(ctx context.Context) ([]tables_enterprise.TablePromptDeployment, error) {
	// We don't currently have a "list all by label" store method —
	// leverage the existing ListPromptDeployments per prompt by first
	// scanning the underlying DB via the exposed gorm handle.
	rdb, ok := c.store.(*RDBConfigStore)
	if !ok {
		return nil, nil
	}
	var rows []tables_enterprise.TablePromptDeployment
	if err := rdb.db.WithContext(ctx).Where("label = ?", "production").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
