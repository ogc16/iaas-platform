package webhooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ogc16/iaas-platform/internal/metrics"
	"github.com/ogc16/iaas-platform/internal/models"
)

// scriptedDeliveryStore records which transitions the dispatcher requested.
type scriptedDeliveryStore struct {
	mu        sync.Mutex
	due       []models.WebhookDelivery
	delivered []int64
	failed    []failCall
}

type failCall struct {
	id      int64
	status  string
	lastErr string
}

func (s *scriptedDeliveryStore) CreateDeliveries(ctx context.Context, orgID int64, event string, payload []byte) (int, error) {
	return 0, nil
}

func (s *scriptedDeliveryStore) CreateSingleDelivery(ctx context.Context, webhookID int64, event string, payload []byte) (int64, error) {
	return 1, nil
}

func (s *scriptedDeliveryStore) ListDue(ctx context.Context, limit int) ([]models.WebhookDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.due, nil
}

func (s *scriptedDeliveryStore) MarkDelivered(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delivered = append(s.delivered, id)
	return nil
}

func (s *scriptedDeliveryStore) MarkFailed(ctx context.Context, id int64, status, lastErr string, nextAttemptAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, failCall{id: id, status: status, lastErr: lastErr})
	return nil
}

func TestDispatchOnceDeliversSignedEnvelope(t *testing.T) {
	var received struct {
		sync.Mutex
		headers   http.Header
		body      envelope
		signature string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Lock()
		defer received.Unlock()
		received.headers = r.Header
		received.signature = r.Header.Get(SignatureHeader)
		if err := json.NewDecoder(r.Body).Decode(&received.body); err != nil {
			t.Errorf("decode envelope: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &scriptedDeliveryStore{due: []models.WebhookDelivery{
		{
			ID:             11,
			WebhookID:      3,
			OrganizationID: 7,
			Event:          models.EventInstanceCreated,
			Payload:        []byte(`{"id":9}`),
			Status:         models.WebhookDeliveryPending,
			MaxAttempts:    5,
			NextAttemptAt:  time.Now().Add(-time.Minute),
			URL:            srv.URL,
			Secret:         "s3cret",
		},
	}}

	delivered := metrics.NewRegistry().NewCounter("t_delivered_total", "", nil).WithLabelValues()
	dispatcher := NewDispatcher(store, nil,
		WithMetrics(delivered, nil, nil),
		WithDispatchInterval(time.Hour),
	)

	n, err := dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("dispatch once: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 delivery attempt, got %d", n)
	}

	received.Lock()
	defer received.Unlock()
	if received.body.Event != models.EventInstanceCreated {
		t.Errorf("unexpected event %q", received.body.Event)
	}
	if received.body.OrganizationID != 7 {
		t.Errorf("unexpected org %d", received.body.OrganizationID)
	}
	if received.headers.Get("X-IaaS-Event") != models.EventInstanceCreated {
		t.Errorf("missing X-IaaS-Event header")
	}
	if received.headers.Get("X-IaaS-Delivery") != "11" {
		t.Errorf("missing X-IaaS-Delivery header")
	}
	if received.signature == "" {
		t.Fatal("missing signature header")
	}
	if !Verify("s3cret", mustJSON(t, received.body), received.signature) {
		t.Error("signature does not verify")
	}

	if len(store.delivered) != 1 || store.delivered[0] != 11 {
		t.Errorf("expected delivery 11 marked delivered, got %v", store.delivered)
	}
}

func TestDispatchOnceRetriesThenFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := &scriptedDeliveryStore{due: []models.WebhookDelivery{
		{
			ID:             11,
			WebhookID:      3,
			OrganizationID: 7,
			Event:          models.EventInstanceCreated,
			Payload:        []byte(`{}`),
			Status:         models.WebhookDeliveryPending,
			MaxAttempts:    5,
			NextAttemptAt:  time.Now().Add(-time.Minute),
			URL:            srv.URL,
			Secret:         "s3cret",
		},
	}}

	failed := metrics.NewRegistry().NewCounter("t_failed_total", "", nil).WithLabelValues()
	dispatcher := NewDispatcher(store, nil, WithMetrics(nil, failed, nil))

	if _, err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("dispatch once: %v", err)
	}

	// A 500 response within the attempt budget is a retry, not a failure.
	if len(store.failed) != 1 {
		t.Fatalf("expected 1 MarkFailed call, got %d", len(store.failed))
	}
	if store.failed[0].status != models.WebhookDeliveryPending {
		t.Errorf("expected pending (retry) status, got %q", store.failed[0].status)
	}
	if store.failed[0].lastErr == "" {
		t.Error("expected a recorded error message")
	}

	// Bump attempts past the budget: the next failure is terminal.
	store.mu.Lock()
	store.due[0].Attempts = 5
	store.mu.Unlock()
	store.failed = nil
	if _, err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if len(store.failed) != 1 || store.failed[0].status != models.WebhookDeliveryFailed {
		t.Errorf("expected terminal failure, got %v", store.failed)
	}
}

func TestBackoff(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{9, 5 * time.Minute}, // capped
		{20, 5 * time.Minute},
	}
	for _, tc := range cases {
		if got := backoff(tc.attempts); got != tc.want {
			t.Errorf("backoff(%d) = %v, want %v", tc.attempts, got, tc.want)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestEmitIsNilSafe(t *testing.T) {
	Emit(context.Background(), nil, 7, models.EventInstanceCreated, map[string]any{"a": 1})
}

func TestEmitEnqueuesDeliveries(t *testing.T) {
	store := &fakeDeliveryStore{}
	em := NewEmitter(store)
	if err := em.Emit(context.Background(), 7, models.EventInstanceCreated, map[string]any{"a": 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}
}
