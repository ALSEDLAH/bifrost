// Audit bridge for guardrail outcomes (spec 016 T009).
//
// Every block + flag fires an audit.Emit so the compliance trail
// captures the decision. Log-only rules also emit (as outcome=log)
// so operators can prove coverage when investigating a leak.
//
// The bridge is indirection over audit.Emit so the plugin compiles
// cleanly in unit tests that don't stand up a real audit sink.

package guardrailsruntime

import (
	"context"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/plugins/audit"
)

// auditEmitter is the function the plugin calls to record an audit
// event. In production it wraps audit.Emit; in tests a fake can be
// injected via SetAuditEmitter.
type auditEmitter func(ctx context.Context, bctx *schemas.BifrostContext, e audit.Entry) error

// SetAuditEmitter swaps the bridge used by block/flag paths.
// Passing nil restores the default (audit.Emit).
func (p *Plugin) SetAuditEmitter(fn auditEmitter) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.emit = fn
	p.mu.Unlock()
}

// emitAudit records a guardrail outcome. Safe on nil plugin or
// when the audit plugin hasn't been initialised yet — the
// default emitter returns ErrNoSink which we log+swallow so the
// inference path is never held up by audit-sink availability.
func (p *Plugin) emitAudit(
	bctx *schemas.BifrostContext,
	r *ruleEntry,
	triggerName string,
	outcome string,
	reason string,
) {
	if p == nil {
		return
	}
	p.mu.RLock()
	fn := p.emit
	p.mu.RUnlock()
	if fn == nil {
		fn = audit.Emit
	}
	err := fn(context.Background(), bctx, audit.Entry{
		Action:       "guardrail." + outcome, // e.g. guardrail.block / guardrail.flag / guardrail.log
		ResourceType: "guardrail_rule",
		ResourceID:   r.id,
		Outcome:      outcomeToAuditString(outcome),
		Reason:       reason,
		RequestID:    reqIDFromCtx(bctx),
		After: map[string]any{
			"rule_name": r.name,
			"trigger":   triggerName,
			"action":    string(r.action),
			"matched_at": time.Now().UTC(),
		},
	})
	if err != nil && p.logger != nil {
		// Missing sink is expected during unit tests; don't spam Warn.
		p.logger.Debug("guardrails-runtime: audit.Emit: %v", err)
	}
}

func outcomeToAuditString(outcome string) string {
	switch outcome {
	case "block":
		return "denied"
	case "flag", "log":
		return "allowed"
	default:
		return "allowed"
	}
}

func reqIDFromCtx(bctx *schemas.BifrostContext) string {
	if bctx == nil {
		return ""
	}
	if v, ok := bctx.Value(schemas.BifrostContextKeyRequestID).(string); ok {
		return v
	}
	return ""
}
