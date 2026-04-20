// Package audit is the enterprise audit-log sink.
//
// Constitution Principle VI — every enterprise plugin emits audit
// entries via this plugin's Emit helper. Records are persisted to the
// ent_audit_entries table (defined in framework/logstore/tables_enterprise.go).
//
// The plugin implements ObservabilityPlugin so per-request lifecycle
// events are also captured asynchronously after the response is on the
// wire (no hot-path latency cost).
//
// Direct admin/governance actions (create/update/delete of keys, roles,
// budgets, guardrails, etc.) do NOT go through ObservabilityPlugin —
// they call audit.Emit() directly from their handler / plugin Init.
package audit

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/framework/tenancy"
	"gorm.io/gorm"
)

// PluginName is the registered plugin identifier.
const PluginName = "audit"

// Plugin is the audit sink. Holds a DB handle for synchronous writes
// (admin/governance actions) and a buffered channel for async lifecycle
// events fed by ObservabilityPlugin.Inject.
type Plugin struct {
	db        *gorm.DB
	logger    schemas.Logger
	asyncCh   chan logstore.TableAuditEntry
	asyncWG   sync.WaitGroup
	cancelFn  context.CancelFunc
	startedAt time.Time

	// chain holds the HMAC chain state (spec 015). Zero value is fine —
	// Enabled() reports false when no key has been loaded.
	chain chainState
}

// Config holds optional knobs read from config.enterprise.audit.
type Config struct {
	// AsyncBufferSize is the bounded channel capacity for async lifecycle
	// audits. Default 1000. When full, oldest entries are dropped and the
	// audit_dropped_total counter increments.
	AsyncBufferSize int `json:"async_buffer_size"`
}

// droppedTotal is incremented when the async channel is full at Inject
// time. Wired to a Prometheus counter by the telemetry plugin via the
// observability-completeness manifest.
var droppedTotal atomic.Uint64

// DroppedTotal returns the current count for telemetry consumption.
func DroppedTotal() uint64 { return droppedTotal.Load() }

// Init constructs the audit plugin and starts its async worker.
//
// db is the *gorm.DB that holds `ent_audit_entries`. Per spec
// clarification 2026-04-20, audit entries live in the **configstore**
// (co-located with `ent_roles`, `ent_users`, governance tables they
// reference), not the logstore. The `logstore.TableAuditEntry` type
// is authored in the logstore package for historical reasons — the
// package name no longer implies the physical database.
func Init(ctx context.Context, db *gorm.DB, logger schemas.Logger, cfg Config) (*Plugin, error) {
	if db == nil {
		return nil, fmt.Errorf("audit: nil db")
	}
	bufSize := cfg.AsyncBufferSize
	if bufSize <= 0 {
		bufSize = 1000
	}

	pluginCtx, cancel := context.WithCancel(ctx)
	p := &Plugin{
		db:        db,
		logger:    logger,
		asyncCh:   make(chan logstore.TableAuditEntry, bufSize),
		cancelFn:  cancel,
		startedAt: time.Now().UTC(),
	}
	// Load HMAC key (spec 015). Malformed key is fatal; missing key is
	// fine — chain stays disabled.
	key, keyErr := loadHMACKey()
	if keyErr != nil {
		return nil, keyErr
	}
	p.chain.key = key
	if logger != nil && len(key) > 0 {
		logger.Info("audit: HMAC chain enabled (spec 015)")
	}

	p.asyncWG.Add(1)
	go p.runAsyncWorker(pluginCtx)

	// Register this plugin instance as the default Emit sink so other
	// plugins can call audit.Emit() without a handle to *Plugin.
	setDefaultSink(p)
	return p, nil
}

// GetName satisfies BasePlugin.
func (p *Plugin) GetName() string { return PluginName }

// Cleanup drains the async channel and stops the worker.
func (p *Plugin) Cleanup() error {
	p.cancelFn()
	close(p.asyncCh)
	p.asyncWG.Wait()
	clearDefaultSink()
	return nil
}

// Inject implements ObservabilityPlugin. Called asynchronously by core
// after the HTTP response is flushed. We extract tenant context from
// the trace's attributes (when present) and record a request-completion
// audit entry.
//
// Inject MUST not block; it pushes to the bounded channel and returns.
func (p *Plugin) Inject(ctx context.Context, trace *schemas.Trace) error {
	if trace == nil {
		return nil
	}
	entry := logstore.TableAuditEntry{
		ID:           uuid.NewString(),
		ActorType:    "system",
		Action:       "request.completed",
		ResourceType: "request",
		ResourceID:   trace.RequestID,
		Outcome:      "allowed",
		RequestID:    trace.RequestID,
		CreatedAt:    time.Now().UTC(),
	}
	// Best-effort tenant attribution from the Go context if available.
	// Missing tenant => synthetic-default audit (still recorded; analyst
	// sees ResolvedVia=default).
	if tc, err := tenancy.FromGoContext(ctx); err == nil {
		entry.OrganizationID = tc.OrganizationID
		entry.WorkspaceID = tc.WorkspaceID
		if tc.UserID != "" {
			entry.ActorType = "user"
			entry.ActorID = tc.UserID
		}
	}
	select {
	case p.asyncCh <- entry:
	default:
		droppedTotal.Add(1)
	}
	return nil
}

// runAsyncWorker drains asyncCh into the DB. One worker is sufficient
// at expected enterprise scale; the bounded channel + drop-newest
// behavior keeps us off the request hot path.
func (p *Plugin) runAsyncWorker(ctx context.Context) {
	defer p.asyncWG.Done()
	batch := make([]logstore.TableAuditEntry, 0, 64)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		// Spec 015: stamp the HMAC chain on each row in the batch
		// under the chain mutex so concurrent Emit + async flush do
		// not race for the same predecessor.
		if p.chain.Enabled() {
			p.seedChainFromDB()
			p.chain.mu.Lock()
			for i := range batch {
				batch[i].HMAC, batch[i].PrevHMAC = p.chain.computeHMAC(batch[i].CanonicalBytes())
			}
			p.chain.mu.Unlock()
		}
		if err := p.db.WithContext(ctx).Create(&batch).Error; err != nil {
			if p.logger != nil {
				p.logger.Warn(fmt.Sprintf("audit: async batch flush failed: %v", err))
			}
		}
		batch = batch[:0]
	}
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case e, ok := <-p.asyncCh:
			if !ok {
				flush()
				return
			}
			batch = append(batch, e)
			if len(batch) >= 64 {
				flush()
			}
		case <-tick.C:
			flush()
		}
	}
}
