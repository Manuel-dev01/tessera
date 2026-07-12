// Command provider runs the Tessera CAP provider service.
//
// TRANSPORT=loopback (default) runs an in-memory round-trip demo needing no
// credentials — the Phase 0 deliverable. TRANSPORT=croo connects to live CAP via
// the CROO SDK (requires a registered agent + CROO_SDK_KEY).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"tessera/agent/internal/app"
	"tessera/agent/internal/cap"
	"tessera/agent/internal/config"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Load .env from repo root (../../.env relative to agent/) or CWD.
	cfg, err := config.Load(envPath())
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	log.Info("tessera provider starting", "transport", cfg.Transport, "consensusMode", cfg.ConsensusMode)

	// Cancel on Ctrl-C / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Build the verify+sign handler (multi-source consensus + finality + EIP-191).
	orderHandler, signer, err := app.BuildVerifyHandler(cfg, log)
	if err != nil {
		log.Error("build handler", "err", err)
		os.Exit(1)
	}
	log.Info("oracle signer ready", "address", signer.Address())

	transport, err := buildTransport(cfg, log)
	if err != nil {
		log.Error("build transport", "err", err)
		os.Exit(1)
	}

	if err := transport.Run(ctx, orderHandler); err != nil && ctx.Err() == nil {
		log.Error("provider stopped", "err", err)
		os.Exit(1)
	}
	log.Info("provider exited cleanly")
}

func buildTransport(cfg *config.Config, log *slog.Logger) (cap.Transport, error) {
	switch cfg.Transport {
	case "croo":
		rpc := "https://mainnet.base.org"
		if len(cfg.BaseRPCURLs) > 0 {
			rpc = cfg.BaseRPCURLs[0]
		}
		return cap.NewLiveTransport(cfg.CrooAPIURL, cfg.CrooWSURL, rpc, cfg.CrooSDKKey, log)
	default: // loopback
		return &cap.Loopback{Log: log}, nil
	}
}

// envPath returns the first existing .env path, preferring the repo root.
func envPath() string {
	for _, p := range []string{".env", "../.env", "../../.env"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ".env"
}
