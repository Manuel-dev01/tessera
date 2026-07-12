// Package cap abstracts the CAP order transport so Tessera's order logic runs
// unchanged against either the live CROO SDK or an in-memory loopback harness.
package cap

import (
	"context"

	"tessera/agent/internal/handler"
)

// Transport drives the CAP order lifecycle and dispatches paid orders to h.
// Implementations: LiveTransport (real CROO SDK) and Loopback (offline demo).
type Transport interface {
	// Run blocks until ctx is cancelled or a fatal transport error occurs,
	// invoking h for each paid order and delivering the returned proof.
	Run(ctx context.Context, h handler.OrderHandler) error
}
