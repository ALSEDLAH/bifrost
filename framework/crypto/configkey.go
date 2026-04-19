// Package crypto unifies at-rest encryption for enterprise data.
//
// Two backends:
//   - configkey (this file): wraps upstream's framework/encrypt, which
//     Bifrost has already initialized at boot with the configstore's
//     encryption_key. Default for selfhosted/airgapped deployments
//     without BYOK active.
//   - envelope (envelope.go): two-tier KEK + DEK ciphertext layout for
//     BYOK / KMS-backed encryption (R-05). Used when a kms_config row
//     applies to the entity being encrypted.
//
// The Encryptor interface is identical across backends so callers (audit
// emit, license storage, prompt content, BYOK plugin) don't need to know
// which backend is active.
//
// Constitution Principle VII (security-by-default) + research R-05.
package crypto

import (
	"context"

	"github.com/maximhq/bifrost/framework/encrypt"
)

// Encryptor is the unified contract used by enterprise plugins.
type Encryptor interface {
	// Encrypt encodes plaintext into a self-contained ciphertext byte
	// slice that includes any envelope metadata needed for Decrypt.
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)

	// Decrypt reverses Encrypt. Returns an error if the ciphertext is
	// malformed, the wrapping key is unavailable, or the underlying
	// authentication fails.
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)

	// BackendName returns "configkey" or "envelope:<provider>" for logs.
	BackendName() string
}

// ConfigKeyEncryptor delegates to upstream's package-global encrypt
// helpers. Bifrost calls encrypt.Init(rawKey, logger) at boot from the
// configstore's encryption_key; this wrapper assumes that has happened.
type ConfigKeyEncryptor struct{}

// NewConfigKeyEncryptor returns the singleton wrapper. There is no
// per-instance state — the underlying encrypt package owns the key.
func NewConfigKeyEncryptor() *ConfigKeyEncryptor {
	return &ConfigKeyEncryptor{}
}

// Encrypt delegates to upstream encrypt.Encrypt. Returns ciphertext
// as bytes (decoded from the upstream string return for binary callers).
// If upstream encrypt is not enabled (encrypt.IsEnabled()==false), returns
// the plaintext unchanged — matches upstream's pass-through behavior.
func (c *ConfigKeyEncryptor) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	if !encrypt.IsEnabled() {
		return plaintext, nil
	}
	out, err := encrypt.Encrypt(string(plaintext))
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// Decrypt delegates to upstream encrypt.Decrypt with the inverse coercion.
func (c *ConfigKeyEncryptor) Decrypt(_ context.Context, ciphertext []byte) ([]byte, error) {
	if !encrypt.IsEnabled() {
		return ciphertext, nil
	}
	out, err := encrypt.Decrypt(string(ciphertext))
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// BackendName returns "configkey".
func (c *ConfigKeyEncryptor) BackendName() string { return "configkey" }
