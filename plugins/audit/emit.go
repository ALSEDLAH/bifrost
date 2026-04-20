// audit.Emit — synchronous audit-entry emit helper.
//
// Every enterprise plugin/handler that performs an administrative or
// governance action (create/update/delete of orgs, workspaces, users,
// roles, keys, budgets, guardrails, configs, etc.) calls Emit at the
// completion of that action. Direct admin emits are SYNCHRONOUS so the
// caller can confirm the audit row landed before returning success to
// the operator (Constitution Principle VI).
//
// The helper resolves tenant context from the BifrostContext and
// inserts the row via the audit plugin's underlying *gorm.DB. Errors
// are returned to the caller — admin handlers should surface them
// (HTTP 500) rather than swallow them, because a missing audit entry
// is itself a compliance issue.
//
// Plugin-Init wiring: plugins/audit Init() registers itself as the
// process-default sink via setDefaultSink so Emit() works without
// passing a *Plugin handle around.

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/framework/tenancy"
)

// Entry is the typed shape passed to Emit. Field semantics match
// logstore.TableAuditEntry; this layer abstracts over the storage
// type so callers don't import logstore directly.
type Entry struct {
	// Action is the canonical verb (e.g. "workspace.create",
	// "role.assign", "guardrail.deny"). See contracts/events.md for
	// the full verb list.
	Action string

	// ResourceType + ResourceID identify the affected resource.
	ResourceType string
	ResourceID   string

	// Outcome is one of "allowed", "denied", "error".
	// Defaults to "allowed" if empty.
	Outcome string

	// Reason is free-form text shown when Outcome != allowed.
	Reason string

	// Before / After are JSON-serializable snapshots of the resource's
	// state before and after the action. Either may be nil.
	Before any
	After  any

	// ActorIP overrides the auto-derived IP. Most callers leave empty;
	// the HTTP handler sets it from the request when relevant.
	ActorIP string

	// RequestID overrides the auto-derived request ID.
	RequestID string

	// Workspace override — for audit entries about an action that
	// affects a workspace different from the caller's resolved one.
	WorkspaceIDOverride string
}

// defaultSink is the registered audit plugin instance, set by Init.
// Protected by sinkMu to allow safe re-init in tests.
var (
	sinkMu      sync.RWMutex
	defaultSink *Plugin
)

func setDefaultSink(p *Plugin) {
	sinkMu.Lock()
	defaultSink = p
	sinkMu.Unlock()
}

func clearDefaultSink() {
	sinkMu.Lock()
	defaultSink = nil
	sinkMu.Unlock()
}

func getDefaultSink() *Plugin {
	sinkMu.RLock()
	defer sinkMu.RUnlock()
	return defaultSink
}

// DefaultSink returns the process-default audit plugin instance or
// nil if audit.Init hasn't run. Exposed so HTTP handlers can call
// Verify() without being handed an explicit plugin reference.
func DefaultSink() *Plugin {
	return getDefaultSink()
}

// ErrNoSink is returned by Emit when no audit plugin has been
// registered. In a correctly-configured enterprise deployment this is
// a startup ordering bug (audit must Init before any other enterprise
// plugin). In tests, callers can suppress the error with InitForTest.
var ErrNoSink = fmt.Errorf("audit: no sink registered (audit plugin not initialized)")

// Emit writes an audit entry synchronously. The caller's BifrostContext
// supplies tenant attribution; the entry is rejected with a clear error
// if no TenantContext has been resolved (indicates middleware ordering
// bug — caller should fix the route's middleware chain).
func Emit(ctx context.Context, bctx *schemas.BifrostContext, e Entry) error {
	sink := getDefaultSink()
	if sink == nil {
		return ErrNoSink
	}

	tc, err := tenancy.FromContext(bctx)
	if err != nil {
		return fmt.Errorf("audit.Emit: %w", err)
	}

	row := logstore.TableAuditEntry{
		ID:             uuid.NewString(),
		OrganizationID: tc.OrganizationID,
		WorkspaceID:    tc.WorkspaceID,
		ActorType:      string(tc.ResolvedVia),
		ActorID:        tc.UserID,
		ActorIP:        e.ActorIP,
		Action:         e.Action,
		ResourceType:   e.ResourceType,
		ResourceID:     e.ResourceID,
		Outcome:        defaultOutcome(e.Outcome),
		Reason:         e.Reason,
		RequestID:      e.RequestID,
		CreatedAt:      time.Now().UTC(),
	}
	if e.WorkspaceIDOverride != "" {
		row.WorkspaceID = e.WorkspaceIDOverride
	}
	if e.Before != nil {
		if buf, err := json.Marshal(e.Before); err == nil {
			row.BeforeJSON = string(buf)
		}
	}
	if e.After != nil {
		if buf, err := json.Marshal(e.After); err == nil {
			row.AfterJSON = string(buf)
		}
	}

	// Spec 015: stamp HMAC under the chain mutex so concurrent Emits
	// can't both observe the same predecessor. When the chain is
	// disabled (no key) the fields stay empty.
	if sink.chain.Enabled() {
		sink.seedChainFromDB()
		sink.chain.mu.Lock()
		row.HMAC, row.PrevHMAC = sink.chain.computeHMAC(row.CanonicalBytes())
		sink.chain.mu.Unlock()
	}

	return sink.db.WithContext(ctx).Create(&row).Error
}

// EmitDenied is a convenience for the common "denied" outcome.
func EmitDenied(ctx context.Context, bctx *schemas.BifrostContext, action, resourceType, resourceID, reason string) error {
	return Emit(ctx, bctx, Entry{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      "denied",
		Reason:       reason,
	})
}

// EmitError is a convenience for the "error" outcome (unexpected
// failure, distinct from a permission denial).
func EmitError(ctx context.Context, bctx *schemas.BifrostContext, action, resourceType, resourceID string, opErr error) error {
	return Emit(ctx, bctx, Entry{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      "error",
		Reason:       opErr.Error(),
	})
}

func defaultOutcome(s string) string {
	if s == "" {
		return "allowed"
	}
	return s
}
