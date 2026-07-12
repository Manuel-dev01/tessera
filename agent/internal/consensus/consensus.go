// Package consensus queries many independent SourceAdapters concurrently and
// decides whether a supermajority agrees on a transaction's on-chain facts,
// with a dynamic (responder-based) quorum and a finality gate. It is the trust
// core of Tessera: nothing gets signed unless enough diverse sources agree AND
// the block is final.
package consensus

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"tessera/agent/internal/proof"
	"tessera/agent/internal/source"
)

// SourceReport is a single source's live result, emitted as it responds so a UI
// can animate consensus forming in real time.
type SourceReport struct {
	Name        string `json:"name"`
	OK          bool   `json:"ok"` // responded without a transport error
	Found       bool   `json:"found"`
	BlockNumber uint64 `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
	Status      uint64 `json:"status"`
	Millis      int64  `json:"ms"`
}

// Outcome is the result of a consensus round.
type Outcome struct {
	Receipt       source.Receipt // richest member of the winning agreement group
	Reached       bool           // responders >= floor AND agreed >= quorum
	Sources       int            // configured sources
	Responders    int            // sources that answered in time
	Agreed        int            // size of the winning agreement group
	Quorum        int            // dynamic threshold that had to be met
	AgreedSources []string       // names of sources in the winning group
	Reason        *string        // set when !Reached
	Finality      proof.Finality // finality summary for the agreed block
	FinalOK       bool           // a quorum of finality-reporting sources confirm the block is final
}

// Engine holds the source pool and consensus parameters.
type Engine struct {
	sources        []source.SourceAdapter
	quorumFraction float64       // supermajority of responders, e.g. 9/11
	minResponders  int           // floor: below this we refuse to sign
	minConfirms    uint64        // 0 = require the finalized tag; >0 = require this many confirmations
	timeout        time.Duration // per-source timeout
}

// New builds an Engine. quorumFraction is clamped to (0,1]; minResponders and
// timeout get sane defaults if non-positive.
func New(sources []source.SourceAdapter, quorumFraction float64, minResponders int, minConfirms uint64, timeout time.Duration) *Engine {
	if quorumFraction <= 0 || quorumFraction > 1 {
		quorumFraction = 9.0 / 11.0
	}
	if minResponders < 2 {
		minResponders = 2
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Engine{sources, quorumFraction, minResponders, minConfirms, timeout}
}

// Sources returns the configured source count.
func (e *Engine) Sources() int { return len(e.sources) }

// SourceNames returns the label of every configured source (for UIs).
func (e *Engine) SourceNames() []string {
	names := make([]string, 0, len(e.sources))
	for _, s := range e.sources {
		names = append(names, s.Name())
	}
	return names
}

// MinResponders / QuorumFraction expose config for UIs.
func (e *Engine) MinResponders() int      { return e.minResponders }
func (e *Engine) QuorumFraction() float64 { return e.quorumFraction }

type reply struct {
	name    string
	receipt source.Receipt
	ok      bool // responded without error
}

// Fetch runs one consensus round for txHash.
func (e *Engine) Fetch(ctx context.Context, txHash string) Outcome {
	return e.FetchStream(ctx, txHash, nil)
}

// FetchStream runs one consensus round, invoking onSource (if non-nil) once per
// source as it responds — for live UIs. onSource is called serially.
func (e *Engine) FetchStream(ctx context.Context, txHash string, onSource func(SourceReport)) Outcome {
	replies := e.gather(ctx, txHash, onSource)

	var responders []reply
	for _, r := range replies {
		if r.ok {
			responders = append(responders, r)
		}
	}

	out := Outcome{Sources: len(e.sources), Responders: len(responders)}
	quorum := int(math.Ceil(e.quorumFraction * float64(len(responders))))
	if quorum < 2 {
		quorum = 2
	}
	out.Quorum = quorum

	if len(responders) < e.minResponders {
		out.reason("insufficient live sources: %d responded, need >= %d", len(responders), e.minResponders)
		return out
	}

	// Group responders by agreement key and find the largest group.
	groups := map[string][]reply{}
	for _, r := range responders {
		groups[agreementKey(r.receipt)] = append(groups[agreementKey(r.receipt)], r)
	}
	var winner []reply
	for _, g := range groups {
		if len(g) > len(winner) {
			winner = g
		}
	}

	out.Agreed = len(winner)
	for _, r := range winner {
		out.AgreedSources = append(out.AgreedSources, r.name)
	}
	sort.Strings(out.AgreedSources)
	out.Receipt = richest(winner)

	if out.Agreed < quorum {
		out.reason("quorum not reached: %d/%d responders agreed (need %d)", out.Agreed, len(responders), quorum)
		return out
	}
	out.Reached = true

	// Finality gate over the agreed block (only sources that report heads vote).
	out.Finality, out.FinalOK = e.finality(responders, out.Receipt.BlockNumber)
	return out
}

// gather queries every source concurrently, each under its own timeout. If
// onSource is non-nil it is invoked (serially) as each source responds.
func (e *Engine) gather(ctx context.Context, txHash string, onSource func(SourceReport)) []reply {
	replies := make([]reply, len(e.sources))
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i, s := range e.sources {
		wg.Add(1)
		go func(i int, s source.SourceAdapter) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, e.timeout)
			defer cancel()
			start := time.Now()
			rc, err := s.Fetch(cctx, txHash)
			ms := time.Since(start).Milliseconds()
			replies[i] = reply{name: s.Name(), receipt: rc, ok: err == nil}
			if onSource != nil {
				mu.Lock()
				onSource(SourceReport{
					Name: s.Name(), OK: err == nil, Found: rc.Found,
					BlockNumber: rc.BlockNumber, BlockHash: rc.BlockHash, Status: rc.Status, Millis: ms,
				})
				mu.Unlock()
			}
		}(i, s)
	}
	wg.Wait()
	return replies
}

// finality decides whether the agreed block is settled. Only sources that report
// the metric actually in use vote — a source that returns a latest head but not
// the `finalized` tag (common on public RPCs) abstains from the finalized-tag
// vote rather than being counted as a non-confirming reporter. A supermajority
// of the voting sources must confirm.
func (e *Engine) finality(responders []reply, block uint64) (proof.Finality, bool) {
	var finalizedHeads, latestHeads []uint64
	for _, r := range responders {
		if r.receipt.FinalizedBlock > 0 {
			finalizedHeads = append(finalizedHeads, r.receipt.FinalizedBlock)
		}
		if r.receipt.LatestBlock > 0 {
			latestHeads = append(latestHeads, r.receipt.LatestBlock)
		}
	}

	confirms, reporters := 0, 0
	if e.minConfirms > 0 { // confirmation-depth mode: sources reporting a latest head vote
		for _, h := range latestHeads {
			reporters++
			if h >= block && h-block >= e.minConfirms {
				confirms++
			}
		}
	} else { // finalized-tag mode: only sources that report a finalized head vote
		for _, h := range finalizedHeads {
			reporters++
			if h >= block {
				confirms++
			}
		}
	}

	fin := proof.Finality{}
	if len(finalizedHeads) > 0 {
		fin.FinalizedBlock = median(finalizedHeads)
	}
	if latest := medianOrZero(latestHeads); latest >= block {
		fin.Confirmations = latest - block
	}

	need := int(math.Ceil(e.quorumFraction * float64(reporters)))
	if need < 1 {
		need = 1
	}
	ok := reporters > 0 && confirms >= need
	fin.Finalized = ok
	return fin, ok
}

func (o *Outcome) reason(format string, a ...any) {
	s := fmt.Sprintf(format, a...)
	o.Reason = &s
}

// agreementKey is what a quorum must match on: existence, block, and status.
func agreementKey(r source.Receipt) string {
	if !r.Found {
		return "notfound"
	}
	return fmt.Sprintf("%d|%s|%d", r.BlockNumber, r.BlockHash, r.Status)
}

// richest returns the group member with the most complete receipt (RPC receipts
// carry logs + sender that explorer receipts may lack), so downstream event-log
// checks have the data they need.
func richest(group []reply) source.Receipt {
	best := group[0].receipt
	for _, r := range group[1:] {
		if r.receipt.Richness() > best.Richness() {
			best = r.receipt
		}
	}
	return best
}

func median(xs []uint64) uint64 {
	s := append([]uint64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}

func medianOrZero(xs []uint64) uint64 {
	if len(xs) == 0 {
		return 0
	}
	return median(xs)
}
