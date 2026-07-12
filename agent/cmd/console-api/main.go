// Command console-api serves Tessera's verification stack over HTTP for the
// operator console (real consensus, finality, signing, and bond — no mocks).
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"tessera/agent/internal/api"
	"tessera/agent/internal/config"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(envPath())
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	srv, err := api.New(context.Background(), cfg, log)
	if err != nil {
		log.Error("build server", "err", err)
		os.Exit(1)
	}
	log.Info("tessera console-api listening", "addr", cfg.APIAddr, "webOrigin", cfg.WebOrigin, "signer", srv.SignerAddress())
	if err := http.ListenAndServe(cfg.APIAddr, srv.Handler()); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
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
