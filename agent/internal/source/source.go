// Package source abstracts a read-only view of an EVM chain behind the
// SourceAdapter interface, so the verifier is independent of any single RPC
// provider. Phase 1 ships one adapter (RPCSource); Phase 2 adds more and layers
// a quorum on top without touching the verifier.
package source

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// Log is a normalized event log entry (only what the verifier needs).
type Log struct {
	Index   uint   // block-level log index
	Address string // lowercased 0x hex of the emitting contract
	Topic0  string // lowercased 0x hex of topics[0] (event signature hash), "" if none
}

// Receipt is a source-agnostic view of a transaction's on-chain result.
type Receipt struct {
	Found       bool
	BlockNumber uint64
	BlockHash   string // 0x hex
	TxIndex     uint
	Status      uint64 // 1 success, 0 reverted
	From        string // lowercased 0x hex
	To          string // lowercased 0x hex ("" for contract creation)
	Logs        []Log

	// Chain heads as this source sees them, for the finality gate. 0 = the
	// source does not report it (e.g. explorer sources abstain from finality).
	FinalizedBlock uint64
	LatestBlock    uint64
}

// HasLogs/richness helpers let the consensus engine pick the most complete
// receipt among an agreeing group (RPC receipts carry logs + sender; explorers
// may not).
func (r Receipt) Richness() int {
	n := 0
	if r.From != "" {
		n++
	}
	n += len(r.Logs)
	return n
}

// SourceAdapter fetches a normalized Receipt for a tx hash. Implementations must
// return Receipt{Found:false} (nil error) when the tx does not exist, and a
// non-nil error only for transport/RPC failures.
type SourceAdapter interface {
	Fetch(ctx context.Context, txHash string) (Receipt, error)
	Name() string
}

// RPCSource is a SourceAdapter backed by a single JSON-RPC endpoint.
type RPCSource struct {
	name   string
	client *ethclient.Client
}

// NewRPCSource dials url and returns an RPCSource labeled name.
func NewRPCSource(name, url string) (*RPCSource, error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", url, err)
	}
	return &RPCSource{name: name, client: client}, nil
}

func (s *RPCSource) Name() string { return s.name }

// Fetch pulls the receipt (block, status, index, logs) and the tx sender/recipient.
func (s *RPCSource) Fetch(ctx context.Context, txHash string) (Receipt, error) {
	hash := common.HexToHash(txHash)

	receipt, err := s.client.TransactionReceipt(ctx, hash)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return Receipt{Found: false}, nil
		}
		return Receipt{}, fmt.Errorf("%s: receipt: %w", s.name, err)
	}

	out := Receipt{
		Found:       true,
		BlockNumber: receipt.BlockNumber.Uint64(),
		BlockHash:   strings.ToLower(receipt.BlockHash.Hex()),
		TxIndex:     receipt.TransactionIndex,
		Status:      receipt.Status,
	}
	for _, l := range receipt.Logs {
		lg := Log{Index: l.Index, Address: strings.ToLower(l.Address.Hex())}
		if len(l.Topics) > 0 {
			lg.Topic0 = strings.ToLower(l.Topics[0].Hex())
		}
		out.Logs = append(out.Logs, lg)
	}

	// Sender/recipient require decoding the tx. Some chain-specific types (e.g.
	// OP-stack deposit txs, 0x7E) can't be decoded by go-ethereum; degrade
	// gracefully (leave From/To empty) rather than failing — the receipt already
	// proves existence, block, status, and logs.
	if tx, _, err := s.client.TransactionByHash(ctx, hash); err == nil {
		if to := tx.To(); to != nil {
			out.To = strings.ToLower(to.Hex())
		}
		if from, err := s.client.TransactionSender(ctx, tx, receipt.BlockHash, receipt.TransactionIndex); err == nil {
			out.From = strings.ToLower(from.Hex())
		}
	}

	// Chain heads for the finality gate (best-effort; leave 0 on error).
	if h, err := s.client.HeaderByNumber(ctx, big.NewInt(int64(rpc.FinalizedBlockNumber))); err == nil && h != nil {
		out.FinalizedBlock = h.Number.Uint64()
	}
	if h, err := s.client.HeaderByNumber(ctx, nil); err == nil && h != nil {
		out.LatestBlock = h.Number.Uint64()
	}
	return out, nil
}
