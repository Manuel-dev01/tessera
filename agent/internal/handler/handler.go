// Package handler holds Tessera's order-processing logic, independent of the CAP
// transport. A Transport (real CROO or the loopback harness) invokes an
// OrderHandler with the requester's Requirements JSON and delivers whatever
// proof bytes it returns.
package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"tessera/agent/internal/consensus"
	"tessera/agent/internal/proof"
	"tessera/agent/internal/sign"
	"tessera/agent/internal/verify"
)

// OrderHandler processes a paid order's Requirements and returns the deliverable
// (an AVP v1 proof, marshaled JSON). It must not know which transport called it.
type OrderHandler func(ctx context.Context, requirements json.RawMessage) (deliverable json.RawMessage, err error)

// Fetcher is the consensus dependency the handler needs — satisfied by
// *consensus.Engine, and by fakes in tests.
type Fetcher interface {
	Fetch(ctx context.Context, txHash string) consensus.Outcome
}

// Anchorer anchors a proof on-chain (TesseraBond). Satisfied by *bond.Client.
type Anchorer interface {
	Anchor(ctx context.Context, proofID [32]byte, blockNumber, slashAmount *big.Int, claimedBlockHash [32]byte) (txHash string, err error)
}

// BondOpts configures how proofs advertise / commit to the honesty bond.
type BondOpts struct {
	Enabled            bool     // stamp the standing-bond field on verified proofs
	Contract           string   // TesseraBond address
	StakedUSDC         string   // display: oracle's standing stake
	ChallengeWindowSec int      // display + on-chain window
	Anchorer           Anchorer // nil disables on-chain anchoring
	SlashAmount        *big.Int // earmark per anchored proof
	Sync               bool     // anchor before delivering (hero demo) vs async
}

// nowUnix is injectable so behavior is deterministic in tests. Defaults to
// wall-clock seconds.
var nowUnix = func() int64 { return time.Now().Unix() }

// NewVerifyHandler is the Phase 2 handler: it gathers multi-source consensus on
// the tx, verifies the requester's claim against the agreed facts, enforces the
// finality gate, builds the AVP, and signs it (EIP-191). A verification/consensus
// FAILURE is a signed verified:false proof, not an error — only malformed input
// returns a non-nil error.
func NewVerifyHandler(engine Fetcher, v *verify.Verifier, signer *sign.Signer, bondOpts BondOpts) OrderHandler {
	return func(ctx context.Context, requirements json.RawMessage) (json.RawMessage, error) {
		var in verify.Input
		if err := json.Unmarshal(requirements, &in); err != nil {
			return nil, fmt.Errorf("invalid requirements JSON: %w", err)
		}
		if in.TxHash == "" {
			return nil, fmt.Errorf("requirements missing txHash")
		}
		if in.ChainID == 0 {
			in.ChainID = 8453 // Base mainnet default
		}

		out := engine.Fetch(ctx, in.TxHash)
		p, err := AssembleProof(ctx, in, out, v, signer, bondOpts)
		if err != nil {
			return nil, err
		}
		out2, err := p.JSON()
		if err != nil {
			return nil, fmt.Errorf("marshal proof: %w", err)
		}
		return out2, nil
	}
}

// AssembleProof turns a consensus Outcome into a signed AVP: it applies the
// claim rules against the agreed facts, enforces the finality gate, stamps
// consensus/finality/bond, and signs (EIP-191). Shared by the CAP handler and
// the HTTP API so both produce byte-identical proofs.
func AssembleProof(ctx context.Context, in verify.Input, out consensus.Outcome, v *verify.Verifier, signer *sign.Signer, bondOpts BondOpts) (proof.Proof, error) {
	var res verify.Result
	var finality *proof.Finality
	switch {
	case !out.Reached:
		res = verify.Result{Verified: false, Reason: out.Reason}
	default:
		res = v.Verify(in, out.Receipt)
		fin := out.Finality
		finality = &fin
		if res.Verified && !out.FinalOK {
			reason := fmt.Sprintf("block %d not finalized (finalized head %d)", res.BlockNumber, out.Finality.FinalizedBlock)
			res = verify.Result{Verified: false, Reason: &reason}
		}
	}

	fields := toFields(in, res, nowUnix())
	fields.Consensus = proof.Consensus{
		Sources:       out.Sources,
		Responders:    out.Responders,
		Agreed:        out.Agreed,
		Quorum:        out.Quorum,
		AgreedSources: out.AgreedSources,
	}
	fields.Finality = finality
	if bondOpts.Enabled && res.Verified {
		fields.Bond = buildBond(ctx, bondOpts, fields, res)
	}

	p := proof.Build(fields)
	if err := signer.Sign(&p); err != nil {
		return proof.Proof{}, fmt.Errorf("sign proof: %w", err)
	}
	return p, nil
}

// buildBond stamps the standing-bond field and, if an Anchorer is configured,
// anchors this proof on-chain (sync = before delivery; async = fire-and-forget).
func buildBond(ctx context.Context, o BondOpts, fields proof.Fields, res verify.Result) *proof.Bond {
	// Compute the stable proofId from the proof's core (bond/attestation excluded).
	core := proof.Build(fields)
	pidBytes, err := bondProofID(core)
	b := &proof.Bond{
		Contract:           o.Contract,
		StakedUSDC:         o.StakedUSDC,
		ChallengeWindowSec: o.ChallengeWindowSec,
		Anchored:           false,
	}
	if err == nil {
		b.ProofID = "0x" + hex.EncodeToString(pidBytes[:])
	}

	if o.Anchorer == nil || err != nil {
		return b
	}
	blockNum := new(big.Int).SetUint64(res.BlockNumber)
	claimed := common.HexToHash(res.BlockHash)
	doAnchor := func() (string, error) {
		return o.Anchorer.Anchor(ctx, pidBytes, blockNum, o.SlashAmount, claimed)
	}
	if o.Sync {
		if tx, aerr := doAnchor(); aerr == nil {
			b.Anchored = true
			b.AnchorTx = &tx
		}
	} else {
		go func() { _, _ = doAnchor() }() // best-effort; delivery not blocked
	}
	return b
}

func bondProofID(p proof.Proof) ([32]byte, error) {
	core, err := proof.CanonicalCoreBytes(p)
	if err != nil {
		return [32]byte{}, err
	}
	return crypto.Keccak256Hash(core), nil
}

// toFields maps a verification Result onto the proof builder's inputs. Block
// facts are only carried when verification succeeded.
func toFields(in verify.Input, res verify.Result, issuedAt int64) proof.Fields {
	f := proof.Fields{
		ChainID:  in.ChainID,
		TxHash:   in.TxHash,
		Verified: res.Verified,
		Reason:   res.Reason,
		IssuedAt: issuedAt,
	}
	if res.Verified {
		bn := res.BlockNumber
		ti := res.TxIndex
		f.BlockNumber = &bn
		f.BlockHash = res.BlockHash
		f.TxIndex = &ti
	}
	return f
}

// Echo is the Phase 0 handler, kept for the loopback demo and reference. It
// returns a structurally valid AVP asserting verified:true with a placeholder
// (unsigned) attestation — no real RPC verification.
func Echo(_ context.Context, requirements json.RawMessage) (json.RawMessage, error) {
	var in verify.Input
	if err := json.Unmarshal(requirements, &in); err != nil {
		return nil, fmt.Errorf("invalid requirements JSON: %w", err)
	}
	if in.TxHash == "" {
		return nil, fmt.Errorf("requirements missing txHash")
	}
	if in.ChainID == 0 {
		in.ChainID = 8453
	}
	p := proof.BuildEcho(in.ChainID, in.TxHash, in.BlockNumber, nowUnix())
	out, err := p.JSON()
	if err != nil {
		return nil, fmt.Errorf("marshal proof: %w", err)
	}
	return out, nil
}
