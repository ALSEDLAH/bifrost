// Integration test — full plugin pipeline with a real SQLite
// configstore, a fake Datadog intake, and a synthetic trace driven
// through Inject (spec 017 T010).

package logexport

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

// TestIntegration_FullPipeline seeds a Datadog connector in the real
// SQLite enterprise schema, boots the plugin, and asserts that a
// synthetic Trace flows end-to-end to the captured intake.
func TestIntegration_FullPipeline(t *testing.T) {
	store := newSQLiteStore(t)
	capture := newDatadogCaptureServer(t, http.StatusAccepted)
	useCaptureServer(t, capture)

	id := seedDatadogConnector(t, store, "datadoghq.com", "integ-key")
	_ = id

	p, err := Init(context.Background(), store, silentLogger{})
	if err != nil {
		t.Fatalf("plugin init: %v", err)
	}

	trace := &schemas.Trace{
		RequestID: "r-integ",
		TraceID:   "t-integ",
		EndTime:   time.Now(),
		Attributes: map[string]any{
			"model":       "claude-3-5-sonnet",
			"provider":    "anthropic",
			"latency_ms":  int64(842),
			"status_code": 200,
		},
	}

	if err := p.Inject(context.Background(), trace); err != nil {
		t.Fatalf("inject: %v", err)
	}
	waitForDelivery(t, capture)

	// Decode + assert the intake received what the trace produced.
	var body []map[string]any
	if err := json.Unmarshal(capture.body, &body); err != nil {
		t.Fatalf("body: %v — raw %s", err, string(capture.body))
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 event; got %d", len(body))
	}
	ev := body[0]
	if ev["model"] != "claude-3-5-sonnet" || ev["provider"] != "anthropic" {
		t.Errorf("attributes not forwarded: %+v", ev)
	}
	if ev["request_id"] != "r-integ" || ev["trace_id"] != "t-integ" {
		t.Errorf("trace ids not forwarded: %+v", ev)
	}
	// timestamp is ms-since-epoch; assert it's within +/- 5s of now.
	nowMs := time.Now().UnixMilli()
	ts, _ := ev["timestamp"].(float64)
	if delta := int64(ts) - nowMs; delta < -5000 || delta > 5000 {
		t.Errorf("timestamp out of range: got %v, now %d (delta %d)", ev["timestamp"], nowMs, delta)
	}
}
