// Command checker runs a minimal CAP provider for a THIRD agent: the sub-checker
// that Tessera itself hires as a second opinion. It serves a "TX Receipt Check"
// service that, given {chainId, txHash}, looks up the transaction receipt and
// returns {found, status, blockNumber, blockHash}.
//
// This exists to demonstrate A2A DEPTH: Tessera is both a provider (hired by a
// buyer) and a consumer (it hires this sub-checker), a two-hop composition.
//
// The sub-checker's SDK key is read from the CHECKER_SDK_KEY environment variable
// so it is never committed. Run:
//
//	CHECKER_SDK_KEY=croo_sk_... go run ./cmd/checker
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"tessera/agent/internal/cap"
	"tessera/agent/internal/config"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(envPath())
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	key := os.Getenv("CHECKER_SDK_KEY")
	if key == "" {
		log.Error("CHECKER_SDK_KEY required (the third agent's SDK key)")
		os.Exit(1)
	}
	rpc := "https://mainnet.base.org"
	if len(cfg.BaseRPCURLs) > 0 {
		rpc = cfg.BaseRPCURLs[0]
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	transport, err := cap.NewLiveTransport(cfg.CrooAPIURL, cfg.CrooWSURL, rpc, key, log)
	if err != nil {
		log.Error("build transport", "err", err)
		os.Exit(1)
	}
	log.Info("sub-checker starting", "rpc", rpc)

	if err := transport.Run(ctx, checkerHandler(rpc, log)); err != nil && ctx.Err() == nil {
		log.Error("checker stopped", "err", err)
		os.Exit(1)
	}
}

// checkerHandler independently looks up a transaction receipt and returns a
// compact JSON verdict. A missing tx is a valid answer (found:false), not an error.
func checkerHandler(rpcURL string, log *slog.Logger) func(context.Context, json.RawMessage) (json.RawMessage, error) {
	return func(ctx context.Context, requirements json.RawMessage) (json.RawMessage, error) {
		var in struct {
			ChainID int64  `json:"chainId"`
			TxHash  string `json:"txHash"`
		}
		if err := json.Unmarshal(requirements, &in); err != nil {
			return nil, fmt.Errorf("invalid requirements: %w", err)
		}
		if in.TxHash == "" {
			return nil, fmt.Errorf("requirements missing txHash")
		}
		out := map[string]any{"source": "tx-receipt-check", "txHash": in.TxHash, "found": false}

		ec, err := ethclient.DialContext(ctx, rpcURL)
		if err == nil {
			if r, rerr := ec.TransactionReceipt(ctx, common.HexToHash(in.TxHash)); rerr == nil {
				out["found"] = true
				out["status"] = r.Status
				out["blockNumber"] = r.BlockNumber.Uint64()
				out["blockHash"] = r.BlockHash.Hex()
			}
			ec.Close()
		}
		b, _ := json.Marshal(out)
		log.Info("checker delivered verdict", "txHash", in.TxHash, "found", out["found"])
		return json.RawMessage(b), nil
	}
}

func envPath() string {
	for _, p := range []string{".env", "../.env", "../../.env"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ".env"
}
