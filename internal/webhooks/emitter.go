package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// Emit enqueues a delivery for every active webhook in the org subscribed to
// the event. It is a no-op when em is nil so optional instrumentation never
// breaks the caller. Payload is marshalled to JSON at enqueue time.
func Emit(ctx context.Context, em Emitter, orgID int64, event string, payload any) {
	if em == nil {
		return
	}
	if err := em.Emit(ctx, orgID, event, payload); err != nil {
		// Emission is fire-and-forget: a delivery queue failure must not fail
		// the underlying domain operation. The dispatcher surfaces failures
		// per delivery; log here so operators can notice systemic problems.
		slog.Error("webhooks: emit failed", "org_id", orgID, "event", event, "error", err)
	}
}

// queueEmitter is the default Emitter implementation backed by the delivery
// store.
type queueEmitter struct {
	deliver DeliveryStore
}

func NewEmitter(deliver DeliveryStore) Emitter {
	return &queueEmitter{deliver: deliver}
}

func (e *queueEmitter) Emit(ctx context.Context, orgID int64, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if _, err := e.deliver.CreateDeliveries(ctx, orgID, event, data); err != nil {
		return fmt.Errorf("enqueue deliveries: %w", err)
	}
	return nil
}
