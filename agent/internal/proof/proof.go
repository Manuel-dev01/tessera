// Package proof defines the AgenticVerificationProof v1 (AVP) output format and
// helpers to build and canonicalize it. The structs mirror
// /schema/agentic-verification-proof-v1.json exactly.
package proof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SchemaVersion is the AVP version tag. Verifiers must reject unknown versions.
const SchemaVersion = "avp/1.0"

// Placeholder attestation used in Phase 0 before the oracle signing key is wired
// (Phase 1). The zero address and zero signature are structurally valid but
// carry no cryptographic weight.
const (
	placeholderSigner = "0x0000000000000000000000000000000000000000"
	placeholderSig    = "0x" +
		"0000000000000000000000000000000000000000000000000000000000000000" + // r
		"0000000000000000000000000000000000000000000000000000000000000000" + // s
		"00" // v
)

// Consensus summarizes multi-source agreement. `sources` is the number of
// independent sources configured; `responders` answered in time; `agreed` voted
// for the signed value; `quorum` is the (dynamic) threshold that had to be met;
// `agreedSources` names the responders that agreed (transparency + challenges).
type Consensus struct {
	Sources       int      `json:"sources"`
	Responders    int      `json:"responders"`
	Agreed        int      `json:"agreed"`
	Quorum        int      `json:"quorum"`
	AgreedSources []string `json:"agreedSources"`
}

// Finality records how settled the block is. A proof is only verified:true when
// the block is final (or ≥ the configured confirmations), protecting against a
// reorg reverting a signed — and later bonded — fact.
type Finality struct {
	Finalized      bool   `json:"finalized"`
	FinalizedBlock uint64 `json:"finalizedBlock"`
	Confirmations  uint64 `json:"confirmations"`
}

// Attestation is the signature over the canonical proof (all fields but this one).
type Attestation struct {
	Signer    string `json:"signer"`
	Scheme    string `json:"scheme"` // "EIP-191"
	Signature string `json:"signature"`
}

// MerkleProof is a transactions-trie inclusion proof (Phase 6): standalone
// cryptographic evidence that the tx sits at txIndex in the block, checkable
// against the block header's transactionsRoot with no trust in the oracle. Nodes
// and leaf are 0x-hex; the leaf is the tx's consensus encoding and keccak256(leaf)
// must equal txHash.
type MerkleProof struct {
	Type             string   `json:"type"`             // "transactionsTrie"
	TransactionsRoot string   `json:"transactionsRoot"` // block header field this proof roots to
	TxIndex          uint64   `json:"txIndex"`
	Key              string   `json:"key"`   // 0x RLP(txIndex) — the trie key
	Nodes            []string `json:"nodes"` // 0x MPT proof nodes, root -> leaf
	Leaf             string   `json:"leaf"`  // 0x tx consensus encoding (the trie value)
}

// Bond is the on-chain honesty stake backing a proof (Phase 3).
//   - contract/stakedUSDC/challengeWindowSec advertise the oracle's STANDING bond.
//   - proofId identifies the core verified fact (keccak256 of the canonical proof
//     with `attestation` AND `bond` removed — see CanonicalCoreBytes).
//   - anchored/anchorTx are set when THIS proof was individually anchored on-chain
//     (earmarking part of the standing bond as trustlessly slashable).
type Bond struct {
	Contract           string  `json:"contract"`
	StakedUSDC         string  `json:"stakedUSDC"`
	ChallengeWindowSec int     `json:"challengeWindowSec"`
	ProofID            string  `json:"proofId"`
	Anchored           bool    `json:"anchored"`
	AnchorTx           *string `json:"anchorTx"`
}

// Proof is the AgenticVerificationProof v1 object. Nullable JSON fields use
// pointers so they serialize as `null` rather than a zero value.
type Proof struct {
	SchemaVersion string           `json:"schemaVersion"`
	Verified      bool             `json:"verified"`
	ChainID       int64            `json:"chainId"`
	BlockNumber   *int64           `json:"blockNumber"`
	BlockHash     *string          `json:"blockHash"`
	TxHash        string           `json:"txHash"`
	TxIndex       *int64           `json:"txIndex"`
	Consensus     Consensus        `json:"consensus"`
	Finality      *Finality        `json:"finality"`
	MerkleProof   *MerkleProof     `json:"merkleProof"`
	Attestation   Attestation      `json:"attestation"`
	Bond          *Bond            `json:"bond"`
	Reason        *string          `json:"reason"`
	IssuedAt      int64            `json:"issuedAt"`
}

// BuildEcho constructs the Phase 0 echo proof: a structurally valid AVP that
// simply asserts verified:true for the requested tx, with a single-source
// consensus and a placeholder (unsigned) attestation. Real RPC verification and
// EIP-191 signing arrive in Phase 1.
func BuildEcho(chainID int64, txHash string, blockNumber int64, issuedAt int64) Proof {
	bn := blockNumber
	return Proof{
		SchemaVersion: SchemaVersion,
		Verified:      true,
		ChainID:       chainID,
		BlockNumber:   &bn,
		BlockHash:     nil,
		TxHash:        txHash,
		TxIndex:       nil,
		Consensus:     Consensus{Sources: 1, Responders: 1, Agreed: 1, Quorum: 1},
		MerkleProof:   nil,
		Attestation: Attestation{
			Signer:    placeholderSigner,
			Scheme:    "EIP-191",
			Signature: placeholderSig,
		},
		Bond:     nil,
		Reason:   nil,
		IssuedAt: issuedAt,
	}
}

// Fields are the verified facts a proof is assembled from. Block fields are
// pointers so an unlocated tx / failed verification serializes them as null.
type Fields struct {
	ChainID     int64
	TxHash      string
	Verified    bool
	BlockNumber *uint64
	BlockHash   string // "" -> null
	TxIndex     *uint
	Reason      *string
	IssuedAt    int64
	Consensus   Consensus
	Finality    *Finality
	Bond        *Bond
	MerkleProof *MerkleProof
}

// Build assembles an AVP from verified facts and the consensus/finality summary.
// The attestation is a placeholder until a Signer fills it.
func Build(f Fields) Proof {
	p := Proof{
		SchemaVersion: SchemaVersion,
		Verified:      f.Verified,
		ChainID:       f.ChainID,
		TxHash:        f.TxHash,
		Consensus:     f.Consensus,
		Finality:      f.Finality,
		MerkleProof:   f.MerkleProof,
		Attestation:   Attestation{Signer: placeholderSigner, Scheme: "EIP-191", Signature: placeholderSig},
		Bond:          f.Bond,
		Reason:        f.Reason,
		IssuedAt:      f.IssuedAt,
	}
	if f.BlockNumber != nil {
		bn := int64(*f.BlockNumber)
		p.BlockNumber = &bn
	}
	if f.BlockHash != "" {
		bh := f.BlockHash
		p.BlockHash = &bh
	}
	if f.TxIndex != nil {
		ti := int64(*f.TxIndex)
		p.TxIndex = &ti
	}
	return p
}

// JSON returns the pretty-printed proof for display/delivery.
func (p Proof) JSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// CanonicalBytes returns the RFC 8785-style canonical byte form of the proof
// with the `attestation` member removed — the exact bytes that get EIP-191
// signed in Phase 1 and that a verifier must recompute. Keys are sorted
// lexicographically at every level with no insignificant whitespace.
//
// Producer and consumer MUST agree on these bytes or signatures won't validate.
func CanonicalBytes(p Proof) ([]byte, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	delete(m, "attestation")

	var buf bytes.Buffer
	if err := writeCanonical(&buf, m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// CanonicalCoreBytes is the canonical form with BOTH `attestation` and `bond`
// removed — the stable identity of the verified fact. proofId = keccak256 of this.
// It must exclude `bond` because bond carries proofId itself (a circular hash
// otherwise), and because a proof's identity should not change when it is bonded.
func CanonicalCoreBytes(p Proof) ([]byte, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	delete(m, "attestation")
	delete(m, "bond")

	var buf bytes.Buffer
	if err := writeCanonical(&buf, m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeCanonical emits a value with object keys sorted and no extra whitespace.
// Numbers/strings/bools/null are delegated to encoding/json's canonical scalar
// forms via json.Marshal.
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
		b, err := marshalNoHTMLEscape(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	}
}

// marshalNoHTMLEscape serializes a scalar without Go's default HTML escaping of
// <, >, and &. RFC 8785 (and the reference JS `canonicalize`) do NOT escape those,
// so escaping them here would make the signed preimage diverge from what verifiers
// recompute — breaking the signature for any proof whose strings contain them
// (e.g. a reason like "need >= 7").
func marshalNoHTMLEscape(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encoder.Encode appends a trailing newline; strip it.
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// String renders the canonical bytes as a string, for logging/debugging.
func CanonicalString(p Proof) (string, error) {
	b, err := CanonicalBytes(p)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(string(b), "{") {
		return "", fmt.Errorf("unexpected canonical form: %s", b)
	}
	return string(b), nil
}
