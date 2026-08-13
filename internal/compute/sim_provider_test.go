package compute

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

type fakeSimStateStore struct {
	states map[string]string
	ats    map[string]time.Time
	getErr error
	setErr error
}

func newFakeSimStateStore() *fakeSimStateStore {
	return &fakeSimStateStore{states: map[string]string{}, ats: map[string]time.Time{}}
}

func (f *fakeSimStateStore) Get(ctx context.Context, providerID string) (string, time.Time, error) {
	if f.getErr != nil {
		return "", time.Time{}, f.getErr
	}
	if s, ok := f.states[providerID]; ok {
		return s, f.ats[providerID], nil
	}
	return "", time.Time{}, database.ErrNotFound
}

func (f *fakeSimStateStore) Set(ctx context.Context, providerID, desiredState string, transitionAt time.Time) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.states[providerID] = desiredState
	f.ats[providerID] = transitionAt
	return nil
}

func newTestSimProvider() (*SimProvider, *fakeSimStateStore, *time.Time) {
	store := newFakeSimStateStore()
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p := NewSimProvider(store, time.Second, time.Second)
	p.now = func() time.Time { return clock }
	return p, store, &clock
}

func TestSimProvider_ProvisionReachesRunningAfterDelay(t *testing.T) {
	p, _, clock := newTestSimProvider()

	pi, err := p.Provision(context.Background(), ProviderSpec{ProviderID: "i-1"})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if pi.ProviderID != "i-1" || pi.State != models.InstanceStatusPending {
		t.Fatalf("unexpected provision result: %+v", pi)
	}

	if st, _ := p.GetState(context.Background(), "i-1"); st != models.InstanceStatusPending {
		t.Fatalf("expected pending before delay, got %q", st)
	}

	*clock = clock.Add(2 * time.Second)
	if st, _ := p.GetState(context.Background(), "i-1"); st != models.InstanceStatusRunning {
		t.Fatalf("expected running after delay, got %q", st)
	}
}

func TestSimProvider_StopReachesStoppedAfterDelay(t *testing.T) {
	p, _, clock := newTestSimProvider()

	if err := p.Stop(context.Background(), "i-1"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if st, _ := p.GetState(context.Background(), "i-1"); st != models.InstanceStatusStopping {
		t.Fatalf("expected stopping before delay, got %q", st)
	}

	*clock = clock.Add(2 * time.Second)
	if st, _ := p.GetState(context.Background(), "i-1"); st != models.InstanceStatusStopped {
		t.Fatalf("expected stopped after delay, got %q", st)
	}
}

func TestSimProvider_StartReprovisions(t *testing.T) {
	p, _, clock := newTestSimProvider()

	if err := p.Start(context.Background(), "i-1"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if st, _ := p.GetState(context.Background(), "i-1"); st != models.InstanceStatusPending {
		t.Fatalf("expected pending after start, got %q", st)
	}

	*clock = clock.Add(2 * time.Second)
	if st, _ := p.GetState(context.Background(), "i-1"); st != models.InstanceStatusRunning {
		t.Fatalf("expected running after reprovision delay, got %q", st)
	}
}

func TestSimProvider_TerminateIsImmediate(t *testing.T) {
	p, _, _ := newTestSimProvider()

	if err := p.Terminate(context.Background(), "i-1"); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	if st, _ := p.GetState(context.Background(), "i-1"); st != models.InstanceStatusTerminated {
		t.Fatalf("expected terminated immediately, got %q", st)
	}
}

func TestSimProvider_GetStateUnknown(t *testing.T) {
	p, _, _ := newTestSimProvider()

	if _, err := p.GetState(context.Background(), "i-missing"); !errors.Is(err, ErrProviderStateNotFound) {
		t.Fatalf("expected ErrProviderStateNotFound, got %v", err)
	}
}

func TestSimProvider_PropagatesStoreErrors(t *testing.T) {
	p, store, _ := newTestSimProvider()
	store.setErr = errors.New("connection refused")

	if _, err := p.Provision(context.Background(), ProviderSpec{ProviderID: "i-1"}); err == nil {
		t.Fatal("expected provision to propagate store error")
	}

	store.getErr = errors.New("connection refused")
	if _, err := p.GetState(context.Background(), "i-1"); err == nil {
		t.Fatal("expected GetState to propagate store error")
	}
}
