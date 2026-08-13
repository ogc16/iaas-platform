package compute

import (
	"context"
	"errors"
	"testing"

	"github.com/ogc16/iaas-platform/internal/models"
)

func TestReconciler_AdvancesPendingToRunning(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusPending)
	e.provider.states[inst.ProviderID] = models.InstanceStatusRunning

	r := NewReconciler(e.instances, e.provider, nil)
	changed, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if changed != 1 {
		t.Fatalf("expected 1 change, got %d", changed)
	}
	if e.instances.instances[inst.ID].Status != models.InstanceStatusRunning {
		t.Fatalf("expected status running, got %q", e.instances.instances[inst.ID].Status)
	}
}

func TestReconciler_AdvancesStoppingToStopped(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusStopping)
	e.provider.states[inst.ProviderID] = models.InstanceStatusStopped

	r := NewReconciler(e.instances, e.provider, nil)
	if _, err := r.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if e.instances.instances[inst.ID].Status != models.InstanceStatusStopped {
		t.Fatalf("expected status stopped, got %q", e.instances.instances[inst.ID].Status)
	}
}

func TestReconciler_AdvancesTerminatingToTerminated(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusTerminating)
	e.provider.states[inst.ProviderID] = models.InstanceStatusTerminated

	r := NewReconciler(e.instances, e.provider, nil)
	if _, err := r.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if e.instances.instances[inst.ID].Status != models.InstanceStatusTerminated {
		t.Fatalf("expected status terminated, got %q", e.instances.instances[inst.ID].Status)
	}
}

func TestReconciler_MarksFailedWhenProviderStateLost(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusRunning)
	delete(e.provider.states, inst.ProviderID)

	r := NewReconciler(e.instances, e.provider, nil)
	changed, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if changed != 1 {
		t.Fatalf("expected 1 change, got %d", changed)
	}
	if e.instances.instances[inst.ID].Status != models.InstanceStatusFailed {
		t.Fatalf("expected status failed, got %q", e.instances.instances[inst.ID].Status)
	}
}

func TestReconciler_NoChangeWhenStatesMatch(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	inst := seedInstance(t, e, 1, 2, models.InstanceStatusRunning)
	e.provider.states[inst.ProviderID] = models.InstanceStatusRunning

	r := NewReconciler(e.instances, e.provider, nil)
	changed, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if changed != 0 {
		t.Fatalf("expected 0 changes, got %d", changed)
	}
}

func TestReconciler_SkipsTerminatedInstances(t *testing.T) {
	e := newTestEnv()
	e.addMember(1, 2)
	seedInstance(t, e, 1, 2, models.InstanceStatusTerminated)

	r := NewReconciler(e.instances, e.provider, nil)
	changed, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if changed != 0 {
		t.Fatalf("expected 0 changes for terminated instances, got %d", changed)
	}
}

func TestReconciler_PropagatesListError(t *testing.T) {
	e := newTestEnv()
	e.instances.listErr = errors.New("connection refused")

	r := NewReconciler(e.instances, e.provider, nil)
	if _, err := r.Tick(context.Background()); err == nil {
		t.Fatal("expected list error to propagate")
	}
}
