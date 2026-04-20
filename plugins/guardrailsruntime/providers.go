// External-provider evaluators for the guardrails runtime (spec 016
// Phase 2). Each evaluator takes the rule + candidate text and
// returns (matched, err).
//
// Failure semantics: by default the caller (hooks.go) logs the error
// and lets the request through (fail-open). Rules with
// `fail_closed=true` (webhook/moderation config) treat any error as
// a match, for defense-in-depth on security-critical rules.

package guardrailsruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const externalCallTimeout = 2 * time.Second

// httpClient is shared across evaluators so we reuse idle connections.
var httpClient = &http.Client{Timeout: externalCallTimeout}

// evaluateModeration calls OpenAI's /v1/moderations endpoint with the
// candidate text. Matches when `results[0].flagged` is true.
// No category filters in v1 — any flagged category counts as a match.
func evaluateModeration(r *ruleEntry, text string) (bool, error) {
	if r.provider == nil {
		return false, fmt.Errorf("moderation rule %s has no provider", r.id)
	}
	if r.provider.apiKey == "" {
		return false, fmt.Errorf("moderation provider %s missing api_key", r.provider.id)
	}
	base := r.provider.baseURL
	if base == "" {
		base = "https://api.openai.com"
	}
	payload := map[string]any{
		"input": text,
		"model": "omni-moderation-latest",
	}
	body, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), externalCallTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(base, "/")+"/v1/moderations", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("moderation: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.provider.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("moderation: http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return false, fmt.Errorf("moderation: non-2xx %d: %s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		Results []struct {
			Flagged    bool            `json:"flagged"`
			Categories json.RawMessage `json:"categories"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return false, fmt.Errorf("moderation: decode: %w", err)
	}
	if len(parsed.Results) == 0 {
		return false, nil
	}
	return parsed.Results[0].Flagged, nil
}

// webhookRequest is what we POST to custom-webhook providers.
type webhookRequest struct {
	Text     string `json:"text"`
	Trigger  string `json:"trigger"`
	RuleID   string `json:"rule_id"`
	RuleName string `json:"rule_name"`
	Pattern  string `json:"pattern,omitempty"`
}

// webhookResponse is what we expect back. `match: true` is the
// canonical match indicator; `reason` is optional.
type webhookResponse struct {
	Match  bool   `json:"match"`
	Reason string `json:"reason,omitempty"`
}

// evaluateWebhook POSTs the candidate text to the configured webhook
// URL and reads back `{match: bool}`. 2-second timeout. 2xx with
// `match: true` is a rule hit.
func evaluateWebhook(r *ruleEntry, triggerName, text string) (bool, error) {
	if r.provider == nil {
		return false, fmt.Errorf("webhook rule %s has no provider", r.id)
	}
	if r.provider.webhookURL == "" {
		return false, fmt.Errorf("webhook provider %s missing url", r.provider.id)
	}
	body, _ := json.Marshal(webhookRequest{
		Text:     text,
		Trigger:  triggerName,
		RuleID:   r.id,
		RuleName: r.name,
		Pattern:  r.pattern,
	})
	ctx, cancel := context.WithTimeout(context.Background(), externalCallTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.provider.webhookURL, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("webhook: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("webhook: http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return false, fmt.Errorf("webhook: non-2xx %d: %s", resp.StatusCode, string(raw))
	}

	var parsed webhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return false, fmt.Errorf("webhook: decode: %w", err)
	}
	return parsed.Match, nil
}
