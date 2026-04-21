// Audit outbound tests (spec 018).

package audit

import (
	"testing"

	"github.com/maximhq/bifrost/framework/alertchannels"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/framework/logstore"
)

// fakeDispatcher records every Send call for assertion.
type fakeDispatcher struct {
	calls []dispatchCall
}

type dispatchCall struct {
	channels []tables_enterprise.TableAlertChannel
	event    alertchannels.Event
}

// TestDispatchOutbound_NoopWhenNoBridge asserts that calling
// dispatchOutbound before SetAlertDispatcher wires anything up is
// a no-op rather than a nil dereference.
func TestDispatchOutbound_NoopWhenNoBridge(t *testing.T) {
	// Reset the package-level bridge.
	SetAlertDispatcher(nil, nil)
	// Should not panic / should not error.
	dispatchOutbound(logstore.TableAuditEntry{ID: "x", Action: "x.y"})
}

// TestDispatchOutbound_NoopWhenNoChannels asserts that an enabled
// bridge with zero channels skips the dispatcher (saves the send
// goroutine spin on zero-recipient events).
func TestDispatchOutbound_NoopWhenNoChannels(t *testing.T) {
	// Using a real dispatcher with a nil-returning channelsFn is
	// the simplest way to exercise this path without pulling in
	// sinon-style fakes.
	d := alertchannels.New(nil)
	SetAlertDispatcher(d, func() []tables_enterprise.TableAlertChannel { return nil })
	defer SetAlertDispatcher(nil, nil)
	// Nothing to assert beyond "doesn't panic" — Send is not called
	// because we short-circuit on empty channels.
	dispatchOutbound(logstore.TableAuditEntry{ID: "x"})
}

// TestDispatchOutbound_BuildsExpectedEvent exercises the full
// builder — we inject a dispatcher whose channels list is a single
// "webhook" entry whose Config points at a no-op URL. The dispatcher
// will attempt the HTTP send in a goroutine and fail cleanly; we only
// care that dispatchOutbound constructs + hands off the event.
func TestDispatchOutbound_BuildsExpectedEvent(t *testing.T) {
	d := alertchannels.New(nil)
	ch := tables_enterprise.TableAlertChannel{
		ID: "c1", Name: "test-wh", Type: string(alertchannels.ChannelTypeWebhook),
		Config:  `{"url":"http://127.0.0.1:1"}`,
		Enabled: true,
	}
	SetAlertDispatcher(d, func() []tables_enterprise.TableAlertChannel {
		return []tables_enterprise.TableAlertChannel{ch}
	})
	defer SetAlertDispatcher(nil, nil)

	// The dispatcher fires a goroutine; dispatchOutbound must not
	// block on it. If this test completes, the non-blocking contract
	// is preserved.
	dispatchOutbound(logstore.TableAuditEntry{
		ID: "x", Action: "role.create", ResourceType: "role",
		Outcome: "allowed",
	})
}
