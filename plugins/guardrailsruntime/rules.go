package guardrailsruntime

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/maximhq/bifrost/core/schemas"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
)

// Action enumerates the outcome options on a rule match.
type Action string

const (
	ActionBlock Action = "block"
	ActionFlag  Action = "flag"
	ActionLog   Action = "log"
)

// Trigger enumerates when a rule is evaluated.
type Trigger string

const (
	TriggerInput  Trigger = "input"
	TriggerOutput Trigger = "output"
	TriggerBoth   Trigger = "both"
)

// ProviderType enumerates the evaluation backends.
type ProviderType string

const (
	ProviderRegex            ProviderType = "regex"
	ProviderOpenAIModeration ProviderType = "openai-moderation"
	ProviderCustomWebhook    ProviderType = "custom-webhook"
)

// providerEntry holds the compiled config for a guardrail provider.
type providerEntry struct {
	id         string
	name       string
	typ        ProviderType
	apiKey     string // openai-moderation
	baseURL    string // openai-moderation
	webhookURL string // custom-webhook
}

// ruleEntry holds the compiled config for one rule.
type ruleEntry struct {
	id         string
	name       string
	trigger    Trigger
	action     Action
	pattern    string
	regex      *regexp.Regexp // non-nil for regex provider
	provider   *providerEntry // nil for regex provider
	failClosed bool
}

type webhookConfig struct {
	FailClosed bool `json:"fail_closed,omitempty"`
}

type moderationConfig struct {
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url,omitempty"`
	FailClosed bool   `json:"fail_closed,omitempty"`
}

type webhookProviderConfig struct {
	URL     string `json:"url"`
	Headers map[string]string
}

// buildRuleIndex compiles providers + rules into the plugin's
// runtime index. Malformed entries log a Warn and are skipped so
// a single bad row can't take the whole plugin down.
func buildRuleIndex(
	providers []tables_enterprise.TableGuardrailProvider,
	rules []tables_enterprise.TableGuardrailRule,
	logger schemas.Logger,
) []ruleEntry {
	provs := make(map[string]*providerEntry, len(providers))
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		entry := &providerEntry{
			id:   p.ID,
			name: p.Name,
			typ:  ProviderType(p.Type),
		}
		switch entry.typ {
		case ProviderOpenAIModeration:
			var cfg moderationConfig
			if err := json.Unmarshal([]byte(p.Config), &cfg); err != nil {
				warn(logger, fmt.Sprintf("guardrails-runtime: provider %s config parse error: %v", p.ID, err))
				continue
			}
			entry.apiKey = cfg.APIKey
			entry.baseURL = cfg.BaseURL
		case ProviderCustomWebhook:
			var cfg webhookProviderConfig
			if err := json.Unmarshal([]byte(p.Config), &cfg); err != nil {
				warn(logger, fmt.Sprintf("guardrails-runtime: provider %s config parse error: %v", p.ID, err))
				continue
			}
			entry.webhookURL = cfg.URL
		case ProviderRegex:
			// No provider-level config.
		default:
			warn(logger, fmt.Sprintf("guardrails-runtime: skipping provider %s: unsupported type %q", p.ID, p.Type))
			continue
		}
		provs[p.ID] = entry
	}

	out := make([]ruleEntry, 0, len(rules))
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		entry := ruleEntry{
			id:      r.ID,
			name:    r.Name,
			trigger: Trigger(r.Trigger),
			action:  Action(r.Action),
			pattern: r.Pattern,
		}
		if !validTrigger(entry.trigger) || !validAction(entry.action) {
			warn(logger, fmt.Sprintf("guardrails-runtime: skipping rule %s: invalid trigger/action", r.ID))
			continue
		}
		// Bare regex rules (no provider) supported when pattern is non-empty.
		if r.ProviderID == "" {
			re, err := regexp.Compile(r.Pattern)
			if err != nil {
				warn(logger, fmt.Sprintf("guardrails-runtime: skipping rule %s: regex compile: %v", r.ID, err))
				continue
			}
			entry.regex = re
		} else {
			prov, ok := provs[r.ProviderID]
			if !ok {
				warn(logger, fmt.Sprintf("guardrails-runtime: skipping rule %s: provider %s missing/disabled", r.ID, r.ProviderID))
				continue
			}
			entry.provider = prov
			if prov.typ == ProviderRegex {
				re, err := regexp.Compile(r.Pattern)
				if err != nil {
					warn(logger, fmt.Sprintf("guardrails-runtime: skipping rule %s: regex compile: %v", r.ID, err))
					continue
				}
				entry.regex = re
			}
		}
		out = append(out, entry)
	}
	return out
}

func validTrigger(t Trigger) bool {
	return t == TriggerInput || t == TriggerOutput || t == TriggerBoth
}

func validAction(a Action) bool {
	return a == ActionBlock || a == ActionFlag || a == ActionLog
}

func (r *ruleEntry) triggersOnInput() bool {
	return r.trigger == TriggerInput || r.trigger == TriggerBoth
}

func (r *ruleEntry) triggersOnOutput() bool {
	return r.trigger == TriggerOutput || r.trigger == TriggerBoth
}

func warn(logger schemas.Logger, msg string) {
	if logger != nil {
		logger.Warn(msg)
	}
}
