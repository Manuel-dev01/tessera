package merkleproof

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

type meta struct {
	Number           string   `json:"number"`
	TransactionsRoot string   `json:"transactionsRoot"`
	TxCount          int      `json:"txCount"`
	TxHashes         []string `json:"txHashes"`
	TargetIndex      int      `json:"targetIndex"`
	TargetTxHash     string   `json:"targetTxHash"`
}

func load(t *testing.T) ([][]byte, meta) {
	t.Helper()
	rawHex, err := os.ReadFile("testdata/block_48493656.rawhex")
	if err != nil {
		t.Fatalf("read raw block: %v", err)
	}
	raw := common.FromHex(strings.TrimSpace(string(rawHex)))
	txs, err := ExtractBodyTxs(raw)
	if err != nil {
		t.Fatalf("extract txs: %v", err)
	}
	mb, err := os.ReadFile("testdata/block_48493656.meta.json")
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var m meta
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatalf("parse meta: %v", err)
	}
	return txs, m
}

// The headline test: build the tx trie from a REAL Base block (whose index-0 tx
// is an OP-stack deposit, type 0x7E) and confirm the computed root equals the
// block's transactionsRoot. A match proves the deposit tx is handled correctly
// with zero special-casing.
func TestRoot_MatchesRealBaseBlock(t *testing.T) {
	txs, m := load(t)
	if len(txs) != m.TxCount {
		t.Fatalf("extracted %d txs, header says %d", len(txs), m.TxCount)
	}
	got := Root(txs)
	want := common.HexToHash(m.TransactionsRoot)
	if got != want {
		t.Fatalf("computed root %s != header transactionsRoot %s", got.Hex(), want.Hex())
	}
}

func TestBuildAndVerify_TargetTx(t *testing.T) {
	txs, m := load(t)
	incl, err := Build(txs, m.TargetIndex)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	root := common.HexToHash(m.TransactionsRoot)
	if incl.TransactionsRoot != root {
		t.Fatalf("proof root %s != header %s", incl.TransactionsRoot.Hex(), root.Hex())
	}
	ok, err := Verify(root, uint64(m.TargetIndex), incl.Nodes, common.HexToHash(m.TargetTxHash))
	if err != nil || !ok {
		t.Fatalf("verify target: ok=%v err=%v", ok, err)
	}
}

// The deposit tx at index 0 must also prove — the historically hard case.
func TestBuildAndVerify_DepositTxIndex0(t *testing.T) {
	txs, m := load(t)
	incl, err := Build(txs, 0)
	if err != nil {
		t.Fatalf("build index 0: %v", err)
	}
	ok, err := Verify(common.HexToHash(m.TransactionsRoot), 0, incl.Nodes, common.HexToHash(m.TxHashes[0]))
	if err != nil || !ok {
		t.Fatalf("verify deposit tx: ok=%v err=%v", ok, err)
	}
}

func TestVerify_RejectsWrongTxHash(t *testing.T) {
	txs, m := load(t)
	incl, err := Build(txs, m.TargetIndex)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// A valid proof but claiming it proves a different tx must fail.
	wrong := common.HexToHash(m.TxHashes[0]) // deposit tx hash, not the target
	ok, _ := Verify(common.HexToHash(m.TransactionsRoot), uint64(m.TargetIndex), incl.Nodes, wrong)
	if ok {
		t.Fatal("expected verification to reject a mismatched txHash")
	}
}

func TestVerify_RejectsTamperedRoot(t *testing.T) {
	txs, m := load(t)
	incl, err := Build(txs, m.TargetIndex)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	bad := common.HexToHash("0xdeadbeef00000000000000000000000000000000000000000000000000000000")
	ok, _ := Verify(bad, uint64(m.TargetIndex), incl.Nodes, common.HexToHash(m.TargetTxHash))
	if ok {
		t.Fatal("expected verification to fail against a wrong root")
	}
}
