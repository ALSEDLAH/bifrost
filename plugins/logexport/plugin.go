// Package logexport forwards completed request traces to configured
// downstream log-export destinations. Spec 008 shipped the config
// storage (ent_log_export_connectors); this plugin is the runtime
// consumer. v1 ships the Datadog adapter — BigQuery and the other
// destinations listed in the public docs are tracked as follow-up
// specs (017+).
package logexport

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

// PluginName is the stable plugin name used by Config.ReloadPlugin.
const PluginName = "log-export"

// cacheTTL is how long we keep the connector slice before a refresh.
// Admin CRUD handlers call Invalidate() to pre-empt the TTL on edits.
const cacheTTL = 30 * time.Second

// Plugin implements schemas.ObservabilityPlugin.
type Plugin struct {
	store  configstore.ConfigStore
	logger schemas.Logger

	cacheMu      sync.Mutex
	cacheLoaded  atomic.Int64 // UnixNano of last load; 0 = never loaded
	cacheFresh   atomic.Value // *[]tables_enterprise.TableLogExportConnector
	bigQueryWarn sync.Map     // connector-ID → struct{} dedup for the "bigquery deferred" info log
}

// Init constructs the plugin. Returns nil + error when the configstore
// handle is missing — callers should treat that as non-fatal and skip.
func Init(_ context.Context, store configstore.ConfigStore, logger schemas.Logger) (*Plugin, error) {
	if store == nil {
		return nil, fmt.Errorf("logexport: nil configstore")
	}
	p := &Plugin{store: store, logger: logger}
	// Seed the cache so the first Inject after startup doesn't hit
	// the DB on the hot path.
	_, _ = p.load(context.Background())
	return p, nil
}

// GetName satisfies BasePlugin.
func (p *Plugin) GetName() string { return PluginName }

// Cleanup is a no-op — nothing dynamic to tear down.
func (p *Plugin) Cleanup() error { return nil }

// Invalidate drops the cache so the next Inject re-reads the
// connector list. Admin handlers call this after every successful
// create/update/delete so edits propagate in <1s rather than waiting
// the 30s TTL.
func (p *Plugin) Invalidate() {
	if p == nil {
		return
	}
	p.cacheLoaded.Store(0)
}

// Inject is called async by core after a response has been written.
// Iterates every enabled connector + runs the matching adapter. No
// retries, 2s timeout per call — fire-and-forget.
func (p *Plugin) Inject(ctx context.Context, trace *schemas.Trace) error {
	if p == nil || trace == nil {
		return nil
	}
	connectors, err := p.listEnabled(ctx)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn(fmt.Sprintf("logexport: connector fetch failed: %v", err))
		}
		return nil
	}
	if len(connectors) == 0 {
		return nil
	}

	// Copy the trace fields we need before spawning goroutines — the
	// caller recycles *Trace to a pool once Inject returns.
	rec := extractLogRecord(trace)

	for i := range connectors {
		c := connectors[i]
		switch c.Type {
		case "datadog":
			go p.sendDatadog(c, rec)
		case "bigquery":
			p.warnBigQueryOnce(c)
		default:
			if p.logger != nil {
				p.logger.Warn(fmt.Sprintf("logexport: unknown connector type %q (id=%s)", c.Type, c.ID))
			}
		}
	}
	return nil
}

// listEnabled returns the cached slice, refreshing when the TTL has
// elapsed or Invalidate() has forced a reload.
func (p *Plugin) listEnabled(ctx context.Context) ([]tables_enterprise.TableLogExportConnector, error) {
	last := p.cacheLoaded.Load()
	fresh := time.Now().UnixNano() - last
	if last != 0 && time.Duration(fresh) < cacheTTL {
		if v, ok := p.cacheFresh.Load().(*[]tables_enterprise.TableLogExportConnector); ok && v != nil {
			return *v, nil
		}
	}
	return p.load(ctx)
}

func (p *Plugin) load(ctx context.Context) ([]tables_enterprise.TableLogExportConnector, error) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	// Re-check after acquiring the mutex — a sibling goroutine may
	// have just refreshed.
	last := p.cacheLoaded.Load()
	fresh := time.Now().UnixNano() - last
	if last != 0 && time.Duration(fresh) < cacheTTL {
		if v, ok := p.cacheFresh.Load().(*[]tables_enterprise.TableLogExportConnector); ok && v != nil {
			return *v, nil
		}
	}
	rows, err := p.store.ListLogExportConnectors(ctx, "")
	if err != nil {
		return nil, err
	}
	enabled := rows[:0]
	for _, r := range rows {
		if r.Enabled {
			enabled = append(enabled, r)
		}
	}
	out := make([]tables_enterprise.TableLogExportConnector, len(enabled))
	copy(out, enabled)
	p.cacheFresh.Store(&out)
	p.cacheLoaded.Store(time.Now().UnixNano())
	return out, nil
}

func (p *Plugin) warnBigQueryOnce(c tables_enterprise.TableLogExportConnector) {
	if _, already := p.bigQueryWarn.LoadOrStore(c.ID, struct{}{}); already {
		return
	}
	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("logexport: connector %s (%s) type=bigquery is configured; BigQuery forwarding is scheduled for spec 018", c.Name, c.ID))
	}
}

// logRecord is the wire-ready shape we hand to each adapter. Stable
// field names so adapters can do their own mapping.
type logRecord struct {
	Timestamp  time.Time      `json:"timestamp"`
	RequestID  string         `json:"request_id"`
	TraceID    string         `json:"trace_id"`
	Message    string         `json:"message"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// extractLogRecord flattens the parts of the trace we forward. We
// deliberately copy scalar fields so the adapter goroutine is safe
// after the caller recycles *Trace.
func extractLogRecord(t *schemas.Trace) logRecord {
	rec := logRecord{
		Timestamp: t.EndTime,
		RequestID: t.GetRequestID(),
		TraceID:   t.TraceID,
	}
	if t.Attributes != nil {
		rec.Attributes = make(map[string]any, len(t.Attributes))
		for k, v := range t.Attributes {
			rec.Attributes[k] = v
		}
	}
	// Message is a compact human-readable summary that Datadog can
	// index as full-text. Keep it short.
	if rec.Attributes != nil {
		if model, _ := rec.Attributes["model"].(string); model != "" {
			rec.Message = "bifrost inference: " + model
		}
	}
	if rec.Message == "" {
		rec.Message = "bifrost inference"
	}
	return rec
}

// marshalForAdapter is exposed for tests so they can assert payload
// bytes without reconstructing the serializer.
func (r logRecord) marshalJSON() ([]byte, error) { return json.Marshal(r) }
