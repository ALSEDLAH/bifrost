// Package license implements the enterprise license plugin.
//
// In v1 (this scaffold) the plugin is a no-op default that always returns
// "entitled" so that downstream feature plugins can call IsEntitled()
// safely during Phase 2 development. The real license verification +
// expiry + grace logic lands in Phase 8 Train E (T400-series).
//
// Constitution Principle XI rule 1: this is a new plugin module, not an
// edit to any upstream file.
//
// Wiring (deferred to Phase 2 enterprise-gate plugin):
//   - In selfhosted/airgapped modes, the plugin's Init refuses to start
//     without a valid license file at the configured path.
//   - In cloud mode, the plugin returns "always entitled" without reading
//     a license file.
package license

import (
	"context"
	"sync"
	"time"

	"github.com/maximhq/bifrost/framework/deploymentmode"
)

// PluginName is the registered plugin identifier.
const PluginName = "license"

// LicensePlugin is the public interface that gated feature plugins
// consume via context-key lookup.
type LicensePlugin interface {
	// Name returns the plugin name.
	Name() string

	// IsEntitled reports whether the current license grants access to a
	// named feature. v1 default scaffold always returns true; Phase 8
	// implementation parses the license JWT's "entitlements" claim.
	IsEntitled(feature string) bool

	// DaysUntilExpiry returns days remaining before the license expires.
	// Negative when in grace period; 0 means hard-expired.
	DaysUntilExpiry() int

	// InGracePeriod returns true when the license has expired but the
	// 14-day grace window has not yet elapsed.
	InGracePeriod() bool

	// Cleanup releases resources at shutdown.
	Cleanup() error
}

// noopLicense is the v1 scaffold implementation. It satisfies the
// interface so that gated features depend on the contract today
// without coupling to Phase 8's signature-verification machinery.
type noopLicense struct {
	mu        sync.RWMutex
	startedAt time.Time
}

// Init constructs the v1 no-op license plugin. In selfhosted/airgapped
// mode this constructor will eventually require a license file path; in
// the v1 scaffold it logs a startup banner and returns successfully.
func Init(_ context.Context) (LicensePlugin, error) {
	mode := deploymentmode.Current()
	defaults := deploymentmode.DefaultsFor(mode)

	if defaults.LicenseRequired {
		// TODO(Phase 8 T404+): load the JWT license from
		// config.enterprise.license.path, verify offline against the
		// embedded vendor public-key array, populate entitlements +
		// expiry, refuse boot if invalid. For now (Phase 1.5 scaffold)
		// we proceed in permissive mode so downstream Phase 2
		// development can use IsEntitled() without a real license file.
		_ = defaults
	}

	return &noopLicense{startedAt: time.Now().UTC()}, nil
}

func (n *noopLicense) Name() string                   { return PluginName }
func (n *noopLicense) IsEntitled(feature string) bool { return true }
func (n *noopLicense) DaysUntilExpiry() int           { return 365 }
func (n *noopLicense) InGracePeriod() bool            { return false }
func (n *noopLicense) Cleanup() error                 { return nil }
