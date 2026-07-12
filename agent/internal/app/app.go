// Package app centralizes construction of the verification stack (source pool →
// consensus engine → verify+sign handler) from Config, so cmd/provider and
// cmd/verify build it identically.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"tessera/agent/internal/bond"
	"tessera/agent/internal/config"
	"tessera/agent/internal/consensus"
	"tessera/agent/internal/handler"
	"tessera/agent/internal/sign"
	"tessera/agent/internal/source"
	"tessera/agent/internal/verify"
)

// BuildEngine assembles the consensus engine: the RPC pool + method-diverse
// explorer sources. Every source is abstain-safe, so unreachable ones simply
// don't vote.
func BuildEngine(cfg *config.Config, log *slog.Logger) (*consensus.Engine, error) {
	var sources []source.SourceAdapter

	for _, u := range cfg.BaseRPCURLs {
		s, err := source.NewRPCSource(operatorName(u), u)
		if err != nil {
			log.Warn("skipping RPC source", "url", u, "err", err)
			continue
		}
		sources = append(sources, s)
	}

	// Explorer source (different method / trust root).
	if cfg.BlockscoutURL != "" {
		sources = append(sources, source.NewBlockscoutSource("blockscout", cfg.BlockscoutURL,
			&http.Client{Timeout: 8 * time.Second}))
	}
	if cfg.BasescanAPIKey != "" {
		sources = append(sources, source.NewBasescanSource("basescan", cfg.BasescanAPIKey,
			&http.Client{Timeout: 8 * time.Second}))
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no consensus sources configured")
	}
	log.Info("consensus sources", "count", len(sources), "quorumFraction", cfg.QuorumFraction,
		"minResponders", cfg.MinResponders, "minConfirmations", cfg.MinConfirmations)

	return consensus.New(sources, cfg.QuorumFraction, cfg.MinResponders, cfg.MinConfirmations, 10*time.Second), nil
}

// BuildVerifyHandler wires engine + verifier + oracle signer into an OrderHandler.
func BuildVerifyHandler(cfg *config.Config, log *slog.Logger) (handler.OrderHandler, *sign.Signer, error) {
	engine, err := BuildEngine(cfg, log)
	if err != nil {
		return nil, nil, err
	}
	if cfg.OraclePrivateKey == "" {
		return nil, nil, fmt.Errorf("ORACLE_PRIVATE_KEY required for verification signing (set it in .env)")
	}
	signer, err := sign.NewSigner(cfg.OraclePrivateKey)
	if err != nil {
		return nil, nil, err
	}

	bondOpts, err := buildBondOpts(cfg, log)
	if err != nil {
		return nil, nil, err
	}
	return handler.NewVerifyHandler(engine, verify.New(), signer, bondOpts), signer, nil
}

// buildBondOpts wires the on-chain honesty bond when BOND_ENABLED. Advertising
// the standing bond needs no chain client; anchoring (BOND_ANCHOR) does.
func buildBondOpts(cfg *config.Config, log *slog.Logger) (handler.BondOpts, error) {
	if !cfg.BondEnabled {
		return handler.BondOpts{}, nil
	}
	opts := handler.BondOpts{
		Enabled:            true,
		Contract:           cfg.BondContract,
		StakedUSDC:         cfg.BondStandingStake,
		ChallengeWindowSec: cfg.ChallengeWindowSec,
		Sync:               cfg.BondAnchorSync,
	}
	if cfg.BondAnchor {
		if cfg.BondContract == "" || cfg.OraclePrivateKey == "" {
			return handler.BondOpts{}, fmt.Errorf("BOND_ANCHOR requires BOND_CONTRACT and ORACLE_PRIVATE_KEY")
		}
		rpcURL := "https://mainnet.base.org"
		if len(cfg.BaseRPCURLs) > 0 {
			rpcURL = cfg.BaseRPCURLs[0]
		}
		client, err := bond.New(context.Background(), rpcURL, cfg.BondContract, cfg.OraclePrivateKey)
		if err != nil {
			return handler.BondOpts{}, fmt.Errorf("bond client: %w", err)
		}
		slash, ok := new(big.Int).SetString(cfg.BondSlashUSDC, 10)
		if !ok {
			return handler.BondOpts{}, fmt.Errorf("invalid BOND_SLASH_USDC %q", cfg.BondSlashUSDC)
		}
		opts.Anchorer = client
		opts.SlashAmount = slash
		log.Info("bond anchoring enabled", "contract", cfg.BondContract, "oracle", client.From().Hex(), "slashUSDC", cfg.BondSlashUSDC, "sync", cfg.BondAnchorSync)
	} else {
		log.Info("bond advertising enabled (no anchoring)", "contract", cfg.BondContract, "standingStake", cfg.BondStandingStake)
	}
	return opts, nil
}

// operatorName derives a short, stable source label from a URL host.
func operatorName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}
