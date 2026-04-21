// Custom-webhook provider tests (spec 016 T013).

package guardrailsruntime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

func newWebhookServer(t *testing.T, respond func(req webhookRequest) (int, webhookResponse)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body webhookRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		status, resp := respond(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func webhookProvider(url string) tables_enterprise.TableGuardrailProvider {
	cfg, _ := json.Marshal(map[string]string{"url": url})
	return tables_enterprise.TableGuardrailProvider{
		ID: "wp", Name: "webhook-test", Type: "custom-webhook",
		Config: string(cfg), Enabled: true,
	}
}

func TestWebhook_MatchTrueBlocks(t *testing.T) {
	srv := newWebhookServer(t, func(r webhookRequest) (int, webhookResponse) {
		return http.StatusOK, webhookResponse{Match: true, Reason: "synthetic"}
	})
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r", Name: "webhook-block", ProviderID: "wp",
			Trigger: "input", Action: "block", Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, []tables_enterprise.TableGuardrailProvider{webhookProvider(srv.URL)})

	_, sc, _ := p.PreLLMHook(nil, chatReq("anything"))
	if sc == nil || sc.Error == nil || sc.Error.StatusCode == nil || *sc.Error.StatusCode != 451 {
		t.Errorf("match:true must block with 451; got %+v", sc)
	}
	if len(fa.entries) != 1 {
		t.Errorf("expected 1 audit entry; got %d", len(fa.entries))
	}
}

func TestWebhook_MatchFalsePassesThrough(t *testing.T) {
	srv := newWebhookServer(t, func(r webhookRequest) (int, webhookResponse) {
		return http.StatusOK, webhookResponse{Match: false}
	})
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r", Name: "webhook-pass", ProviderID: "wp",
			Trigger: "input", Action: "block", Enabled: true},
	}
	p, _ := buildPluginWithRules(rules, []tables_enterprise.TableGuardrailProvider{webhookProvider(srv.URL)})

	_, sc, _ := p.PreLLMHook(nil, chatReq("anything"))
	if sc != nil {
		t.Errorf("match:false must not short-circuit; got %+v", sc)
	}
}

func TestWebhook_CarriesRuleMetadataInBody(t *testing.T) {
	var received webhookRequest
	srv := newWebhookServer(t, func(r webhookRequest) (int, webhookResponse) {
		received = r
		return http.StatusOK, webhookResponse{Match: false}
	})
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r-meta", Name: "meta", ProviderID: "wp",
			Trigger: "input", Action: "log", Pattern: "", Enabled: true},
	}
	p, _ := buildPluginWithRules(rules, []tables_enterprise.TableGuardrailProvider{webhookProvider(srv.URL)})

	_, _, _ = p.PreLLMHook(nil, chatReq("hi"))
	if received.RuleID != "r-meta" || received.RuleName != "meta" || received.Trigger != "input" {
		t.Errorf("webhook body should carry rule_id / rule_name / trigger; got %+v", received)
	}
	if received.Text != "hi" {
		t.Errorf("webhook body should carry the candidate text; got %q", received.Text)
	}
}

func TestWebhook_5xxFailsOpenByDefault(t *testing.T) {
	srv := newWebhookServer(t, func(r webhookRequest) (int, webhookResponse) {
		return http.StatusInternalServerError, webhookResponse{}
	})
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r", Name: "webhook-5xx", ProviderID: "wp",
			Trigger: "input", Action: "block", Enabled: true},
	}
	p, _ := buildPluginWithRules(rules, []tables_enterprise.TableGuardrailProvider{webhookProvider(srv.URL)})

	_, sc, _ := p.PreLLMHook(nil, chatReq("anything"))
	if sc != nil {
		t.Errorf("5xx on webhook must fail-open by default; got short-circuit %+v", sc)
	}
}
