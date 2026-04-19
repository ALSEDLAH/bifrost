// Envelope encryption — two-tier KEK + DEK ciphertext layout.
//
// Used when a kms_config row activates BYOK for the entity being
// encrypted. The KEK lives in customer-managed KMS (AWS / Azure / GCP);
// each record gets a fresh AES-256 DEK that's wrapped by the KEK and
// stored alongside the ciphertext.
//
// Ciphertext layout (research R-05):
//
//	┌─────────────────┬───────────────────┬──────────────────┬─────────┬─────────────┐
//	│ version (1 byte)│ kek_ref_hash (32) │ dek_wrapped (var)│ nonce 12│ ct + tag    │
//	└─────────────────┴───────────────────┴──────────────────┴─────────┴─────────────┘
//
// version starts at 1. kek_ref_hash is SHA-256(kek_ref) so we can pick
// the right KEK on decrypt without storing the full ARN. dek_wrapped
// length is determined by the KMS provider's response; we store it as
// a length-prefixed varint to keep the layout self-describing.
//
// This file defines the layout primitives only — the actual KEK calls
// live in framework/kms/{aws,azure,gcp}.go (Phase 8 T248).
//
// Constitution Principle VII + research R-05.
package crypto

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// EnvelopeVersion is the current ciphertext layout version. Bump only on
// breaking format changes; readers must accept the previous version
// during the transition (R-05 KEK rotation flow).
const EnvelopeVersion byte = 1

// MinEnvelopeSize is the minimum byte length of a valid envelope:
// version(1) + kek_ref_hash(32) + dek_wrapped_len_varint(>=1) + nonce(12) + ct+tag(>=16)
const MinEnvelopeSize = 1 + 32 + 1 + 12 + 16

// EnvelopeHeader holds the parsed header of an envelope ciphertext, used
// by the KMS layer to look up the correct KEK and unwrap the DEK.
type EnvelopeHeader struct {
	Version    byte
	KEKRefHash [32]byte
	DEKWrapped []byte
	Nonce      [12]byte
	Ciphertext []byte // includes the GCM auth tag
}

// HashKEKRef returns SHA-256(kek_ref). kek_ref is the AWS ARN, Azure
// keyvault URI, or GCP resource name; we store only the hash inline so
// the full identifier remains in the kms_configs table.
func HashKEKRef(kekRef string) [32]byte {
	return sha256.Sum256([]byte(kekRef))
}

// PackEnvelope assembles the final ciphertext byte slice from its parts.
// dekWrapped, nonce, and ct must be supplied by the caller (KMS-Encrypt
// for dekWrapped, crypto/rand for nonce, AES-GCM for ct).
func PackEnvelope(kekRefHash [32]byte, dekWrapped []byte, nonce [12]byte, ct []byte) ([]byte, error) {
	if len(dekWrapped) == 0 {
		return nil, errors.New("crypto: dekWrapped is empty")
	}
	if len(ct) < 16 {
		return nil, errors.New("crypto: ct must include the GCM auth tag (>=16 bytes)")
	}

	out := make([]byte, 0, 1+32+10+len(dekWrapped)+12+len(ct))
	out = append(out, EnvelopeVersion)
	out = append(out, kekRefHash[:]...)

	// length-prefixed dekWrapped (varint)
	var lenBuf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(lenBuf[:], uint64(len(dekWrapped)))
	out = append(out, lenBuf[:n]...)
	out = append(out, dekWrapped...)

	out = append(out, nonce[:]...)
	out = append(out, ct...)
	return out, nil
}

// UnpackEnvelope parses an envelope ciphertext into its header parts.
func UnpackEnvelope(envelope []byte) (*EnvelopeHeader, error) {
	if len(envelope) < MinEnvelopeSize {
		return nil, fmt.Errorf("crypto: envelope too short (%d bytes, need >= %d)", len(envelope), MinEnvelopeSize)
	}
	if envelope[0] != EnvelopeVersion {
		return nil, fmt.Errorf("crypto: unsupported envelope version %d (this build supports %d)", envelope[0], EnvelopeVersion)
	}

	h := &EnvelopeHeader{Version: envelope[0]}
	pos := 1

	copy(h.KEKRefHash[:], envelope[pos:pos+32])
	pos += 32

	dekLen, varintN := binary.Uvarint(envelope[pos:])
	if varintN <= 0 {
		return nil, errors.New("crypto: malformed dekWrapped length varint")
	}
	pos += varintN
	if uint64(pos)+dekLen+12+16 > uint64(len(envelope)) {
		return nil, errors.New("crypto: envelope truncated (dekWrapped + nonce + ct mismatch)")
	}
	h.DEKWrapped = envelope[pos : pos+int(dekLen)]
	pos += int(dekLen)

	copy(h.Nonce[:], envelope[pos:pos+12])
	pos += 12

	h.Ciphertext = envelope[pos:]
	return h, nil
}
