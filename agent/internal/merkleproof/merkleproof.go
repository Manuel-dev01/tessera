// Package merkleproof builds and verifies a transactions-trie inclusion proof:
// a Merkle-Patricia proof that a specific transaction sits at a specific index
// in a block, checkable against the block header's transactionsRoot with no
// trust in any oracle or consensus.
//
// It is deliberately type-agnostic. Each trie value is the transaction's raw
// CONSENSUS encoding (an RLP list for legacy txs, or type||payload for EIP-2718
// typed txs). Because it never decodes a transaction's fields, OP-stack deposit
// transactions (type 0x7E, always index 0 on Base) need no special handling —
// the very thing upstream go-ethereum cannot decode. Correctness is self-proving:
// the computed root must equal the block's transactionsRoot.
package merkleproof

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
)

// TxInclusion is a transactions-trie Merkle-Patricia inclusion proof for one tx.
type TxInclusion struct {
	TransactionsRoot common.Hash
	TxIndex          uint64
	Key              []byte   // RLP(txIndex) — the trie key
	Nodes            [][]byte // MPT proof nodes, root -> leaf (order not load-bearing)
	Leaf             []byte   // the tx's consensus encoding (the trie value)
}

// ExtractBodyTxs decodes a raw block (the hex from debug_getRawBlock) and returns
// each transaction's consensus encoding — the exact bytes stored as trie values.
// Legacy txs pass through as their RLP list; typed txs are unwrapped from the
// block body's RLP-string envelope to their type||payload form.
func ExtractBodyTxs(rawBlock []byte) ([][]byte, error) {
	var parts []rlp.RawValue
	if err := rlp.DecodeBytes(rawBlock, &parts); err != nil {
		return nil, fmt.Errorf("decode block rlp: %w", err)
	}
	if len(parts) < 2 {
		return nil, fmt.Errorf("block rlp has %d top-level items, want >= 2", len(parts))
	}
	var body []rlp.RawValue
	if err := rlp.DecodeBytes(parts[1], &body); err != nil {
		return nil, fmt.Errorf("decode transactions list: %w", err)
	}
	txs := make([][]byte, len(body))
	for i, raw := range body {
		v, err := consensusEncoding(raw)
		if err != nil {
			return nil, fmt.Errorf("tx %d: %w", i, err)
		}
		txs[i] = v
	}
	return txs, nil
}

// consensusEncoding turns a block-body tx element into the trie value. An RLP
// list (first byte >= 0xc0) is a legacy tx and is its own consensus encoding; an
// RLP string wraps a typed tx's type||payload, which we unwrap.
func consensusEncoding(bodyTx rlp.RawValue) ([]byte, error) {
	if len(bodyTx) == 0 {
		return nil, fmt.Errorf("empty tx element")
	}
	if bodyTx[0] >= 0xc0 {
		return bodyTx, nil
	}
	var inner []byte
	if err := rlp.DecodeBytes(bodyTx, &inner); err != nil {
		return nil, fmt.Errorf("unwrap typed tx: %w", err)
	}
	return inner, nil
}

// buildTrie constructs the transactions trie: key = RLP(index), value = the tx's
// consensus encoding. This is exactly how a block's transactionsRoot is derived.
func buildTrie(txs [][]byte) *trie.Trie {
	tr := trie.NewEmpty(triedb.NewDatabase(rawdb.NewMemoryDatabase(), nil))
	for i, v := range txs {
		tr.MustUpdate(rlp.AppendUint64(nil, uint64(i)), v)
	}
	return tr
}

// Root computes the transactions root for a set of consensus-encoded txs. Callers
// should confirm it equals the block header's transactionsRoot.
func Root(txs [][]byte) common.Hash {
	return buildTrie(txs).Hash()
}

// Build produces the inclusion proof for txIndex. The returned TransactionsRoot
// is the computed root; the caller must confirm it matches the block header.
func Build(txs [][]byte, txIndex int) (*TxInclusion, error) {
	if txIndex < 0 || txIndex >= len(txs) {
		return nil, fmt.Errorf("txIndex %d out of range [0,%d)", txIndex, len(txs))
	}
	tr := buildTrie(txs)
	key := rlp.AppendUint64(nil, uint64(txIndex))
	var nodes proofList
	if err := tr.Prove(key, &nodes); err != nil {
		return nil, fmt.Errorf("prove index %d: %w", txIndex, err)
	}
	return &TxInclusion{
		TransactionsRoot: tr.Hash(),
		TxIndex:          uint64(txIndex),
		Key:              key,
		Nodes:            nodes,
		Leaf:             txs[txIndex],
	}, nil
}

// Verify checks an inclusion proof against a known transactionsRoot and confirms
// the proven leaf hashes to expectedTxHash. Trustless: it re-derives everything
// from the proof nodes and the root — no chain access, no oracle.
func Verify(transactionsRoot common.Hash, txIndex uint64, nodes [][]byte, expectedTxHash common.Hash) (bool, error) {
	db := memorydb.New()
	for _, n := range nodes {
		if err := db.Put(crypto.Keccak256(n), n); err != nil {
			return false, err
		}
	}
	key := rlp.AppendUint64(nil, txIndex)
	val, err := trie.VerifyProof(transactionsRoot, key, db)
	if err != nil {
		return false, fmt.Errorf("verify proof: %w", err)
	}
	if val == nil {
		return false, fmt.Errorf("no value at index %d", txIndex)
	}
	if !bytes.Equal(crypto.Keccak256(val), expectedTxHash.Bytes()) {
		return false, fmt.Errorf("proven leaf hashes to %#x, not the claimed txHash", crypto.Keccak256(val))
	}
	return true, nil
}

// proofList collects trie proof nodes in root->leaf order.
type proofList [][]byte

func (p *proofList) Put(key, value []byte) error { *p = append(*p, value); return nil }
func (p *proofList) Delete(key []byte) error     { return fmt.Errorf("proofList: delete not supported") }
