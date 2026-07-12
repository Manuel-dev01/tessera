package avp

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

// MerkleProof mirrors the AVP `merkleProof` object: a transactions-trie inclusion
// proof for the verified tx.
type MerkleProof struct {
	Type             string   `json:"type"`
	TransactionsRoot string   `json:"transactionsRoot"`
	TxIndex          uint64   `json:"txIndex"`
	Key              string   `json:"key"`
	Nodes            []string `json:"nodes"`
	Leaf             string   `json:"leaf"`
}

// VerifyInclusion checks a transactions-trie inclusion proof: that `nodes` prove a
// leaf at RLP(txIndex) under `root`, and that the leaf's keccak256 equals txHash.
//
// Pass the block header's real transactionsRoot as `root` for a FULLY TRUSTLESS
// check (tx is provably in that block). Passing the proof's own transactionsRoot
// checks internal consistency only, relying on the AVP signature to vouch for the
// root — always verify the signature (verifyAVP) as well.
func VerifyInclusion(root common.Hash, txIndex uint64, nodesHex []string, txHash common.Hash) (bool, error) {
	db := memorydb.New()
	for _, h := range nodesHex {
		n, err := hexutil.Decode(h)
		if err != nil {
			return false, fmt.Errorf("decode proof node: %w", err)
		}
		if err := db.Put(crypto.Keccak256(n), n); err != nil {
			return false, err
		}
	}
	val, err := trie.VerifyProof(root, rlp.AppendUint64(nil, txIndex), db)
	if err != nil {
		return false, fmt.Errorf("verify proof: %w", err)
	}
	if val == nil {
		return false, fmt.Errorf("no value at index %d", txIndex)
	}
	if !bytes.Equal(crypto.Keccak256(val), txHash.Bytes()) {
		return false, fmt.Errorf("proven leaf hashes to %#x, not txHash %s", crypto.Keccak256(val), txHash.Hex())
	}
	return true, nil
}

// VerifyProofInclusion extracts the merkleProof and txHash from an AVP and verifies
// inclusion. If trustedRoot is non-nil it is used (trustless — supply the block
// header's transactionsRoot); otherwise the proof's own transactionsRoot is used
// (relies on the signature). Returns (false, nil) when the proof has no merkleProof.
func VerifyProofInclusion(proofJSON []byte, trustedRoot *common.Hash) (bool, error) {
	var p struct {
		TxHash      string       `json:"txHash"`
		MerkleProof *MerkleProof `json:"merkleProof"`
	}
	if err := json.Unmarshal(proofJSON, &p); err != nil {
		return false, err
	}
	if p.MerkleProof == nil {
		return false, nil
	}
	root := common.HexToHash(p.MerkleProof.TransactionsRoot)
	if trustedRoot != nil {
		root = *trustedRoot
	}
	return VerifyInclusion(root, p.MerkleProof.TxIndex, p.MerkleProof.Nodes, common.HexToHash(p.TxHash))
}
