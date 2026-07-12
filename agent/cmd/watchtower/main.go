// Command watchtower is Tessera's decentralized fraud enforcer. Given an anchored
// proofId, it independently re-verifies the anchored block hash against its own
// RPC, and if the oracle's claim is fraudulent it submits challenge() to slash the
// bond — permissionless and economically incentivized (the slash pays the caller).
//
// This is the role a hired CROO sub-agent verifier would fill: an independent
// party that disputes bad proofs. The on-chain arbiter is EIP-2935, so no trust
// in the watchtower is required — it only decides whether to spend gas.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"tessera/agent/internal/bond"
	"tessera/agent/internal/config"
)

func main() {
	proofIDHex := flag.String("proofId", "", "0x-prefixed proofId to check and (if fraudulent) challenge")
	dryRun := flag.Bool("dry-run", false, "detect fraud but do not submit the on-chain challenge")
	flag.Parse()

	cfg, err := config.Load(envPath())
	if err != nil {
		die("config: %v", err)
	}
	if cfg.BondContract == "" || cfg.OraclePrivateKey == "" {
		die("BOND_CONTRACT and ORACLE_PRIVATE_KEY (challenger key) required in .env")
	}
	pid, err := parseHash(*proofIDHex)
	if err != nil {
		die("proofId: %v", err)
	}

	rpcURL := "https://mainnet.base.org"
	if len(cfg.BaseRPCURLs) > 0 {
		rpcURL = cfg.BaseRPCURLs[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := bond.New(ctx, rpcURL, cfg.BondContract, cfg.OraclePrivateKey)
	if err != nil {
		die("bond client: %v", err)
	}

	a, err := c.GetAnchor(ctx, pid)
	if err != nil {
		die("read anchor: %v", err)
	}
	if !a.Exists {
		die("no anchor found for proofId %s", *proofIDHex)
	}
	fmt.Printf("anchor: oracle=%s block=%s claimed=0x%s resolved=%v\n",
		a.Oracle.Hex(), a.BlockNumber, hex.EncodeToString(a.ClaimedBlockHash[:]), a.Resolved)
	if a.Resolved {
		fmt.Println("already resolved (slashed or released) — nothing to do")
		return
	}

	// Independent verification: fetch the canonical hash ourselves.
	canonical, err := c.CanonicalBlockHash(ctx, a.BlockNumber)
	if err != nil {
		die("canonical hash: %v", err)
	}
	fmt.Printf("canonical: 0x%s\n", hex.EncodeToString(canonical[:]))

	if bytes.Equal(canonical[:], a.ClaimedBlockHash[:]) {
		fmt.Println("VERDICT: honest — claimed hash matches canonical. No challenge.")
		return
	}
	fmt.Println("VERDICT: FRAUD — claimed hash != canonical.")
	if *dryRun {
		fmt.Println("(dry-run) would submit challenge() to slash the bond")
		return
	}

	fmt.Println("submitting challenge() to slash...")
	tx, err := c.Challenge(ctx, pid)
	if err != nil {
		die("challenge: %v (tx %s)", err, tx)
	}
	fmt.Printf("SLASHED. challenge tx: %s\n", tx)
	fmt.Printf("  basescan: https://basescan.org/tx/%s\n", tx)
}

func parseHash(s string) ([32]byte, error) {
	var h [32]byte
	b, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(s), "0x"))
	if err != nil {
		return h, err
	}
	if len(b) != 32 {
		return h, fmt.Errorf("want 32 bytes, got %d", len(b))
	}
	copy(h[:], b)
	return h, nil
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

func envPath() string {
	for _, p := range []string{".env", "../.env", "../../.env"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ".env"
}
