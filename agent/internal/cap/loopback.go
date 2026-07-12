package cap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"tessera/agent/internal/handler"
)

// Loopback is an in-memory Transport that simulates the full CAP order lifecycle
// without any credentials or network. It fabricates a sample order, drives it
// through negotiation -> paid -> deliver -> completed, and prints each step.
// This is the Phase 0 deliverable: it proves the order-handling code path end to
// end offline, using the exact same OrderHandler the live transport uses.
type Loopback struct {
	Log *slog.Logger
	// SampleRequirements is the Requirements JSON a simulated requester submits.
	SampleRequirements json.RawMessage
}

// DefaultSampleRequirements is a real Base transaction, so the offline loopback
// demo exercises genuine RPC verification (not a placeholder) and returns a
// verified:true, signed proof.
var DefaultSampleRequirements = json.RawMessage(`{
  "chainId": 8453,
  "address": "0xdd734d464346e6e9a80de2fBf9c55ae28758CC53",
  "txHash": "0xbd027434e46f991849ed942594719dc2c56ad5e82fef21698ab7de586c559123",
  "blockNumber": 48321293
}`)

func (l *Loopback) Run(ctx context.Context, h handler.OrderHandler) error {
	log := l.Log
	if log == nil {
		log = slog.Default()
	}
	req := l.SampleRequirements
	if len(req) == 0 {
		req = DefaultSampleRequirements
	}

	const (
		negotiationID = "neg_loopback_0001"
		orderID       = "ord_loopback_0001"
		serviceID     = "svc_tessera_verify"
	)

	fmt.Println("── CAP loopback: simulated order lifecycle ──────────────────")
	log.Info("event", "type", "order_negotiation_created", "negotiationId", negotiationID, "serviceId", serviceID)
	log.Info("provider action", "call", "AcceptNegotiation", "negotiationId", negotiationID)
	log.Info("event", "type", "order_created", "orderId", orderID)
	log.Info("requester action", "call", "PayOrder", "orderId", orderID, "note", "USDC locked in CAPVault")
	log.Info("event", "type", "order_paid", "orderId", orderID)

	fmt.Println("── requester Requirements ───────────────────────────────────")
	fmt.Println(indentJSON(req))

	deliverable, err := h(ctx, req)
	if err != nil {
		log.Error("handler failed", "err", err)
		log.Info("provider action", "call", "RejectOrder", "orderId", orderID, "reason", err.Error())
		return fmt.Errorf("loopback handler: %w", err)
	}

	log.Info("provider action", "call", "DeliverOrder", "orderId", orderID, "deliverableType", "schema")
	fmt.Println("── delivered AgenticVerificationProof v1 ────────────────────")
	fmt.Println(string(deliverable))
	log.Info("event", "type", "order_completed", "orderId", orderID)
	fmt.Println("── round-trip complete ✓ ────────────────────────────────────")
	return nil
}

func indentJSON(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(b)
}
