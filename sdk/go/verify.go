// Package avp verifies an AgenticVerificationProof v1 (AVP) — the signed,
// consensus-backed on-chain fact receipt issued by Tessera. Verification needs
// nothing but the proof JSON: it recomputes the RFC 8785 canonical preimage
// (the proof minus its `attestation`) and recovers the EIP-191 signer.
//
// A valid signature vouches for every other field, INCLUDING a verified:false
// verdict — a signed negative is an equally trustworthy result.
package avp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Version is the only AVP schema tag this library validates.
const Version = "avp/1.0"

// Result is the outcome of verifying a proof.
type Result struct {
	OK        bool   // recovered address == attestation.signer
	Recovered string // address recovered from the signature (checksummed)
	Signer    string // address the proof claims signed it
	Reason    string // set when OK is false
}

// Verify checks a proof's EIP-191 signature over its RFC 8785 canonical bytes.
// By default it also requires schemaVersion == Version; pass requireVersion=false
// to skip that (e.g. to inspect a future minor version's signature).
func Verify(proofJSON []byte, requireVersion bool) (Result, error) {
	var m map[string]any
	if err := json.Unmarshal(proofJSON, &m); err != nil {
		return Result{}, fmt.Errorf("parse proof: %w", err)
	}

	att, ok := m["attestation"].(map[string]any)
	if !ok {
		return Result{}, fmt.Errorf("proof has no attestation object")
	}
	signer, _ := att["signer"].(string)
	sig, _ := att["signature"].(string)

	if requireVersion {
		if v, _ := m["schemaVersion"].(string); v != Version {
			return Result{Signer: signer, Reason: fmt.Sprintf("unknown AVP version: %v", m["schemaVersion"])}, nil
		}
	}

	// Preimage = canonical JSON of the proof with `attestation` removed.
	delete(m, "attestation")
	var buf bytes.Buffer
	if err := writeCanonical(&buf, m); err != nil {
		return Result{Signer: signer}, err
	}

	recovered, err := recoverEIP191(buf.Bytes(), sig)
	if err != nil {
		return Result{Signer: signer, Reason: fmt.Sprintf("recover failed: %v", err)}, nil
	}
	if strings.EqualFold(recovered, signer) {
		return Result{OK: true, Recovered: recovered, Signer: signer}, nil
	}
	return Result{Recovered: recovered, Signer: signer, Reason: fmt.Sprintf("recovered %s != signer %s", recovered, signer)}, nil
}

// recoverEIP191 recovers the address that produced an EIP-191 (personal_sign)
// signature over msg.
func recoverEIP191(msg []byte, sigHex string) (string, error) {
	sig := common.FromHex(sigHex)
	if len(sig) != 65 {
		return "", fmt.Errorf("signature is %d bytes, want 65", len(sig))
	}
	// Normalize recovery id from {27,28} to {0,1} if needed.
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	hash := accounts.TextHash(msg) // "\x19Ethereum Signed Message:\n"+len(msg)+msg, keccak256
	pub, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return "", err
	}
	return crypto.PubkeyToAddress(*pub).Hex(), nil
}

// writeCanonical emits a JSON value with object keys sorted lexicographically and
// no insignificant whitespace — the RFC 8785 shape the issuer signed. It mirrors
// the reference issuer's canonicalizer exactly so signatures cross-validate.
func writeCanonical(buf *bytes.Buffer, v any) error {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			if err := writeCanonical(buf, val[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	case []any:
		buf.WriteByte('[')
		for i, e := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	}
}
