package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ogc16/iaas-platform/internal/metrics"
	"github.com/ogc16/iaas-platform/internal/models"
)

const (
	defaultDispatchInterval = 5 * time.Second
	defaultDispatchLimit    = 50
	dispatchHTTPTimeout     = 10 * time.Second
	backoffBase             = 2 * time.Second
	backoffMax              = 5 * time.Minute
)

// envelope is the payload POSTed to a webhook endpoint. It wraps the event's
// data so receivers always see a stable shape regardless of the event.
type envelope struct {
	ID             int64           `json:"id"`
	Event          string          `json:"event"`
	OrganizationID int64           `json:"organization_id"`
	Timestamp      time.Time       `json:"timestamp"`
	Data           json.RawMessage `json:"data"`
}

// Dispatcher is the worker that delivers queued webhook payloads. It polls
// the delivery store for due rows, POSTs the signed envelope to the endpoint,
// and retries with exponential backoff until max_attempts is exhausted.
type Dispatcher struct {
	deliver   DeliveryStore
	client    *http.Client
	logger    *slog.Logger
	interval  time.Duration
	limit     int
	delivered *metrics.Sample
	failed    *metrics.Sample
	retried   *metrics.Sample
}

type DispatcherOption func(*Dispatcher)

// WithDispatchInterval overrides the poll interval (default 5s).
func WithDispatchInterval(interval time.Duration) DispatcherOption {
	return func(d *Dispatcher) { d.interval = interval }
}

// WithMetrics wires Prometheus counters for delivery outcomes.
func WithMetrics(delivered, failed, retried *metrics.Sample) DispatcherOption {
	return func(d *Dispatcher) {
		d.delivered = delivered
		d.failed = failed
		d.retried = retried
	}
}

func NewDispatcher(deliver DeliveryStore, logger *slog.Logger, opts ...DispatcherOption) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	d := &Dispatcher{
		deliver:  deliver,
		client:   &http.Client{Timeout: dispatchHTTPTimeout},
		logger:   logger,
		interval: defaultDispatchInterval,
		limit:    defaultDispatchLimit,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Run polls for due deliveries every interval until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	d.logger.Info("webhook dispatcher started", "interval", d.interval.String())
	for {
		select {
		case <-ctx.Done():
			d.logger.Info("webhook dispatcher stopped")
			return
		case <-ticker.C:
			if _, err := d.DispatchOnce(ctx); err != nil {
				d.logger.Error("webhook dispatch tick failed", "error", err)
			}
		}
	}
}

// DispatchOnce delivers up to d.limit due rows, returning how many were
// attempted.
func (d *Dispatcher) DispatchOnce(ctx context.Context) (int, error) {
	due, err := d.deliver.ListDue(ctx, d.limit)
	if err != nil {
		return 0, fmt.Errorf("list due deliveries: %w", err)
	}

	for i := range due {
		dl := due[i]
		d.deliverOne(ctx, &dl)
	}
	return len(due), nil
}

func (d *Dispatcher) deliverOne(ctx context.Context, dl *models.WebhookDelivery) {
	body, err := d.buildEnvelope(dl)
	if err != nil {
		d.fail(ctx, dl, err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dl.URL, bytes.NewReader(body))
	if err != nil {
		d.fail(ctx, dl, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "iaas-platform-webhook")
	req.Header.Set("X-IaaS-Event", dl.Event)
	req.Header.Set("X-IaaS-Delivery", fmt.Sprintf("%d", dl.ID))
	req.Header.Set(SignatureHeader, Sign(dl.Secret, body))

	resp, err := d.client.Do(req)
	if err != nil {
		d.fail(ctx, dl, err)
		return
	}
	defer resp.Body.Close()
	// Drain a bounded amount so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		d.fail(ctx, dl, fmt.Errorf("endpoint returned HTTP %d", resp.StatusCode))
		return
	}

	if err := d.deliver.MarkDelivered(ctx, dl.ID); err != nil {
		d.logger.Error("webhook: mark delivered failed", "delivery_id", dl.ID, "error", err)
		return
	}
	d.inc(d.delivered)
	d.logger.Info("webhook delivered", "delivery_id", dl.ID, "webhook_id", dl.WebhookID, "event", dl.Event)
}

// fail records a failed attempt. When the attempt budget is exhausted the
// delivery is marked failed; otherwise it is rescheduled with backoff.
func (d *Dispatcher) fail(ctx context.Context, dl *models.WebhookDelivery, cause error) {
	attempts := dl.Attempts + 1
	status := models.WebhookDeliveryPending
	next := time.Now().UTC().Add(backoff(attempts))
	if attempts >= dl.MaxAttempts {
		status = models.WebhookDeliveryFailed
		next = time.Now().UTC()
	}
	if err := d.deliver.MarkFailed(ctx, dl.ID, status, cause.Error(), next); err != nil {
		d.logger.Error("webhook: mark failed failed", "delivery_id", dl.ID, "error", err)
		return
	}
	if status == models.WebhookDeliveryFailed {
		d.inc(d.failed)
	} else {
		d.inc(d.retried)
	}
	d.logger.Warn("webhook delivery failed",
		"delivery_id", dl.ID, "webhook_id", dl.WebhookID, "event", dl.Event,
		"attempts", attempts, "max_attempts", dl.MaxAttempts, "error", cause.Error())
}

func (d *Dispatcher) buildEnvelope(dl *models.WebhookDelivery) ([]byte, error) {
	return json.Marshal(envelope{
		ID:             dl.ID,
		Event:          dl.Event,
		OrganizationID: dl.OrganizationID,
		Timestamp:      time.Now().UTC(),
		Data:           json.RawMessage(dl.Payload),
	})
}

func (d *Dispatcher) inc(sample *metrics.Sample) {
	if sample != nil {
		sample.Add(1)
	}
}

func backoff(attempts int) time.Duration {
	delay := backoffBase * time.Duration(1<<uint(attempts-1))
	if delay > backoffMax {
		return backoffMax
	}
	return delay
}
