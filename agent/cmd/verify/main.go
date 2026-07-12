// Command verify runs Tessera's verification+signing pipeline once, offline (no
// CAP, no USDC), and prints the signed AgenticVerificationProof. Handy for demos
// and for feeding a proof into a client-side signature checker.
//
//	go run ./cmd/verify -tx 0x... -block 123 -addr 0x...
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"tessera/agent/internal/app"
	"tessera/agent/internal/config"
)

func main() {
	tx := flag.String("tx", "", "Base tx hash to verify (required)")
	addr := flag.String("addr", "", "address that must be involved in the tx")
	block := flag.Int64("block", 0, "block number that should contain the tx")
	chain := flag.Int64("chain", 8453, "EVM chain id (Base = 8453)")
	event := flag.String("event", "", "optional event signature, e.g. Transfer(address,address,uint256)")
	logIndex := flag.Int64("logindex", -1, "optional block-level log index")
	flag.Parse()

	if *tx == "" {
		fmt.Fprintln(os.Stderr, "-tx is required")
		os.Exit(2)
	}

	cfg, err := config.Load(envPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	h, signer, err := app.BuildVerifyHandler(cfg, log)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "oracle signer:", signer.Address())

	req := map[string]any{"chainId": *chain, "address": *addr, "txHash": *tx, "blockNumber": *block}
	if *event != "" {
		req["eventSignature"] = *event
	}
	if *logIndex >= 0 {
		req["logIndex"] = *logIndex
	}
	requirements, _ := json.Marshal(req)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := h(ctx, requirements)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify:", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

func envPath() string {
	for _, p := range []string{".env", "../.env", "../../.env"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ".env"
}
