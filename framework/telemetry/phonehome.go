// Package telemetry — phone-home gate.
//
// Constitution Principle XI rule 6 (and FR-037): air-gapped deployments
// are forbidden from making any outbound call to vendor infrastructure.
// Self-hosted deployments default phone-home OFF; opt-in via config.
// Cloud deployments default ON (the vendor operates them).
//
// This file is the policy gate. Actual emit logic (when permitted) lives
// in a separate sender that respects PhoneHomeAllowed().
package telemetry

import (
	"errors"
	"sync"

	"github.com/maximhq/bifrost/framework/deploymentmode"
)

// Config holds the runtime phone-home settings sourced from
// config.enterprise.deployment.telemetry.
type Config struct {
	// PhoneHomeOptIn is the operator's explicit choice from config.
	// Ignored in airgapped mode (the gate forbids it regardless).
	PhoneHomeOptIn bool
}

var (
	cfgMu sync.RWMutex
	cfg   Config
)

// SetConfig is called once at boot after deployment mode is resolved.
// Returns ErrPhoneHomeForbidden if airgapped mode is active and the
// operator nonetheless tried to enable phone-home.
func SetConfig(c Config) error {
	mode := deploymentmode.Current()
	defaults := deploymentmode.DefaultsFor(mode)

	if !defaults.PhoneHomePermitted && c.PhoneHomeOptIn {
		return deploymentmode.ErrPhoneHomeForbidden
	}

	cfgMu.Lock()
	cfg = c
	cfgMu.Unlock()
	return nil
}

// PhoneHomeAllowed returns true when the deployment is permitted to send
// anonymous version-ping telemetry to vendor infrastructure.
//
// Decision matrix:
//
//	cloud      -> always true (vendor-operated)
//	selfhosted -> true only when operator opts in via config
//	airgapped  -> always false (forbidden by mode)
func PhoneHomeAllowed() bool {
	mode := deploymentmode.Current()
	defaults := deploymentmode.DefaultsFor(mode)

	if !defaults.PhoneHomePermitted {
		return false // airgapped: hard NO
	}

	cfgMu.RLock()
	defer cfgMu.RUnlock()

	if mode == deploymentmode.Cloud {
		return true // cloud: always YES (vendor's deployment)
	}
	// selfhosted: opt-in
	return cfg.PhoneHomeOptIn
}

// AssertNotEgressing is a defensive runtime check used by the air-gapped
// smoke test (research R-06). When true (default in airgapped mode) the
// telemetry sender will refuse to construct any HTTP client.
func AssertNotEgressing() error {
	if !PhoneHomeAllowed() {
		return nil
	}
	if deploymentmode.Current() == deploymentmode.AirGapped {
		return errors.New("telemetry: airgapped mode forbids outbound telemetry")
	}
	return nil
}
