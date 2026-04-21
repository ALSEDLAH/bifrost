// OpenAI Moderation provider tests (spec 016 T012).
//
// Uses httptest.Server to stand in for the real API. Asserts match
// semantics, auth-header forwarding, fail-open default, and the
// fail-closed override.

package guardrailsruntime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

// newModerationServer returns an httptest.Server whose /v1/moderations
// endpoint replies with a JSON body driven by the provided handler.
// The handler receives the request text (from the JSON body) and
// returns the value to stuff into results[0].flagged.
func newModerationServer(t *testing.T, handler func(text string) (status int, flagged bool)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/moderations" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("moderation: missing Bearer auth header; got %q", got)
		}
		var body struct {
			Input string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		status, flagged := handler(body.Input)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status >= 200 && status < 300 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"flagged": flagged, "categories": map[string]bool{}}},
			})
		} else {
			fmt.Fprintf(w, `{"error": "simulated %d"}`, status)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func moderationProvider(baseURL string) tables_enterprise.TableGuardrailProvider {
	cfg, _ := json.Marshal(map[string]string{"api_key": "test-key", "base_url": baseURL})
	return tables_enterprise.TableGuardrailProvider{
		ID: "prov1", Name: "openai-test", Type: "openai-moderation",
		Config: string(cfg), Enabled: true,
	}
}

func TestModeration_FlaggedTriggersBlock(t *testing.T) {
	srv := newModerationServer(t, func(text string) (int, bool) {
		return http.StatusOK, strings.Contains(text, "bad")
	})

	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r1", Name: "mod-block", ProviderID: "prov1",
			Trigger: "input", Action: "block", Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, []tables_enterprise.TableGuardrailProvider{moderationProvider(srv.URL)})

	// Non-flagged input passes through.
	_, sc, err := p.PreLLMHook(nil, chatReq("clean text"))
	if err != nil || sc != nil {
		t.Fatalf("expected pass-through for clean input; sc=%+v err=%v", sc, err)
	}

	// Flagged input blocks.
	_, sc, err = p.PreLLMHook(nil, chatReq("this is bad content"))
	if err != nil {
		t.Fatalf("hook err: %v", err)
	}
	if sc == nil || sc.Error == nil || sc.Error.StatusCode == nil || *sc.Error.StatusCode != 451 {
		t.Errorf("expected 451 block; got %+v", sc)
	}
	if len(fa.entries) != 1 {
		t.Errorf("expected 1 audit entry for the block; got %d", len(fa.entries))
	}
}

func TestModeration_NetworkErrorFailsOpenByDefault(t *testing.T) {
	// Point at a closed port so every call errors out.
	cfg, _ := json.Marshal(map[string]string{"api_key": "k", "base_url": "http://127.0.0.1:1"})
	prov := tables_enterprise.TableGuardrailProvider{
		ID: "p-dead", Name: "dead", Type: "openai-moderation",
		Config: string(cfg), Enabled: true,
	}
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r", Name: "mod-block", ProviderID: "p-dead",
			Trigger: "input", Action: "block", Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, []tables_enterprise.TableGuardrailProvider{prov})

	// Error → fail-open → request passes through, no audit emitted on
	// eval error (only a plugin-level Warn logged).
	_, sc, err := p.PreLLMHook(nil, chatReq("anything"))
	if err != nil {
		t.Fatalf("hook err: %v", err)
	}
	if sc != nil {
		t.Errorf("fail-open default should let request through; got short-circuit %+v", sc)
	}
	if len(fa.entries) != 0 {
		t.Errorf("no audit on eval error in fail-open mode; got %+v", fa.entries)
	}
}

func TestModeration_FailClosedBlocksOnNetworkError(t *testing.T) {
	cfg, _ := json.Marshal(map[string]string{"api_key": "k", "base_url": "http://127.0.0.1:1"})
	prov := tables_enterprise.TableGuardrailProvider{
		ID: "p-dead", Name: "dead", Type: "openai-moderation",
		Config: string(cfg), Enabled: true,
	}
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r", Name: "mod-fc", ProviderID: "p-dead",
			Trigger: "input", Action: "block", Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, []tables_enterprise.TableGuardrailProvider{prov})
	// Flip the single rule to fail_closed — simulates setting the
	// flag from the UI. Direct slice mutation is fine in tests.
	p.mu.Lock()
	for i := range p.rules {
		p.rules[i].failClosed = true
	}
	p.mu.Unlock()

	_, sc, _ := p.PreLLMHook(nil, chatReq("anything"))
	if sc == nil {
		t.Errorf("fail_closed + provider error must block; got pass-through")
	}
	if len(fa.entries) != 1 {
		t.Errorf("expected 1 audit on fail_closed block; got %d", len(fa.entries))
	}
}

func TestModeration_Non2xxIsTreatedAsEvalError(t *testing.T) {
	srv := newModerationServer(t, func(text string) (int, bool) {
		return http.StatusTooManyRequests, false
	})
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r", Name: "mod-rate", ProviderID: "prov1",
			Trigger: "input", Action: "block", Enabled: true},
	}
	p, _ := buildPluginWithRules(rules, []tables_enterprise.TableGuardrailProvider{moderationProvider(srv.URL)})

	_, sc, _ := p.PreLLMHook(nil, chatReq("hello"))
	// fail-open default: non-2xx is an error path → pass-through.
	if sc != nil {
		t.Errorf("non-2xx must fail-open by default; got short-circuit %+v", sc)
	}
}
