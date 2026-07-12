package cap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

	croo "github.com/CROO-Network/go-sdk"

	"tessera/agent/internal/handler"
)

// LiveTransport is the real CAP transport backed by the CROO Go SDK. It is the
// ONLY file in the codebase that imports github.com/CROO-Network/go-sdk, so all
// SDK-specific assumptions are quarantined here. Compiles today; goes live once
// an agent + service are registered on the dashboard and CROO_SDK_KEY is set.
//
// Lifecycle wired per the verified SDK API:
//
//	order_negotiation_created -> AcceptNegotiation (captures Requirements)
//	order_paid                -> OrderHandler -> DeliverOrder(schema)
//	order_completed           -> log
type LiveTransport struct {
	client *croo.AgentClient
	log    *slog.Logger

	mu   sync.Mutex
	reqs map[string]json.RawMessage // orderID -> Requirements JSON, captured at accept
}

// NewLiveTransport builds the SDK client. baseURL/wsURL/rpcURL come from config;
// sdkKey is the provider's dashboard SDK-Key.
func NewLiveTransport(baseURL, wsURL, rpcURL, sdkKey string, log *slog.Logger) (*LiveTransport, error) {
	if log == nil {
		log = slog.Default()
	}
	// Keep the SDK's own logs at WARN: they are very chatty and log the SDK key
	// in the WS URL at INFO. Our own handler logs (via `log`) stay at INFO.
	sdkLog := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client, err := croo.NewAgentClient(croo.Config{
		BaseURL: baseURL,
		WSURL:   wsURL,
		RPCURL:  rpcURL,
		Logger:  sdkLog,
	}, sdkKey)
	if err != nil {
		return nil, fmt.Errorf("new agent client: %w", err)
	}
	return &LiveTransport{
		client: client,
		log:    log,
		reqs:   make(map[string]json.RawMessage),
	}, nil
}

func (t *LiveTransport) Run(ctx context.Context, h handler.OrderHandler) error {
	stream, err := t.client.ConnectWebSocket(ctx)
	if err != nil {
		return fmt.Errorf("connect websocket: %w", err)
	}
	defer stream.Close()

	stream.On(croo.EventNegotiationCreated, func(e croo.Event) {
		t.onNegotiation(ctx, e)
	})
	stream.On(croo.EventOrderPaid, func(e croo.Event) {
		t.onPaid(ctx, e, h)
	})
	stream.On(croo.EventOrderCompleted, func(e croo.Event) {
		t.log.Info("event", "type", "order_completed", "orderId", e.OrderID)
	})

	t.log.Info("provider online — waiting for orders", "ws", "connected")
	<-ctx.Done()
	if err := stream.Err(); err != nil {
		return fmt.Errorf("event stream: %w", err)
	}
	return ctx.Err()
}

// onNegotiation accepts the negotiation and stores its Requirements keyed by the
// resulting orderID, so the paid handler can read the requester's input.
func (t *LiveTransport) onNegotiation(ctx context.Context, e croo.Event) {
	t.log.Info("event", "type", "order_negotiation_created", "negotiationId", e.NegotiationID)
	res, err := t.client.AcceptNegotiation(ctx, e.NegotiationID)
	if err != nil {
		t.log.Error("AcceptNegotiation failed", "negotiationId", e.NegotiationID, "err", err)
		return
	}
	if res.Order != nil && res.Negotiation != nil {
		t.mu.Lock()
		t.reqs[res.Order.OrderID] = json.RawMessage(res.Negotiation.Requirements)
		t.mu.Unlock()
		t.log.Info("accepted", "orderId", res.Order.OrderID)
	}
}

// onPaid runs the OrderHandler over the captured Requirements and delivers the
// resulting AVP proof. Requirements are recovered from the accept-time cache, or
// re-fetched via GetNegotiation as a fallback.
func (t *LiveTransport) onPaid(ctx context.Context, e croo.Event, h handler.OrderHandler) {
	t.log.Info("event", "type", "order_paid", "orderId", e.OrderID)

	t.mu.Lock()
	req, ok := t.reqs[e.OrderID]
	t.mu.Unlock()
	if !ok {
		if e.NegotiationID != "" {
			if n, err := t.client.GetNegotiation(ctx, e.NegotiationID); err == nil {
				req = json.RawMessage(n.Requirements)
				ok = true
			}
		}
	}
	if !ok || len(req) == 0 {
		t.rejectOrder(ctx, e.OrderID, "requirements unavailable")
		return
	}

	deliverable, err := h(ctx, req)
	if err != nil {
		t.log.Error("handler failed", "orderId", e.OrderID, "err", err)
		t.rejectOrder(ctx, e.OrderID, err.Error())
		return
	}

	// Deliver as Text: the payload is the full AVP JSON, but CAP carries it as an
	// opaque string rather than structurally validating it against the dashboard
	// deliverable schema (whose nested numeric fields are brittle to match).
	if _, err := t.client.DeliverOrder(ctx, e.OrderID, croo.DeliverOrderRequest{
		DeliverableType: croo.DeliverableText,
		DeliverableText: string(deliverable),
	}); err != nil {
		t.log.Error("DeliverOrder failed", "orderId", e.OrderID, "err", err)
		return
	}
	t.log.Info("delivered", "orderId", e.OrderID, "deliverableType", croo.DeliverableText)

	t.mu.Lock()
	delete(t.reqs, e.OrderID)
	t.mu.Unlock()
}

func (t *LiveTransport) rejectOrder(ctx context.Context, orderID, reason string) {
	if err := t.client.RejectOrder(ctx, orderID, reason); err != nil {
		t.log.Error("RejectOrder failed", "orderId", orderID, "err", err)
	}
}
