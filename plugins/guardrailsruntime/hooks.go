// PreLLMHook + PostLLMHook — Phase 1 regex-only path (spec 016).
//
// External-provider evaluation (OpenAI Moderation, custom webhook)
// is stubbed for this iteration — those arrive in Phase 2 of the
// spec (next loop wakes).

package guardrailsruntime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

// GuardrailFlag is the JSON shape attached to the request context on
// `flag` action so downstream middleware (audit, observability) can
// record the hit without blocking.
type GuardrailFlag struct {
	RuleID    string    `json:"rule_id"`
	RuleName  string    `json:"rule_name"`
	Trigger   string    `json:"trigger"`
	MatchedAt time.Time `json:"matched_at"`
}

// PreLLMHook evaluates input-triggered rules. Block actions return a
// LLMPluginShortCircuit with an HTTP 451 error. Flag + log actions
// attach context + audit (audit wired in Phase 3).
func (p *Plugin) PreLLMHook(
	ctx *schemas.BifrostContext,
	req *schemas.BifrostRequest,
) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	if p == nil || req == nil {
		return req, nil, nil
	}
	p.mu.RLock()
	rules := p.rules
	p.mu.RUnlock()
	if len(rules) == 0 {
		return req, nil, nil
	}

	text := extractRequestText(req)
	if text == "" {
		return req, nil, nil
	}

	var flags []GuardrailFlag
	for i := range rules {
		r := &rules[i]
		if !r.triggersOnInput() {
			continue
		}
		matched, err := p.evaluate(r, "input", text)
		if err != nil {
			warn(p.logger, fmt.Sprintf("guardrails-runtime: rule %s eval error: %v", r.id, err))
			if !r.failClosed {
				continue
			}
			// fail_closed + error → treat as match for safety
			matched = true
		}
		if !matched {
			continue
		}
		switch r.action {
		case ActionBlock:
			p.emitAudit(ctx, r, "input", "block", fmt.Sprintf("input matched rule %q", r.name))
			return req, blockShortCircuit(r), nil
		case ActionFlag:
			p.emitAudit(ctx, r, "input", "flag", fmt.Sprintf("input matched rule %q (non-blocking)", r.name))
			flags = append(flags, GuardrailFlag{
				RuleID: r.id, RuleName: r.name,
				Trigger: "input", MatchedAt: time.Now().UTC(),
			})
		case ActionLog:
			p.emitAudit(ctx, r, "input", "log", fmt.Sprintf("input matched rule %q (log-only)", r.name))
			if p.logger != nil {
				p.logger.Info(fmt.Sprintf("guardrails-runtime: rule %s matched on input (log-only)", r.name))
			}
		}
	}
	if len(flags) > 0 && ctx != nil {
		if buf, err := json.Marshal(flags); err == nil {
			ctx.SetValue(BifrostContextKeyGuardrailFlags, string(buf))
		}
	}
	return req, nil, nil
}

// PostLLMHook scans response text against output+both-trigger rules.
// Block on post converts the upstream response to a 451-style error;
// flag attaches context metadata; log is audit-only (wired in phase 3).
func (p *Plugin) PostLLMHook(
	ctx *schemas.BifrostContext,
	resp *schemas.BifrostResponse,
	bifrostErr *schemas.BifrostError,
) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	if p == nil || resp == nil || bifrostErr != nil {
		// Don't run guardrails when the upstream already errored —
		// adding our own error on top is noisy and provides no value.
		return resp, bifrostErr, nil
	}
	p.mu.RLock()
	rules := p.rules
	p.mu.RUnlock()
	if len(rules) == 0 {
		return resp, bifrostErr, nil
	}

	text := extractResponseText(resp)
	if text == "" {
		return resp, bifrostErr, nil
	}

	var flags []GuardrailFlag
	for i := range rules {
		r := &rules[i]
		if !r.triggersOnOutput() {
			continue
		}
		matched, err := p.evaluate(r, "output", text)
		if err != nil {
			warn(p.logger, fmt.Sprintf("guardrails-runtime: rule %s eval error on output: %v", r.id, err))
			if !r.failClosed {
				continue
			}
			matched = true
		}
		if !matched {
			continue
		}
		switch r.action {
		case ActionBlock:
			p.emitAudit(ctx, r, "output", "block", fmt.Sprintf("output matched rule %q", r.name))
			status := http.StatusUnavailableForLegalReasons
			msg := fmt.Sprintf("guardrail blocked output: %s", r.name)
			return nil, &schemas.BifrostError{
				StatusCode: &status,
				Error: &schemas.ErrorField{
					Message: msg,
					Type:    strPtr("guardrail_blocked_output"),
				},
			}, nil
		case ActionFlag:
			p.emitAudit(ctx, r, "output", "flag", fmt.Sprintf("output matched rule %q (non-blocking)", r.name))
			flags = append(flags, GuardrailFlag{
				RuleID: r.id, RuleName: r.name,
				Trigger: "output", MatchedAt: time.Now().UTC(),
			})
		case ActionLog:
			p.emitAudit(ctx, r, "output", "log", fmt.Sprintf("output matched rule %q (log-only)", r.name))
			if p.logger != nil {
				p.logger.Info(fmt.Sprintf("guardrails-runtime: rule %s matched on output (log-only)", r.name))
			}
		}
	}
	if len(flags) > 0 && ctx != nil {
		// Append to existing context flags if any.
		if existing, ok := ctx.Value(BifrostContextKeyGuardrailFlags).(string); ok && existing != "" {
			var prev []GuardrailFlag
			_ = json.Unmarshal([]byte(existing), &prev)
			flags = append(prev, flags...)
		}
		if buf, err := json.Marshal(flags); err == nil {
			ctx.SetValue(BifrostContextKeyGuardrailFlags, string(buf))
		}
	}
	return resp, bifrostErr, nil
}

// extractResponseText flattens non-stream chat + Responses output
// to a single string. Streaming responses (delta chunks) are out of
// scope for v1 — streaming scanner is a Phase 3 follow-up.
func extractResponseText(resp *schemas.BifrostResponse) string {
	if resp == nil {
		return ""
	}
	var b strings.Builder
	if resp.ChatResponse != nil {
		for _, c := range resp.ChatResponse.Choices {
			if c.ChatNonStreamResponseChoice == nil || c.ChatNonStreamResponseChoice.Message == nil {
				continue
			}
			m := c.ChatNonStreamResponseChoice.Message
			if m.Content == nil {
				continue
			}
			if m.Content.ContentStr != nil {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(*m.Content.ContentStr)
			}
			if m.Content.ContentBlocks != nil {
				for _, block := range m.Content.ContentBlocks {
					if block.Text != nil {
						if b.Len() > 0 {
							b.WriteByte('\n')
						}
						b.WriteString(*block.Text)
					}
				}
			}
		}
	}
	return b.String()
}

// evaluate routes the rule through its provider's evaluator.
// Phase 1 shipped regex only; Phase 2 adds external providers.
func (p *Plugin) evaluate(r *ruleEntry, triggerName, text string) (bool, error) {
	// Bare regex rule (no provider) or provider type == regex.
	if r.regex != nil {
		return r.regex.MatchString(text), nil
	}
	if r.provider == nil {
		return false, nil
	}
	switch r.provider.typ {
	case ProviderOpenAIModeration:
		return evaluateModeration(r, text)
	case ProviderCustomWebhook:
		return evaluateWebhook(r, triggerName, text)
	default:
		return false, nil
	}
}

// blockShortCircuit constructs the BifrostError used for block
// actions. Status 451 is semantically appropriate for
// "refused for policy" per RFC 7725.
func blockShortCircuit(r *ruleEntry) *schemas.LLMPluginShortCircuit {
	status := http.StatusUnavailableForLegalReasons // 451
	msg := fmt.Sprintf("guardrail blocked: %s", r.name)
	return &schemas.LLMPluginShortCircuit{
		Error: &schemas.BifrostError{
			StatusCode: &status,
			Error: &schemas.ErrorField{
				Message: msg,
				Type:    strPtr("guardrail_blocked"),
			},
		},
	}
}

func strPtr(s string) *string { return &s }

// extractRequestText pulls a flat string out of chat or Responses
// messages for regex evaluation. Returns empty when the request
// shape has no user-prose payload (embeddings, speech input, etc.)
// — those skip guardrails until a spec adds dedicated scanners.
func extractRequestText(req *schemas.BifrostRequest) string {
	if req == nil {
		return ""
	}
	var b strings.Builder
	if req.ChatRequest != nil {
		for _, m := range req.ChatRequest.Input {
			if m.Content == nil {
				continue
			}
			if m.Content.ContentStr != nil {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(*m.Content.ContentStr)
			}
			if m.Content.ContentBlocks != nil {
				for _, block := range m.Content.ContentBlocks {
					if block.Text != nil {
						if b.Len() > 0 {
							b.WriteByte('\n')
						}
						b.WriteString(*block.Text)
					}
				}
			}
		}
	}
	return b.String()
}
