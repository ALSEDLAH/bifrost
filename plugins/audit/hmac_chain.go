// HMAC chain for tamper-evident audit entries (spec 015).
//
// Enabled when BIFROST_AUDIT_HMAC_KEY is set (hex or base64). When
// disabled, rows insert with empty HMAC / PrevHMAC — backwards
// compatible with pre-spec-015 deployments.

package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/maximhq/bifrost/framework/logstore"
)

const hmacKeyEnvVar = "BIFROST_AUDIT_HMAC_KEY"

// chainState is held by the Plugin; serialises HMAC computation so
// concurrent emits can't both observe the same predecessor row.
type chainState struct {
	mu       sync.Mutex
	key      []byte // nil when disabled
	lastHMAC string // empty when no rows seeded yet this process
	loaded   bool   // true once we've read the latest row from DB
}

// loadKey reads BIFROST_AUDIT_HMAC_KEY and decodes it as hex first,
// then base64. Returns nil key (disabled) when the var is unset.
func loadHMACKey() ([]byte, error) {
	raw := os.Getenv(hmacKeyEnvVar)
	if raw == "" {
		return nil, nil
	}
	// Try hex first.
	if k, err := hex.DecodeString(raw); err == nil && len(k) >= 16 {
		return k, nil
	}
	// Then base64 (std and raw).
	if k, err := base64.StdEncoding.DecodeString(raw); err == nil && len(k) >= 16 {
		return k, nil
	}
	if k, err := base64.RawStdEncoding.DecodeString(raw); err == nil && len(k) >= 16 {
		return k, nil
	}
	return nil, errors.New("BIFROST_AUDIT_HMAC_KEY must be hex or base64 and decode to ≥16 bytes")
}

// Enabled reports whether the HMAC chain is configured.
func (c *chainState) Enabled() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.key) > 0
}

// seedFromDB reads the latest row's HMAC so the first emit in this
// process picks up where the previous one left off. Called lazily.
func (p *Plugin) seedChainFromDB() {
	c := &p.chain
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return
	}
	if len(c.key) == 0 {
		c.loaded = true
		return
	}
	var row logstore.TableAuditEntry
	err := p.db.Model(&logstore.TableAuditEntry{}).
		Order("created_at DESC, id DESC").
		Limit(1).
		First(&row).Error
	if err == nil && row.HMAC != "" {
		c.lastHMAC = row.HMAC
	}
	// Table empty or error both OK — lastHMAC stays "".
	c.loaded = true
}

// computeHMAC generates the next HMAC for `row` in hex. MUST be called
// with the chain mutex already held. Also advances lastHMAC.
func (c *chainState) computeHMAC(canonical []byte) (hmacHex, prevHex string) {
	prevHex = c.lastHMAC
	mac := hmac.New(sha256.New, c.key)
	if prevHex != "" {
		// Feed the hex of the previous HMAC; avoids hex-decoding it and
		// keeps the canonical bytes well-defined.
		mac.Write([]byte(prevHex))
	}
	mac.Write(canonical)
	sum := mac.Sum(nil)
	hmacHex = hex.EncodeToString(sum)
	c.lastHMAC = hmacHex
	return
}

// Verify walks every row and recomputes the chain. Returns
// (entriesChecked, firstBreakID, firstBreakReason).
func (p *Plugin) Verify(limit int) (int, string, string, error) {
	c := &p.chain
	c.mu.Lock()
	key := c.key
	c.mu.Unlock()
	if len(key) == 0 {
		return 0, "", "no HMAC key configured", nil
	}

	// Stream ordered rows.
	var rows []logstore.TableAuditEntry
	q := p.db.Model(&logstore.TableAuditEntry{}).Order("created_at ASC, id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&rows).Error; err != nil {
		return 0, "", "", fmt.Errorf("audit verify: scan failed: %w", err)
	}

	prev := ""
	for i, row := range rows {
		// Handle rows written before the chain was enabled — empty HMAC
		// passes through and resets the chain. Treat a transition
		// populated→empty→populated as a single rotation boundary.
		if row.HMAC == "" {
			prev = ""
			continue
		}
		mac := hmac.New(sha256.New, key)
		if row.PrevHMAC != "" {
			mac.Write([]byte(row.PrevHMAC))
		}
		mac.Write(row.CanonicalBytes())
		want := hex.EncodeToString(mac.Sum(nil))
		if want != row.HMAC {
			return i + 1, row.ID, "hmac mismatch (tampered or wrong key)", nil
		}
		if prev != "" && row.PrevHMAC != "" && row.PrevHMAC != prev {
			return i + 1, row.ID, fmt.Sprintf("prev_hmac chain break: expected %s got %s", prev, row.PrevHMAC), nil
		}
		prev = row.HMAC
	}
	return len(rows), "", "", nil
}
