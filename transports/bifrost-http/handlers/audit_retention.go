// Audit log retention handler (spec 027). Reads/writes the singleton
// retention config + runs prune (manual or background) + protects
// the spec 015 HMAC chain from being silently broken.

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/maximhq/bifrost/plugins/audit"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

// pruneTickInterval is how often the background goroutine wakes; it
// only ACTUALLY prunes if 24h have elapsed since last_pruned_at.
const pruneTickInterval = 6 * time.Hour
const pruneMinSpacing = 24 * time.Hour

type AuditRetentionHandler struct {
	db     *gorm.DB
	logger schemas.Logger

	// configstore stays alongside logstore so the handler can keep
	// operating even if the logstore moves to a separate DB later.
	configDB *gorm.DB

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewAuditRetentionHandler wires the handler. configDB holds the
// retention config row; logDB holds the audit entries.
func NewAuditRetentionHandler(configDB, logDB *gorm.DB, logger schemas.Logger) *AuditRetentionHandler {
	h := &AuditRetentionHandler{
		db: logDB, configDB: configDB, logger: logger,
		stopCh: make(chan struct{}),
	}
	h.wg.Add(1)
	go h.backgroundLoop()
	return h
}

// Close stops the background tick. Safe to call multiple times.
func (h *AuditRetentionHandler) Close() {
	select {
	case <-h.stopCh:
		// already closed
	default:
		close(h.stopCh)
	}
	h.wg.Wait()
}

func (h *AuditRetentionHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/audit-logs/retention", lib.ChainMiddlewares(h.getConfig, middlewares...))
	r.PUT("/api/audit-logs/retention", lib.ChainMiddlewares(h.putConfig, middlewares...))
	r.POST("/api/audit-logs/retention/prune", lib.ChainMiddlewares(h.runPrune, middlewares...))
}

// loadConfig returns the singleton row, creating an empty one in
// memory (NOT persisted) when missing. That way GET on a fresh
// install still returns sensible defaults.
func (h *AuditRetentionHandler) loadConfig() (*tables_enterprise.TableAuditRetentionConfig, error) {
	var cfg tables_enterprise.TableAuditRetentionConfig
	err := h.configDB.Where("id = ?", tables_enterprise.AuditRetentionSingletonID).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &tables_enterprise.TableAuditRetentionConfig{
			ID:            tables_enterprise.AuditRetentionSingletonID,
			Enabled:       false,
			RetentionDays: 90,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (h *AuditRetentionHandler) getConfig(ctx *fasthttp.RequestCtx) {
	cfg, err := h.loadConfig()
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}
	SendJSON(ctx, cfg)
}

type retentionPutBody struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retention_days"`
}

func (h *AuditRetentionHandler) putConfig(ctx *fasthttp.RequestCtx) {
	var req retentionPutBody
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": "invalid body"}, fasthttp.StatusBadRequest)
		return
	}
	if req.RetentionDays < 1 {
		SendJSONWithStatus(ctx, map[string]any{"error": "retention_days must be >= 1"}, fasthttp.StatusBadRequest)
		return
	}
	existing, err := h.loadConfig()
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}
	row := tables_enterprise.TableAuditRetentionConfig{
		ID:            tables_enterprise.AuditRetentionSingletonID,
		Enabled:       req.Enabled,
		RetentionDays: req.RetentionDays,
		LastPrunedAt:  existing.LastPrunedAt,
		UpdatedAt:     time.Now().UTC(),
	}
	if err := h.configDB.Save(&row).Error; err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}
	emitRetentionAudit("audit.retention.updated", map[string]any{
		"enabled": row.Enabled, "retention_days": row.RetentionDays,
	})
	SendJSON(ctx, map[string]any{"ok": true})
}

type prunePostBody struct {
	Force bool `json:"force"`
}

type pruneResult struct {
	Deleted int64  `json:"deleted"`
	Refused string `json:"refused,omitempty"`
}

func (h *AuditRetentionHandler) runPrune(ctx *fasthttp.RequestCtx) {
	var req prunePostBody
	if len(ctx.PostBody()) > 0 {
		_ = json.Unmarshal(ctx.PostBody(), &req)
	}
	cfg, err := h.loadConfig()
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}
	if !cfg.Enabled {
		SendJSONWithStatus(ctx, map[string]any{"error": "retention disabled"}, fasthttp.StatusConflict)
		return
	}
	res, err := h.prune(cfg.RetentionDays, req.Force)
	if err != nil {
		SendJSONWithStatus(ctx, map[string]any{"error": err.Error()}, fasthttp.StatusInternalServerError)
		return
	}
	if res.Refused == "" {
		// stamp last_pruned_at on success
		now := time.Now().UTC()
		_ = h.configDB.Model(&tables_enterprise.TableAuditRetentionConfig{}).
			Where("id = ?", tables_enterprise.AuditRetentionSingletonID).
			Update("last_pruned_at", &now).Error
	}
	emitRetentionAudit("audit.retention.pruned", map[string]any{
		"deleted": res.Deleted, "refused": res.Refused, "forced": req.Force,
	})
	SendJSON(ctx, res)
}

// prune deletes audit entries older than retention_days. When force=false
// and the prune would break the HMAC chain, returns refused with a reason
// and zero deletions.
func (h *AuditRetentionHandler) prune(retentionDays int, force bool) (pruneResult, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	if !force {
		// Find the oldest survivor. If it has a non-empty PrevHMAC, the
		// row matching that HMAC must still exist.
		var survivor logstore.TableAuditEntry
		err := h.db.Where("created_at >= ?", cutoff).Order("created_at ASC").First(&survivor).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return pruneResult{}, err
		}
		if err == nil && survivor.PrevHMAC != "" {
			var ancestorCount int64
			h.db.Model(&logstore.TableAuditEntry{}).
				Where("hmac = ? AND created_at < ?", survivor.PrevHMAC, cutoff).
				Count(&ancestorCount)
			if ancestorCount > 0 {
				return pruneResult{
					Refused: "hmac chain would break — pass force=true to override",
				}, nil
			}
		}
	}

	res := h.db.Where("created_at < ?", cutoff).Delete(&logstore.TableAuditEntry{})
	if res.Error != nil {
		return pruneResult{}, res.Error
	}
	return pruneResult{Deleted: res.RowsAffected}, nil
}

// backgroundLoop ticks every 6h. Runs the prune when retention is
// enabled AND last_pruned_at is null OR older than 24h.
func (h *AuditRetentionHandler) backgroundLoop() {
	defer h.wg.Done()
	t := time.NewTicker(pruneTickInterval)
	defer t.Stop()
	for {
		select {
		case <-h.stopCh:
			return
		case <-t.C:
			h.maybePrune()
		}
	}
}

func (h *AuditRetentionHandler) maybePrune() {
	cfg, err := h.loadConfig()
	if err != nil || !cfg.Enabled {
		return
	}
	if cfg.LastPrunedAt != nil && time.Since(*cfg.LastPrunedAt) < pruneMinSpacing {
		return
	}
	res, err := h.prune(cfg.RetentionDays, false)
	if err != nil {
		h.logger.Error("audit retention background prune: %v", err)
		return
	}
	if res.Refused == "" {
		now := time.Now().UTC()
		_ = h.configDB.Model(&tables_enterprise.TableAuditRetentionConfig{}).
			Where("id = ?", tables_enterprise.AuditRetentionSingletonID).
			Update("last_pruned_at", &now).Error
		emitRetentionAudit("audit.retention.pruned", map[string]any{
			"deleted": res.Deleted, "background": true,
		})
	}
}

func emitRetentionAudit(action string, after map[string]any) {
	bctx := schemas.NewBifrostContext(context.Background(), time.Time{})
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, "system")
	bctx.SetValue(tenancy.BifrostContextKeyUserID, "system")
	bctx.SetValue(tenancy.BifrostContextKeyResolvedVia, tenancy.Resolver("retention"))
	_ = audit.Emit(context.Background(), bctx, audit.Entry{
		Action: action, ResourceType: "audit_retention",
		ResourceID: tables_enterprise.AuditRetentionSingletonID,
		Outcome:    "allowed", After: after,
	})
}
