// Package alertchannels dispatches governance events (e.g. budget
// threshold crossings) to user-configured destinations: generic HTTP
// webhooks and Slack incoming webhooks.
//
// Per specs/004-alert-channels/spec.md FR-003/FR-004. Dispatch is
// fire-and-forget: failures are logged, never block the caller.
package alertchannels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

// ChannelType enumerates supported destinations.
type ChannelType string

const (
	ChannelTypeWebhook ChannelType = "webhook"
	ChannelTypeSlack   ChannelType = "slack"
)

// Event is the dispatched payload shape. Kept intentionally flat so
// both JSON webhook and Slack text renderers can work from it.
type Event struct {
	Type      string         `json:"event_type"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// WebhookConfig is the JSON-unmarshalled Config blob when Type=webhook.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`  // defaults to POST
	Headers map[string]string `json:"headers,omitempty"` // optional extra headers
}

// SlackConfig is the JSON-unmarshalled Config blob when Type=slack.
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// Dispatcher sends events to a set of enabled channels.
type Dispatcher struct {
	client *http.Client
	logger schemas.Logger
}

// New constructs a dispatcher with a 5s HTTP timeout (NFR-002).
func New(logger schemas.Logger) *Dispatcher {
	return &Dispatcher{
		client: &http.Client{Timeout: 5 * time.Second},
		logger: logger,
	}
}

// Send fans out event to every enabled channel in channels. Each send
// runs in its own goroutine so slow/broken channels don't block the
// caller. Returns immediately.
func (d *Dispatcher) Send(channels []tables_enterprise.TableAlertChannel, ev Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		c := ch // capture
		go d.sendOne(c, ev)
	}
}

func (d *Dispatcher) sendOne(ch tables_enterprise.TableAlertChannel, ev Event) {
	defer func() {
		if r := recover(); r != nil && d.logger != nil {
			d.logger.Error(fmt.Sprintf("alertchannels: panic dispatching to %s (%s): %v", ch.ID, ch.Type, r))
		}
	}()

	switch ChannelType(ch.Type) {
	case ChannelTypeWebhook:
		var cfg WebhookConfig
		if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
			d.logError(ch, fmt.Errorf("parse webhook config: %w", err))
			return
		}
		if cfg.URL == "" {
			d.logError(ch, fmt.Errorf("webhook config missing url"))
			return
		}
		method := cfg.Method
		if method == "" {
			method = http.MethodPost
		}
		body, err := json.Marshal(ev)
		if err != nil {
			d.logError(ch, fmt.Errorf("marshal event: %w", err))
			return
		}
		req, err := http.NewRequestWithContext(context.Background(), method, cfg.URL, bytes.NewReader(body))
		if err != nil {
			d.logError(ch, fmt.Errorf("build request: %w", err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range cfg.Headers {
			req.Header.Set(k, v)
		}
		d.doRequest(ch, req)

	case ChannelTypeSlack:
		var cfg SlackConfig
		if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
			d.logError(ch, fmt.Errorf("parse slack config: %w", err))
			return
		}
		if cfg.WebhookURL == "" {
			d.logError(ch, fmt.Errorf("slack config missing webhook_url"))
			return
		}
		text := formatSlackText(ev)
		body, _ := json.Marshal(map[string]string{"text": text})
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, cfg.WebhookURL, bytes.NewReader(body))
		if err != nil {
			d.logError(ch, fmt.Errorf("build slack request: %w", err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		d.doRequest(ch, req)

	default:
		d.logError(ch, fmt.Errorf("unsupported channel type %q", ch.Type))
	}
}

func (d *Dispatcher) doRequest(ch tables_enterprise.TableAlertChannel, req *http.Request) {
	resp, err := d.client.Do(req)
	if err != nil {
		d.logError(ch, fmt.Errorf("http: %w", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	d.logError(ch, fmt.Errorf("non-2xx status %d: %s", resp.StatusCode, string(body)))
}

func (d *Dispatcher) logError(ch tables_enterprise.TableAlertChannel, err error) {
	if d.logger == nil {
		return
	}
	d.logger.Warn(fmt.Sprintf("alertchannels: dispatch failed channel=%s type=%s err=%v", ch.ID, ch.Type, err))
}

// formatSlackText renders an Event into a plain-text Slack message.
// v1 keeps it flat; blocks/attachments are a v2 concern.
func formatSlackText(ev Event) string {
	var buf bytes.Buffer
	buf.WriteString(":rotating_light: *")
	buf.WriteString(ev.Type)
	buf.WriteString("*\n")
	for k, v := range ev.Data {
		fmt.Fprintf(&buf, "• %s: %v\n", k, v)
	}
	return buf.String()
}
