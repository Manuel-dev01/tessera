package consensus

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"tessera/agent/internal/source"
)

type fakeSource struct {
	name string
	r    source.Receipt
	err  error
}

func (f fakeSource) Name() string { return f.name }
func (f fakeSource) Fetch(_ context.Context, _ string) (source.Receipt, error) {
	return f.r, f.err
}

// found builds an agreeing receipt at block/hash with finalized+latest heads set.
func found(block uint64, hash string, finalized, latest uint64) source.Receipt {
	return source.Receipt{
		Found: true, BlockNumber: block, BlockHash: hash, Status: 1, TxIndex: 1,
		From: "0xabc", FinalizedBlock: finalized, LatestBlock: latest,
	}
}

func mkSources(n int, r source.Receipt) []source.SourceAdapter {
	var out []source.SourceAdapter
	for i := 0; i < n; i++ {
		out = append(out, fakeSource{name: fmt.Sprintf("s%d", i), r: r})
	}
	return out
}

const frac = 9.0 / 11.0

func TestConsensus(t *testing.T) {
	ctx := context.Background()
	block := uint64(100)
	hash := "0xdead"

	t.Run("11 agree, finalized -> verified", func(t *testing.T) {
		e := New(mkSources(11, found(block, hash, 120, 130)), frac, 7, 0, time.Second)
		o := e.Fetch(ctx, "0xtx")
		if !o.Reached || o.Agreed != 11 || o.Quorum != 9 {
			t.Fatalf("reached=%v agreed=%d quorum=%d", o.Reached, o.Agreed, o.Quorum)
		}
		if !o.FinalOK || !o.Finality.Finalized {
			t.Fatalf("finality not ok: %+v", o.Finality)
		}
		if len(o.AgreedSources) != 11 {
			t.Fatalf("agreedSources=%v", o.AgreedSources)
		}
	})

	t.Run("2 down -> quorum recomputed on 9 responders", func(t *testing.T) {
		src := mkSources(9, found(block, hash, 120, 130))
		src = append(src, fakeSource{name: "down1", err: context.DeadlineExceeded})
		src = append(src, fakeSource{name: "down2", err: context.DeadlineExceeded})
		e := New(src, frac, 7, 0, time.Second)
		o := e.Fetch(ctx, "0xtx")
		if !o.Reached || o.Responders != 9 || o.Agreed != 9 {
			t.Fatalf("reached=%v responders=%d agreed=%d quorum=%d", o.Reached, o.Responders, o.Agreed, o.Quorum)
		}
	})

	t.Run("below responder floor -> refuse", func(t *testing.T) {
		src := mkSources(6, found(block, hash, 120, 130))
		for i := 0; i < 5; i++ {
			src = append(src, fakeSource{name: fmt.Sprintf("d%d", i), err: context.DeadlineExceeded})
		}
		e := New(src, frac, 7, 0, time.Second)
		o := e.Fetch(ctx, "0xtx")
		if o.Reached || o.Reason == nil || !strings.Contains(*o.Reason, "insufficient live sources") {
			t.Fatalf("expected insufficient-sources refusal, got reached=%v reason=%v", o.Reached, o.Reason)
		}
	})

	t.Run("disagreement -> no quorum", func(t *testing.T) {
		var src []source.SourceAdapter
		for i := 0; i < 6; i++ {
			src = append(src, fakeSource{name: fmt.Sprintf("a%d", i), r: found(block, "0xaaaa", 120, 130)})
		}
		for i := 0; i < 5; i++ {
			src = append(src, fakeSource{name: fmt.Sprintf("b%d", i), r: found(block, "0xbbbb", 120, 130)})
		}
		e := New(src, frac, 7, 0, time.Second)
		o := e.Fetch(ctx, "0xtx")
		if o.Reached || o.Agreed != 6 || o.Reason == nil || !strings.Contains(*o.Reason, "quorum not reached") {
			t.Fatalf("reached=%v agreed=%d reason=%v", o.Reached, o.Agreed, o.Reason)
		}
	})

	t.Run("not-found consensus", func(t *testing.T) {
		e := New(mkSources(11, source.Receipt{Found: false, FinalizedBlock: 120, LatestBlock: 130}), frac, 7, 0, time.Second)
		o := e.Fetch(ctx, "0xtx")
		if !o.Reached || o.Receipt.Found {
			t.Fatalf("expected reached not-found consensus, got reached=%v found=%v", o.Reached, o.Receipt.Found)
		}
	})

	t.Run("FetchStream emits one report per source", func(t *testing.T) {
		e := New(mkSources(11, found(block, hash, 120, 130)), frac, 7, 0, time.Second)
		var mu sync.Mutex
		var reports []SourceReport
		o := e.FetchStream(ctx, "0xtx", func(r SourceReport) {
			mu.Lock()
			reports = append(reports, r)
			mu.Unlock()
		})
		if len(reports) != 11 {
			t.Fatalf("got %d reports, want 11", len(reports))
		}
		for _, r := range reports {
			if !r.OK || !r.Found || r.BlockHash != hash {
				t.Fatalf("bad report: %+v", r)
			}
		}
		if !o.Reached || o.Agreed != 11 { // final outcome unchanged vs Fetch
			t.Fatalf("outcome changed under streaming: %+v", o)
		}
	})

	t.Run("agreed but not finalized", func(t *testing.T) {
		// finalized head (80) < claimed block (100): consensus reached, finality fails.
		e := New(mkSources(11, found(block, hash, 80, 105)), frac, 7, 0, time.Second)
		o := e.Fetch(ctx, "0xtx")
		if !o.Reached {
			t.Fatalf("expected consensus reached")
		}
		if o.FinalOK || o.Finality.Finalized {
			t.Fatalf("expected finality NOT ok, got %+v", o.Finality)
		}
	})
}
