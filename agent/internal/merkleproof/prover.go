package merkleproof

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"tessera/agent/internal/proof"
)

// Prover turns a (blockNumber, txIndex, txHash) into a serializable transactions-
// trie inclusion proof by fetching the block's raw RLP via debug_getRawBlock from
// a debug-capable RPC. It is best-effort: callers should treat any error as
// "no merkle proof available" and leave the AVP field null, never fail the proof.
type Prover struct {
	url string
	hc  *http.Client
}

// NewProver builds a Prover against a debug_getRawBlock-capable endpoint.
func NewProver(rpcURL string) *Prover {
	return &Prover{url: rpcURL, hc: &http.Client{Timeout: 15 * time.Second}}
}

// Prove fetches the block, builds the inclusion proof for txIndex, self-checks
// that the proven leaf hashes to txHash, and returns it as a proof.MerkleProof.
func (p *Prover) Prove(ctx context.Context, blockNumber uint64, txIndex int, txHash string) (*proof.MerkleProof, error) {
	raw, err := p.rawBlock(ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	txs, err := ExtractBodyTxs(raw)
	if err != nil {
		return nil, err
	}
	incl, err := Build(txs, txIndex)
	if err != nil {
		return nil, err
	}
	if want := common.HexToHash(txHash); !bytes.Equal(crypto.Keccak256(incl.Leaf), want.Bytes()) {
		return nil, fmt.Errorf("leaf at index %d hashes to %#x, not txHash %s", txIndex, crypto.Keccak256(incl.Leaf), txHash)
	}

	nodes := make([]string, len(incl.Nodes))
	for i, n := range incl.Nodes {
		nodes[i] = hexutil.Encode(n)
	}
	return &proof.MerkleProof{
		Type:             "transactionsTrie",
		TransactionsRoot: incl.TransactionsRoot.Hex(),
		TxIndex:          incl.TxIndex,
		Key:              hexutil.Encode(incl.Key),
		Nodes:            nodes,
		Leaf:             hexutil.Encode(incl.Leaf),
	}, nil
}

func (p *Prover) rawBlock(ctx context.Context, blockNumber uint64) ([]byte, error) {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"debug_getRawBlock","params":["0x%x"]}`, blockNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := p.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode debug_getRawBlock: %w", err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("debug_getRawBlock: %s", out.Error.Message)
	}
	if out.Result == "" {
		return nil, fmt.Errorf("debug_getRawBlock returned empty result (endpoint may not support it)")
	}
	return common.FromHex(out.Result), nil
}
