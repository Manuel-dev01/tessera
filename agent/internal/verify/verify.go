// Package verify applies Tessera's on-chain claim rules to a normalized
// source.Receipt and produces a Result the proof builder turns into an AVP.
// It is deterministic and source-agnostic.
package verify

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	"tessera/agent/internal/source"
)

// Input is the requester's claim — the CAP service Requirements. Owned here (not
// in handler) so verify has no dependency on the handler package.
type Input struct {
	ChainID        int64   `json:"chainId"`
	Address        string  `json:"address"`
	TxHash         string  `json:"txHash"`
	BlockNumber    int64   `json:"blockNumber"`
	EventSignature *string `json:"eventSignature,omitempty"`
	LogIndex       *int64  `json:"logIndex,omitempty"`
}

// Result is the outcome of verification. On success Verified is true and the
// block fields are populated; on failure Reason explains why.
type Result struct {
	Verified    bool
	Reason      *string
	BlockNumber uint64
	BlockHash   string
	TxIndex     uint
}

// Verifier holds no state today but is a struct so Phase 2 can carry quorum config.
type Verifier struct{}

func New() *Verifier { return &Verifier{} }

// Verify checks the receipt against the claim. It never errors: an unmet claim
// is a valid negative result (which still gets signed downstream).
func (v *Verifier) Verify(in Input, r source.Receipt) Result {
	fail := func(reason string) Result {
		return Result{Verified: false, Reason: &reason}
	}

	if !r.Found {
		return fail("tx not found")
	}
	if int64(r.BlockNumber) != in.BlockNumber {
		return fail(fmt.Sprintf("tx not in claimed block %d (actual %d)", in.BlockNumber, r.BlockNumber))
	}
	if r.Status != 1 {
		return fail("tx reverted (status 0)")
	}

	// address must be involved: sender, recipient, or any log emitter.
	addr := strings.ToLower(strings.TrimSpace(in.Address))
	if addr != "" && !addressInvolved(addr, r) {
		return fail("address not involved in tx")
	}

	// optional event-log check.
	if in.EventSignature != nil && strings.TrimSpace(*in.EventSignature) != "" {
		topic0 := strings.ToLower(crypto.Keccak256Hash([]byte(strings.TrimSpace(*in.EventSignature))).Hex())
		if !logMatches(topic0, in.LogIndex, r) {
			if in.LogIndex != nil {
				return fail(fmt.Sprintf("no matching event log at index %d", *in.LogIndex))
			}
			return fail("no matching event log for signature")
		}
	}

	return Result{
		Verified:    true,
		BlockNumber: r.BlockNumber,
		BlockHash:   r.BlockHash,
		TxIndex:     r.TxIndex,
	}
}

func addressInvolved(addr string, r source.Receipt) bool {
	if addr == r.From || addr == r.To {
		return true
	}
	for _, l := range r.Logs {
		if l.Address == addr {
			return true
		}
	}
	return false
}

// logMatches reports whether a log with the given topic0 exists. If logIndex is
// set, the log at that block-level index must be the one matching.
func logMatches(topic0 string, logIndex *int64, r source.Receipt) bool {
	for _, l := range r.Logs {
		if l.Topic0 != topic0 {
			continue
		}
		if logIndex == nil || int64(l.Index) == *logIndex {
			return true
		}
	}
	return false
}
