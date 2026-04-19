// Package deploymentmode resolves and exposes the deployment-mode flag
// (cloud / selfhosted / airgapped) plus its opinionated defaults.
//
// Constitution Principle XI rule 4 — single mode flag drives a table of
// per-feature defaults so operators don't have to flip five flags
// independently.
//
// Sets at boot from config; immutable thereafter.
package deploymentmode

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Mode is the canonical deployment-mode enum.
type Mode string

const (
	// Cloud — vendor-hosted SaaS. Multi-org, metering + billing enabled,
	// telemetry on by default, no license file required.
	Cloud Mode = "cloud"

	// SelfHosted — customer on-prem deployment. Single-org, license-key
	// gated, no phone-home by default, no metering / billing.
	SelfHosted Mode = "selfhosted"

	// AirGapped — customer on-prem with no outbound network calls allowed.
	// Same as SelfHosted minus phone-home (which is permanently OFF and
	// enforced) and limited to the MVP feature subset per spec FR-037.
	AirGapped Mode = "airgapped"
)

// Defaults captures the opinionated configuration derived from a mode.
// Operators may override individual fields via explicit config; mode is
// the seed.
type Defaults struct {
	MultiOrgEnabled       bool
	PhoneHomeEnabled      bool
	PhoneHomePermitted    bool // false = ANY phone-home setting is rejected at validation time
	LicenseRequired       bool
	MeteringEnabled       bool
	BillingEnabled        bool
	SAMLPermitted         bool // false in airgapped (OIDC only)
	AirGapped             bool
}

var (
	current Mode
	once    sync.Once
	mu      sync.RWMutex
)

// Parse converts a string from config (case-insensitive) into a Mode.
// Empty string defaults to SelfHosted (the safer choice for unknown
// deployments).
func Parse(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(SelfHosted):
		return SelfHosted, nil
	case string(Cloud):
		return Cloud, nil
	case string(AirGapped):
		return AirGapped, nil
	default:
		return "", fmt.Errorf("deploymentmode: unknown mode %q (want cloud|selfhosted|airgapped)", s)
	}
}

// Set fixes the deployment mode for the lifetime of the process. Safe
// to call once. Subsequent calls with a different value return an error.
func Set(m Mode) error {
	mu.Lock()
	defer mu.Unlock()

	if current == "" {
		current = m
		return nil
	}
	if current == m {
		return nil
	}
	return fmt.Errorf("deploymentmode: already set to %q, cannot change to %q", current, m)
}

// Current returns the active mode. Returns SelfHosted if Set was never
// called (safe default for unit tests and bootstrapped contexts).
func Current() Mode {
	mu.RLock()
	defer mu.RUnlock()
	if current == "" {
		return SelfHosted
	}
	return current
}

// DefaultsFor returns the opinionated default config for a mode.
func DefaultsFor(m Mode) Defaults {
	switch m {
	case Cloud:
		return Defaults{
			MultiOrgEnabled:    true,
			PhoneHomeEnabled:   true,
			PhoneHomePermitted: true,
			LicenseRequired:    false,
			MeteringEnabled:    true,
			BillingEnabled:     true,
			SAMLPermitted:      true,
			AirGapped:          false,
		}
	case SelfHosted:
		return Defaults{
			MultiOrgEnabled:    false,
			PhoneHomeEnabled:   false, // off by default; operator may opt in
			PhoneHomePermitted: true,
			LicenseRequired:    true,
			MeteringEnabled:    false,
			BillingEnabled:     false,
			SAMLPermitted:      true,
			AirGapped:          false,
		}
	case AirGapped:
		return Defaults{
			MultiOrgEnabled:    false,
			PhoneHomeEnabled:   false,
			PhoneHomePermitted: false, // any phone-home config rejected at validation
			LicenseRequired:    true,
			MeteringEnabled:    false,
			BillingEnabled:     false,
			SAMLPermitted:      false, // OIDC only per FR-037
			AirGapped:          true,
		}
	}
	return Defaults{}
}

// Defaults returns the opinionated defaults for the currently active mode.
func CurrentDefaults() Defaults {
	return DefaultsFor(Current())
}

// ErrPhoneHomeForbidden is returned by Validate when an air-gapped
// deployment attempts to enable phone-home telemetry.
var ErrPhoneHomeForbidden = errors.New("deploymentmode: phone-home telemetry is forbidden in airgapped mode")
