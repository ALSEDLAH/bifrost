// Regex-rule behavioural tests (spec 016 T011).
//
// Table-driven: a minimal rule set fed into buildRuleIndex, then
// each test case exercises the plugin's PreLLMHook / PostLLMHook
// with a crafted request/response and asserts the action outcome
// (block / flag / log / pass-through).

package guardrailsruntime

import (
	"context"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/plugins/audit"
)

// testLogger satisfies schemas.Logger with no output.
type testLogger struct{}

func (testLogger) Debug(string, ...any)                   {}
func (testLogger) Info(string, ...any)                    {}
func (testLogger) Warn(string, ...any)                    {}
func (testLogger) Error(string, ...any)                   {}
func (testLogger) Fatal(string, ...any)                   {}
func (testLogger) SetLevel(schemas.LogLevel)              {}
func (testLogger) SetOutputType(schemas.LoggerOutputType) {}
func (testLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return schemas.NoopLogEvent
}

// fakeAuditEmitter records emitted entries for assertion. No DB.
type fakeAuditEmitter struct {
	entries []audit.Entry
}

func (f *fakeAuditEmitter) emit(_ context.Context, _ *schemas.BifrostContext, e audit.Entry) error {
	f.entries = append(f.entries, e)
	return nil
}

// buildPluginWithRules constructs a Plugin directly (bypassing Init)
// with a hand-built rule set so the tests don't need a ConfigStore.
func buildPluginWithRules(rules []tables_enterprise.TableGuardrailRule, providers []tables_enterprise.TableGuardrailProvider) (*Plugin, *fakeAuditEmitter) {
	p := &Plugin{logger: testLogger{}}
	p.rules = buildRuleIndex(providers, rules, p.logger)
	fa := &fakeAuditEmitter{}
	p.SetAuditEmitter(fa.emit)
	return p, fa
}

// strPtrVal is a helper for chat message content.
func strPtrVal(s string) *string { return &s }

// chatReq builds a minimal BifrostChatRequest with one user message.
func chatReq(text string) *schemas.BifrostRequest {
	return &schemas.BifrostRequest{
		ChatRequest: &schemas.BifrostChatRequest{
			Input: []schemas.ChatMessage{
				{
					Role:    schemas.ChatMessageRoleUser,
					Content: &schemas.ChatMessageContent{ContentStr: strPtrVal(text)},
				},
			},
		},
	}
}

// chatResp builds a minimal BifrostResponse with one assistant message.
func chatResp(text string) *schemas.BifrostResponse {
	return &schemas.BifrostResponse{
		ChatResponse: &schemas.BifrostChatResponse{
			Choices: []schemas.BifrostResponseChoice{
				{
					ChatNonStreamResponseChoice: &schemas.ChatNonStreamResponseChoice{
						Message: &schemas.ChatMessage{
							Role:    schemas.ChatMessageRoleAssistant,
							Content: &schemas.ChatMessageContent{ContentStr: strPtrVal(text)},
						},
					},
				},
			},
		},
	}
}

// ccnPattern matches any 16-digit run — rough PII canary for tests.
const ccnPattern = `\b\d{16}\b`

func TestPreLLMHook_RegexBlockOnInput(t *testing.T) {
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r1", Name: "ccn-block", Trigger: "input", Action: "block", Pattern: ccnPattern, Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, nil)

	req := chatReq("my card number is 4111111111111111 please charge it")
	_, sc, err := p.PreLLMHook(&schemas.BifrostContext{}, req)

	if err != nil {
		t.Fatalf("unexpected hook error: %v", err)
	}
	if sc == nil || sc.Error == nil {
		t.Fatalf("expected short-circuit with error; got %+v", sc)
	}
	if sc.Error.StatusCode == nil || *sc.Error.StatusCode != 451 {
		t.Errorf("expected status 451; got %+v", sc.Error.StatusCode)
	}
	if sc.Error.Error == nil || !strings.Contains(sc.Error.Error.Message, "ccn-block") {
		t.Errorf("expected rule name in error message; got %+v", sc.Error.Error)
	}
	if len(fa.entries) != 1 || fa.entries[0].Action != "guardrail.block" || fa.entries[0].Outcome != "denied" {
		t.Errorf("expected one guardrail.block audit entry; got %+v", fa.entries)
	}
}

func TestPreLLMHook_RegexFlagOnInput_RequestPassesWithFlag(t *testing.T) {
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r2", Name: "ccn-flag", Trigger: "input", Action: "flag", Pattern: ccnPattern, Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, nil)

	bctx := &schemas.BifrostContext{}
	req := chatReq("card 4111111111111111")
	out, sc, err := p.PreLLMHook(bctx, req)
	if err != nil {
		t.Fatalf("unexpected hook error: %v", err)
	}
	if sc != nil {
		t.Fatalf("flag must not short-circuit; got %+v", sc)
	}
	if out != req {
		t.Errorf("flag must not mutate the request shape")
	}
	if len(fa.entries) != 1 || fa.entries[0].Action != "guardrail.flag" || fa.entries[0].Outcome != "allowed" {
		t.Errorf("expected one guardrail.flag audit entry; got %+v", fa.entries)
	}
	// Context value should be populated.
	if v, _ := bctx.Value(BifrostContextKeyGuardrailFlags).(string); v == "" {
		t.Errorf("expected BifrostContextKeyGuardrailFlags to be set on flag")
	}
}

func TestPreLLMHook_RegexLogOnInput_RequestPassesWithoutFlag(t *testing.T) {
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r3", Name: "ccn-log", Trigger: "input", Action: "log", Pattern: ccnPattern, Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, nil)

	bctx := &schemas.BifrostContext{}
	req := chatReq("card 4111111111111111")
	_, sc, err := p.PreLLMHook(bctx, req)
	if err != nil {
		t.Fatalf("unexpected hook error: %v", err)
	}
	if sc != nil {
		t.Fatalf("log must not short-circuit")
	}
	if len(fa.entries) != 1 || fa.entries[0].Action != "guardrail.log" {
		t.Errorf("expected one guardrail.log audit entry; got %+v", fa.entries)
	}
	// Log action does not set the flag key — only `flag` does.
	if v, _ := bctx.Value(BifrostContextKeyGuardrailFlags).(string); v != "" {
		t.Errorf("log action must not set flag context key")
	}
}

func TestPreLLMHook_NoMatchPassesThrough(t *testing.T) {
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r1", Name: "ccn", Trigger: "input", Action: "block", Pattern: ccnPattern, Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, nil)
	_, sc, err := p.PreLLMHook(&schemas.BifrostContext{}, chatReq("hello world"))
	if err != nil {
		t.Fatalf("hook error: %v", err)
	}
	if sc != nil {
		t.Fatalf("expected pass-through; got short-circuit: %+v", sc)
	}
	if len(fa.entries) != 0 {
		t.Errorf("expected no audit entries; got %+v", fa.entries)
	}
}

func TestPostLLMHook_RegexBlockOnOutput(t *testing.T) {
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r4", Name: "ccn-output", Trigger: "output", Action: "block", Pattern: ccnPattern, Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, nil)

	resp := chatResp("sure, your card 4111111111111111 is valid")
	outResp, outErr, err := p.PostLLMHook(&schemas.BifrostContext{}, resp, nil)
	if err != nil {
		t.Fatalf("hook error: %v", err)
	}
	if outResp != nil {
		t.Errorf("output block must nil the response")
	}
	if outErr == nil || outErr.StatusCode == nil || *outErr.StatusCode != 451 {
		t.Errorf("expected 451 BifrostError; got %+v", outErr)
	}
	if len(fa.entries) != 1 || fa.entries[0].Action != "guardrail.block" {
		t.Errorf("expected one guardrail.block audit entry; got %+v", fa.entries)
	}
}

func TestTriggerBoth_FiresOnInputAndOutput(t *testing.T) {
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r5", Name: "ccn-both", Trigger: "both", Action: "flag", Pattern: ccnPattern, Enabled: true},
	}
	p, fa := buildPluginWithRules(rules, nil)

	bctx := &schemas.BifrostContext{}
	_, _, _ = p.PreLLMHook(bctx, chatReq("card 4111111111111111"))
	_, _, _ = p.PostLLMHook(bctx, chatResp("card 4111111111111111"), nil)

	if len(fa.entries) != 2 {
		t.Fatalf("expected 2 audit entries (input+output); got %d", len(fa.entries))
	}
	// The two entries come from the same rule on different triggers.
	trigs := map[string]bool{}
	for _, e := range fa.entries {
		if t, ok := e.After.(map[string]any)["trigger"].(string); ok {
			trigs[t] = true
		}
	}
	if !trigs["input"] || !trigs["output"] {
		t.Errorf("both input and output triggers should have fired; got %v", trigs)
	}
}

func TestDisabledRule_DoesNotMatch(t *testing.T) {
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "r-off", Name: "off", Trigger: "input", Action: "block", Pattern: ccnPattern, Enabled: false},
	}
	p, fa := buildPluginWithRules(rules, nil)
	_, sc, _ := p.PreLLMHook(&schemas.BifrostContext{}, chatReq("4111111111111111"))
	if sc != nil {
		t.Errorf("disabled rule must not short-circuit")
	}
	if len(fa.entries) != 0 {
		t.Errorf("disabled rule must not audit")
	}
}

func TestBadRegex_RuleIsSkipped(t *testing.T) {
	rules := []tables_enterprise.TableGuardrailRule{
		{ID: "bad", Name: "bad-pattern", Trigger: "input", Action: "block", Pattern: `(?P<`, Enabled: true},
		{ID: "good", Name: "ccn", Trigger: "input", Action: "block", Pattern: ccnPattern, Enabled: true},
	}
	p, _ := buildPluginWithRules(rules, nil)
	// Only the compiled rule should remain — the bad one is skipped.
	if p.RuleCount() != 1 {
		t.Errorf("expected 1 rule (bad regex skipped); got %d", p.RuleCount())
	}
}
