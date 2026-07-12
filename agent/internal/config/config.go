// Package config loads Tessera provider configuration from the environment,
// with an optional .env file loaded first. Secrets never live in code.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds everything the provider needs at startup.
type Config struct {
	CrooAPIURL string // CROO_API_URL
	CrooWSURL  string // CROO_WS_URL
	CrooSDKKey string // CROO_SDK_KEY (provider; required for TRANSPORT=croo)
	Transport  string // TRANSPORT: "loopback" (default) | "croo"

	// RequesterSDKKey (REQUESTER_SDK_KEY) is a SECOND agent's key used by the
	// requester demo client. CAP forbids an agent hiring its own service, so the
	// buyer must be a different agent. Falls back to CrooSDKKey if unset.
	RequesterSDKKey string

	ConsensusMode string   // CONSENSUS_MODE: "internal" (default) | "croo"
	BaseRPCURLs   []string // consensus RPC sources (BASE_RPC_URLS csv | BASE_RPC_URL_1..N | default pool)

	// Consensus tuning.
	QuorumFraction   float64 // CONSENSUS_QUORUM_FRACTION (default 9/11)
	MinResponders    int     // CONSENSUS_MIN_RESPONDERS (default 7)
	MinConfirmations uint64  // CONSENSUS_MIN_CONFIRMATIONS (0 = require finalized tag)

	// Extra (method-diverse) sources — all abstain-safe.
	BlockscoutURL        string // BLOCKSCOUT_URL (default https://base.blockscout.com; empty disables)
	BasescanAPIKey       string // BASESCAN_API_KEY (optional; enables Basescan source)
	CrooCheckerServiceID string // CROO_CHECKER_SERVICE_ID (optional; enables a hired sub-agent source in croo mode)

	OraclePrivateKey string // ORACLE_PRIVATE_KEY: EIP-191 signing key for attestations

	// Console HTTP API.
	APIAddr   string // API_ADDR (default :8787)
	WebOrigin string // WEB_ORIGIN for CORS (default http://localhost:3000)

	// Honesty bond (Phase 3).
	BondEnabled        bool   // BOND_ENABLED: stamp the standing-bond field on verified proofs
	BondContract       string // BOND_CONTRACT: TesseraBond address
	BondStandingStake  string // BOND_STANDING_STAKE: display string (e.g. "0.50")
	BondSlashUSDC      string // BOND_SLASH_USDC: per-anchor earmark, USDC base units (6-dec) as decimal string
	ChallengeWindowSec int    // CHALLENGE_WINDOW_SEC (default 3600)
	BondAnchor         bool   // BOND_ANCHOR: anchor verified proofs on-chain
	BondAnchorSync     bool   // BOND_ANCHOR_SYNC: anchor before delivery (hero demo) vs async

	ServiceID string // SERVICE_ID of the registered CAP service (croo transport)
}

// Load reads .env (if present) into the environment, then builds a Config.
func Load(envPath string) (*Config, error) {
	if err := loadDotEnv(envPath); err != nil {
		return nil, err
	}

	transport := getenvDefault("TRANSPORT", "loopback")
	if transport != "loopback" && transport != "croo" {
		return nil, fmt.Errorf("invalid TRANSPORT %q (want loopback|croo)", transport)
	}

	cfg := &Config{
		CrooAPIURL:    getenvDefault("CROO_API_URL", "https://api.croo.network"),
		CrooWSURL:     getenvDefault("CROO_WS_URL", "wss://api.croo.network/ws"),
		CrooSDKKey:      os.Getenv("CROO_SDK_KEY"),
		RequesterSDKKey: os.Getenv("REQUESTER_SDK_KEY"),
		Transport:       transport,
		ConsensusMode:    getenvDefault("CONSENSUS_MODE", "internal"),
		ServiceID:        os.Getenv("SERVICE_ID"),
		OraclePrivateKey: os.Getenv("ORACLE_PRIVATE_KEY"),
	}
	cfg.BaseRPCURLs = loadRPCPool()
	cfg.QuorumFraction = getenvFloat("CONSENSUS_QUORUM_FRACTION", 9.0/11.0)
	cfg.MinResponders = getenvInt("CONSENSUS_MIN_RESPONDERS", 7)
	cfg.MinConfirmations = uint64(getenvInt("CONSENSUS_MIN_CONFIRMATIONS", 0))
	cfg.BlockscoutURL = getenvDefault("BLOCKSCOUT_URL", "https://base.blockscout.com")
	cfg.BasescanAPIKey = os.Getenv("BASESCAN_API_KEY")
	cfg.CrooCheckerServiceID = os.Getenv("CROO_CHECKER_SERVICE_ID")

	cfg.BondEnabled = getenvBool("BOND_ENABLED", false)
	cfg.BondContract = os.Getenv("BOND_CONTRACT")
	cfg.BondStandingStake = getenvDefault("BOND_STANDING_STAKE", "0.50")
	cfg.BondSlashUSDC = getenvDefault("BOND_SLASH_USDC", "10000") // 0.01 USDC (6-dec)
	cfg.ChallengeWindowSec = getenvInt("CHALLENGE_WINDOW_SEC", 3600)
	cfg.BondAnchor = getenvBool("BOND_ANCHOR", false)
	cfg.BondAnchorSync = getenvBool("BOND_ANCHOR_SYNC", false)

	cfg.APIAddr = getenvDefault("API_ADDR", ":8787")
	// Cloud hosts (Railway, Render, Fly, …) inject the listen port via $PORT and
	// expect the process to bind 0.0.0.0:$PORT. When present, it wins over the
	// local default so the same binary runs unchanged in the cloud.
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		cfg.APIAddr = ":" + port
	}
	cfg.WebOrigin = getenvDefault("WEB_ORIGIN", "http://localhost:3000")

	if cfg.Transport == "croo" && cfg.CrooSDKKey == "" {
		return nil, fmt.Errorf("TRANSPORT=croo requires CROO_SDK_KEY (set it in .env)")
	}
	return cfg, nil
}

// DefaultBaseRPCs is the vetted pool of distinct-operator, keyless Base RPC
// endpoints (audited for liveness + operator independence). Keyed providers
// (Alchemy/Infura/…) can be added via BASE_RPC_URLS.
var DefaultBaseRPCs = []string{
	"https://mainnet.base.org",                 // Coinbase
	"https://base-rpc.publicnode.com",          // PublicNode
	"https://base.drpc.org",                    // dRPC (Geth)
	"https://1rpc.io/base",                     // 1RPC / Automata
	"https://base-mainnet.public.blastapi.io",  // BlastAPI / Bware
	"https://base.gateway.tenderly.co",         // Tenderly
	"https://base-pokt.nodies.app",             // Nodies / POKT
	"https://base.lava.build",                  // Lava
	"https://base.rpc.blxrbdn.com",             // bloXroute
	"https://api.zan.top/base-mainnet",         // ZAN
}

// loadRPCPool resolves the RPC source list: BASE_RPC_URLS (comma-separated) wins,
// else BASE_RPC_URL_1..12, else the default pool.
func loadRPCPool() []string {
	if csv := os.Getenv("BASE_RPC_URLS"); strings.TrimSpace(csv) != "" {
		var out []string
		for _, u := range strings.Split(csv, ",") {
			if u = strings.TrimSpace(u); u != "" {
				out = append(out, u)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	var numbered []string
	for i := 1; i <= 12; i++ {
		if v := os.Getenv(fmt.Sprintf("BASE_RPC_URL_%d", i)); v != "" {
			numbered = append(numbered, v)
		}
	}
	if len(numbered) > 0 {
		return numbered
	}
	return append([]string(nil), DefaultBaseRPCs...)
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}

func getenvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// loadDotEnv parses a simple KEY=VALUE .env file into the process environment.
// Missing file is not an error. Existing env vars are NOT overwritten.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`) // strip surrounding quotes
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}
