// Datadog Logs adapter (spec 017 T002/T003).
//
// POSTs a single log record to Datadog's Logs intake API per enabled
// connector. Fire-and-forget; errors log at Warn. The Datadog body is
// a JSON array so we always send [rec] even for a single record.

package logexport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

// datadogClient is shared across connectors to pool HTTP connections.
var datadogClient = &http.Client{Timeout: 2 * time.Second}

// datadogConfig mirrors the Config JSON shape used by the UI
// (ui/lib/types/logExportConnectors.ts DatadogConnectorConfig).
type datadogConfig struct {
	APIKey string            `json:"api_key"`
	Site   string            `json:"site,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// sendDatadog POSTs rec to the connector's Datadog intake. Called in
// its own goroutine from Inject() so network slowness doesn't back
// up the observability pipeline.
func (p *Plugin) sendDatadog(conn tables_enterprise.TableLogExportConnector, rec logRecord) {
	defer func() {
		if r := recover(); r != nil && p.logger != nil {
			p.logger.Error(fmt.Sprintf("logexport: panic in sendDatadog(%s): %v", conn.ID, r))
		}
	}()

	var cfg datadogConfig
	if err := json.Unmarshal([]byte(conn.Config), &cfg); err != nil {
		p.warn(conn, fmt.Errorf("parse config: %w", err))
		return
	}
	if cfg.APIKey == "" {
		p.warn(conn, fmt.Errorf("api_key is empty"))
		return
	}
	site := cfg.Site
	if site == "" {
		site = "datadoghq.com"
	}

	// Datadog wants a specific envelope on the intake URL. Merge our
	// record attributes into the top-level so their parser can index.
	body := map[string]any{
		"ddsource":   "bifrost",
		"service":    "bifrost",
		"message":    rec.Message,
		"request_id": rec.RequestID,
		"trace_id":   rec.TraceID,
		"ddtags":     formatDDTags(cfg.Tags),
	}
	if !rec.Timestamp.IsZero() {
		// Datadog timestamps are ms-since-epoch.
		body["timestamp"] = rec.Timestamp.UnixMilli()
	}
	for k, v := range rec.Attributes {
		body[k] = v
	}

	raw, err := json.Marshal([]any{body})
	if err != nil {
		p.warn(conn, fmt.Errorf("marshal: %w", err))
		return
	}

	url := fmt.Sprintf("https://http-intake.logs.%s/api/v2/logs", site)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		p.warn(conn, fmt.Errorf("build request: %w", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", cfg.APIKey)

	resp, err := datadogClient.Do(req)
	if err != nil {
		p.warn(conn, fmt.Errorf("http: %w", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		p.warn(conn, fmt.Errorf("non-2xx %d: %s", resp.StatusCode, string(respBody)))
		return
	}
	// Drain the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)
}

func (p *Plugin) warn(conn tables_enterprise.TableLogExportConnector, err error) {
	if p.logger == nil {
		return
	}
	p.logger.Warn(fmt.Sprintf("logexport: datadog connector %s (%s): %v", conn.Name, conn.ID, err))
}

// formatDDTags produces Datadog's comma-separated k:v list. Returns
// empty string when the tag map is empty so the intake doesn't see a
// stray trailing separator.
func formatDDTags(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tags))
	for k, v := range tags {
		parts = append(parts, k+":"+v)
	}
	return strings.Join(parts, ",")
}
