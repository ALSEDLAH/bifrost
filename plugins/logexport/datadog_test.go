// Datadog forwarder tests (spec 017 T007, T008, T009).
//
// Uses httptest.Server to stand in for the Datadog Logs intake.
// Asserts body shape, auth-header forwarding, fail-open on error,
// BigQuery skip path, and Invalidate() cache reload.

package logexport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

// silentLogger is a schemas.Logger with no output.
type silentLogger struct{}

func (silentLogger) Debug(string, ...any)                   {}
func (silentLogger) Info(string, ...any)                    {}
func (silentLogger) Warn(string, ...any)                    {}
func (silentLogger) Error(string, ...any)                   {}
func (silentLogger) Fatal(string, ...any)                   {}
func (silentLogger) SetLevel(schemas.LogLevel)              {}
func (silentLogger) SetOutputType(schemas.LoggerOutputType) {}
func (silentLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return schemas.NoopLogEvent
}

// newSQLiteStore spins up an in-memory SQLite configstore with the
// enterprise migrations applied. Shared across tests.
func newSQLiteStore(t *testing.T) configstore.ConfigStore {
	t.Helper()
	dir := t.TempDir()
	cfg := &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: filepath.Join(dir, "cs.db")},
	}
	store, err := configstore.NewConfigStore(context.Background(), cfg, silentLogger{})
	if err != nil {
		t.Fatalf("store init: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	if err := configstore.RegisterEnterpriseMigrations(context.Background(), store.DB()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return store
}

// datadogCaptureServer stands in for Datadog's intake. It captures
// the first POST body + headers so tests can assert.
type datadogCaptureServer struct {
	mu    sync.Mutex
	body  []byte
	ddKey string
	calls int
	srv   *httptest.Server
	resp  int
}

func newDatadogCaptureServer(t *testing.T, respCode int) *datadogCaptureServer {
	c := &datadogCaptureServer{resp: respCode}
	c.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.calls++
		c.ddKey = r.Header.Get("DD-API-KEY")
		b, _ := readAll(r.Body)
		c.body = b
		w.WriteHeader(c.resp)
	}))
	t.Cleanup(c.srv.Close)
	return c
}

func readAll(r interface{ Read(p []byte) (int, error) }) ([]byte, error) {
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 512)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

// seedDatadogConnector builds a Datadog connector row whose intake
// URL points at captureServer (by overriding site to the captureServer
// host prefix — plugin derives the URL as
// https://http-intake.logs.<site>/api/v2/logs; we hijack it by using
// an empty site and pointing the plugin at a client that resolves
// the test URL).
func seedDatadogConnector(t *testing.T, store configstore.ConfigStore, site, apiKey string) string {
	t.Helper()
	cfg, _ := json.Marshal(map[string]string{"api_key": apiKey, "site": site})
	row := &tables_enterprise.TableLogExportConnector{
		Type:    "datadog",
		Name:    "test-dd",
		Config:  string(cfg),
		Enabled: true,
	}
	if err := store.CreateLogExportConnector(context.Background(), row); err != nil {
		t.Fatalf("create connector: %v", err)
	}
	return row.ID
}

// hookedDatadogClient overrides datadogClient for a test, pointing
// all Datadog POSTs at the httptest.Server regardless of URL. Uses
// a custom RoundTripper that rewrites the scheme+host of outgoing
// requests to the fake server.
func useCaptureServer(t *testing.T, capture *datadogCaptureServer) {
	t.Helper()
	orig := datadogClient
	datadogClient = &http.Client{
		Timeout: 2 * time.Second,
		Transport: rewritingTransport{to: capture.srv.URL},
	}
	t.Cleanup(func() { datadogClient = orig })
}

type rewritingTransport struct{ to string }

func (r rewritingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Parse the target host from the fake server URL and rewrite.
	u, _ := parseURL(r.to)
	req.URL.Scheme = u.scheme
	req.URL.Host = u.host
	req.Host = u.host
	return http.DefaultTransport.RoundTrip(req)
}

type parsedURL struct{ scheme, host string }

func parseURL(u string) (parsedURL, error) {
	// We can use net/url instead of this hand-rolled parser.
	p := parsedURL{scheme: "http"}
	if len(u) >= 7 && u[:7] == "http://" {
		p.host = u[7:]
	} else if len(u) >= 8 && u[:8] == "https://" {
		p.scheme = "https"
		p.host = u[8:]
	} else {
		p.host = u
	}
	return p, nil
}

func TestDatadog_InjectSendsBodyAndAuth(t *testing.T) {
	store := newSQLiteStore(t)
	seedDatadogConnector(t, store, "datadoghq.com", "test-dd-key")

	capture := newDatadogCaptureServer(t, http.StatusAccepted)
	useCaptureServer(t, capture)

	p, err := Init(context.Background(), store, silentLogger{})
	if err != nil {
		t.Fatalf("plugin init: %v", err)
	}

	trace := &schemas.Trace{
		RequestID:  "req-abc",
		TraceID:    "trace-xyz",
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		Attributes: map[string]any{"model": "gpt-4o-mini", "provider": "openai"},
	}
	if err := p.Inject(context.Background(), trace); err != nil {
		t.Fatalf("inject err: %v", err)
	}
	// Inject fires goroutines for delivery; wait briefly.
	waitForDelivery(t, capture)

	if capture.ddKey != "test-dd-key" {
		t.Errorf("expected DD-API-KEY to be forwarded; got %q", capture.ddKey)
	}
	// Body is a JSON array of one event.
	var body []map[string]any
	if err := json.Unmarshal(capture.body, &body); err != nil {
		t.Fatalf("body decode: %v — raw %s", err, string(capture.body))
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 log event; got %d", len(body))
	}
	ev := body[0]
	if ev["ddsource"] != "bifrost" || ev["service"] != "bifrost" {
		t.Errorf("source/service tags wrong: %+v", ev)
	}
	if ev["request_id"] != "req-abc" || ev["trace_id"] != "trace-xyz" {
		t.Errorf("request_id/trace_id missing: %+v", ev)
	}
	if ev["model"] != "gpt-4o-mini" || ev["provider"] != "openai" {
		t.Errorf("attributes missing: %+v", ev)
	}
}

func TestDatadog_5xxFromIntakeDoesNotPanic(t *testing.T) {
	store := newSQLiteStore(t)
	seedDatadogConnector(t, store, "datadoghq.com", "k")
	capture := newDatadogCaptureServer(t, http.StatusInternalServerError)
	useCaptureServer(t, capture)

	p, _ := Init(context.Background(), store, silentLogger{})
	trace := &schemas.Trace{RequestID: "r"}
	if err := p.Inject(context.Background(), trace); err != nil {
		t.Errorf("inject must not error on Datadog 5xx; got %v", err)
	}
	waitForDelivery(t, capture)
	if capture.calls != 1 {
		t.Errorf("expected one attempt; got %d", capture.calls)
	}
}

func TestBigQueryConnector_IsSkippedQuietly(t *testing.T) {
	store := newSQLiteStore(t)
	cfg, _ := json.Marshal(map[string]string{"project_id": "p", "dataset": "d", "table": "t", "credentials_json": "{}"})
	row := &tables_enterprise.TableLogExportConnector{
		Type: "bigquery", Name: "bq", Config: string(cfg), Enabled: true,
	}
	if err := store.CreateLogExportConnector(context.Background(), row); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Capture server exists but must receive NO requests — BigQuery
	// is skipped entirely in v1.
	capture := newDatadogCaptureServer(t, http.StatusOK)
	useCaptureServer(t, capture)

	p, _ := Init(context.Background(), store, silentLogger{})
	trace := &schemas.Trace{RequestID: "r"}
	_ = p.Inject(context.Background(), trace)
	time.Sleep(100 * time.Millisecond)

	if capture.calls != 0 {
		t.Errorf("BigQuery connector must not issue HTTP calls in v1; got %d", capture.calls)
	}
}

func TestInvalidate_ReloadsAddedConnector(t *testing.T) {
	store := newSQLiteStore(t)
	capture := newDatadogCaptureServer(t, http.StatusAccepted)
	useCaptureServer(t, capture)

	p, _ := Init(context.Background(), store, silentLogger{})
	// Initially zero connectors — Inject is a no-op.
	_ = p.Inject(context.Background(), &schemas.Trace{RequestID: "r0"})
	time.Sleep(50 * time.Millisecond)
	if capture.calls != 0 {
		t.Fatalf("no connectors ⇒ no calls; got %d", capture.calls)
	}

	// Add a connector out-of-band + invalidate.
	seedDatadogConnector(t, store, "datadoghq.com", "k2")
	p.Invalidate()
	_ = p.Inject(context.Background(), &schemas.Trace{RequestID: "r1"})
	waitForDelivery(t, capture)
	if capture.calls != 1 {
		t.Errorf("after invalidate + new connector, expected 1 call; got %d", capture.calls)
	}
}

// waitForDelivery polls the capture server for up to 500ms so the
// goroutine launched by Inject has a chance to finish. Fails the test
// if no call landed.
func waitForDelivery(t *testing.T, c *datadogCaptureServer) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		calls := c.calls
		c.mu.Unlock()
		if calls > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no call landed at the fake Datadog server before timeout")
}
