// Command requester hires the Tessera service and drives one order to completion,
// then prints the delivered AgenticVerificationProof. It is a second-agent buyer
// (CAP forbids an agent hiring its own service).
//
// It is WebSocket-driven: after NegotiateOrder it waits for the buyer-side
// order_created event (which carries the OrderID), pays, then waits for
// order_completed and fetches the delivery.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	croo "github.com/CROO-Network/go-sdk"

	"tessera/agent/internal/config"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	// Keep the SDK's own (very chatty, and key-leaking) logs at WARN.
	sdkLog := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	txHash := flag.String("tx", "0x0000000000000000000000000000000000000000000000000000000000000001", "Base tx hash to verify")
	address := flag.String("addr", "0x4200000000000000000000000000000000000006", "address involved in the event")
	blockNumber := flag.Int64("block", 12000000, "block number that should contain the tx")
	chainID := flag.Int64("chain", 8453, "EVM chain id (Base = 8453)")
	timeout := flag.Duration("timeout", 3*time.Minute, "overall timeout")
	serviceOverride := flag.String("service", "", "service id to hire (default SERVICE_ID from .env)")
	keySel := flag.String("key", "requester", "which SDK key to hire with: requester (buyer) | provider (Teressa hires a sub-agent)")
	flag.Parse()

	cfg, err := config.Load(envPath())
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	serviceID := cfg.ServiceID
	if *serviceOverride != "" {
		serviceID = *serviceOverride
	}
	if serviceID == "" {
		log.Error("service id required (set SERVICE_ID in .env or pass -service)")
		os.Exit(1)
	}

	// The buyer hires with REQUESTER_SDK_KEY; -key provider lets Teressa itself
	// hire another agent's service (A2A depth), using its provider key as a buyer.
	var requesterKey string
	if *keySel == "provider" {
		requesterKey = cfg.CrooSDKKey
	} else {
		requesterKey = cfg.RequesterSDKKey
		if requesterKey == "" {
			requesterKey = cfg.CrooSDKKey
		}
	}
	if requesterKey == "" {
		log.Error("no requester key: set REQUESTER_SDK_KEY (second agent) in .env")
		os.Exit(1)
	}
	// CAP forbids hiring your OWN service. Only warn when the selected key owns the
	// target service (buyer key with its own service, or provider key + Teressa's service).
	if requesterKey == cfg.CrooSDKKey && serviceID == cfg.ServiceID {
		log.Warn("hiring own service with the provider key — CAP will reject with 'cannot negotiate own service'")
	}

	rpc := "https://mainnet.base.org"
	if len(cfg.BaseRPCURLs) > 0 {
		rpc = cfg.BaseRPCURLs[0]
	}
	client, err := croo.NewAgentClient(croo.Config{
		BaseURL: cfg.CrooAPIURL,
		WSURL:   cfg.CrooWSURL,
		RPCURL:  rpc,
		Logger:  sdkLog,
	}, requesterKey)
	if err != nil {
		log.Error("new agent client", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	stream, err := client.ConnectWebSocket(ctx)
	if err != nil {
		log.Error("connect websocket", "err", err)
		os.Exit(1)
	}
	defer stream.Close()

	var (
		mu      sync.Mutex
		negID   string
		orderID string
	)
	done := make(chan error, 1)
	finish := func(err error) {
		select {
		case done <- err:
		default:
		}
	}

	stream.On(croo.EventOrderCreated, func(e croo.Event) {
		mu.Lock()
		match := e.NegotiationID == negID
		if match {
			orderID = e.OrderID
		}
		mu.Unlock()
		if !match {
			return
		}
		log.Info("order created — paying", "orderId", e.OrderID)
		if _, err := client.PayOrder(ctx, e.OrderID); err != nil {
			finish(fmt.Errorf("PayOrder: %w", err))
			return
		}
		log.Info("paid — USDC locked in CAPVault", "orderId", e.OrderID)
	})
	stream.On(croo.EventOrderPaid, func(e croo.Event) {
		log.Info("order paid", "orderId", e.OrderID)
	})
	stream.On(croo.EventOrderRejected, func(e croo.Event) {
		mu.Lock()
		ours := e.OrderID == orderID
		mu.Unlock()
		if ours {
			finish(fmt.Errorf("order rejected: %s", e.Reason))
		}
	})
	stream.On(croo.EventOrderCompleted, func(e croo.Event) {
		mu.Lock()
		ours := e.OrderID == orderID
		mu.Unlock()
		if !ours {
			return
		}
		delivery, err := client.GetDelivery(ctx, e.OrderID)
		if err != nil {
			finish(fmt.Errorf("GetDelivery: %w", err))
			return
		}
		fmt.Println("── delivered AgenticVerificationProof v1 ────────────────────")
		fmt.Println(pickDeliverable(delivery))
		fmt.Println("── contentHash (on-chain):", delivery.ContentHash)
		fmt.Println("── live round-trip complete ✓ ───────────────────────────────")
		finish(nil)
	})

	requirements, _ := json.Marshal(map[string]any{
		"chainId":     *chainID,
		"address":     *address,
		"txHash":      *txHash,
		"blockNumber": *blockNumber,
	})

	log.Info("negotiating", "serviceId", serviceID, "key", *keySel)
	neg, err := client.NegotiateOrder(ctx, croo.NegotiateOrderRequest{
		ServiceID:    serviceID,
		Requirements: string(requirements),
	})
	if err != nil {
		log.Error("NegotiateOrder failed", "err", err)
		os.Exit(1)
	}
	mu.Lock()
	negID = neg.NegotiationID
	mu.Unlock()
	log.Info("negotiation created — waiting for order_created", "negotiationId", neg.NegotiationID)

	select {
	case err := <-done:
		if err != nil {
			log.Error("round-trip failed", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		log.Error("timed out waiting for order lifecycle", "err", ctx.Err())
		os.Exit(1)
	}
}

func pickDeliverable(d *croo.Delivery) string {
	// We deliver as Text; prefer it. Treat empty/placeholder schema ("[]") as absent.
	if txt := d.DeliverableText; txt != "" {
		return txt
	}
	if s := d.DeliverableSchema; s != "" && s != "[]" {
		return s
	}
	return "(empty deliverable)"
}

func envPath() string {
	for _, p := range []string{".env", "../.env", "../../.env"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ".env"
}
