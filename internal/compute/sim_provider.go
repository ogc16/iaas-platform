package compute

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

// ErrProviderStateNotFound is returned when a provider has no record for the
// requested instance (for example after provider-side data loss).
var ErrProviderStateNotFound = errors.New("provider state not found")

// SimStateStore persists the simulation provider's desired state and when a
// transition was requested. The database-backed implementation makes the
// simulation restart-safe: desired states survive a server restart and
// continue to advance by wall clock.
type SimStateStore interface {
	Get(ctx context.Context, providerID string) (desiredState string, transitionAt time.Time, err error)
	Set(ctx context.Context, providerID string, desiredState string, transitionAt time.Time) error
}

// SimProvider is a Provider implementation that models the asynchronous
// behavior of a real infrastructure backend without standing one up:
//
//   - Provision/Start take provisionDelay to reach running (pending first).
//   - Stop takes stopDelay to reach stopped (stopping first).
//   - Terminate is immediate.
//
// All transitions are driven by the wall clock, so the lifecycle is
// believable while remaining fully deterministic and testable.
type SimProvider struct {
	store          SimStateStore
	provisionDelay time.Duration
	stopDelay      time.Duration
	now            func() time.Time
}

func NewSimProvider(store SimStateStore, provisionDelay, stopDelay time.Duration) *SimProvider {
	return &SimProvider{
		store:          store,
		provisionDelay: provisionDelay,
		stopDelay:      stopDelay,
		now:            time.Now,
	}
}

func (p *SimProvider) Name() string { return "sim" }

func (p *SimProvider) Provision(ctx context.Context, spec ProviderSpec) (*ProviderInstance, error) {
	if err := p.store.Set(ctx, spec.ProviderID, models.InstanceStatusPending, p.now().UTC()); err != nil {
		return nil, fmt.Errorf("sim provision: %w", err)
	}
	return &ProviderInstance{ProviderID: spec.ProviderID, State: models.InstanceStatusPending}, nil
}

func (p *SimProvider) Start(ctx context.Context, providerID string) error {
	if err := p.store.Set(ctx, providerID, models.InstanceStatusPending, p.now().UTC()); err != nil {
		return fmt.Errorf("sim start: %w", err)
	}
	return nil
}

func (p *SimProvider) Stop(ctx context.Context, providerID string) error {
	if err := p.store.Set(ctx, providerID, models.InstanceStatusStopping, p.now().UTC()); err != nil {
		return fmt.Errorf("sim stop: %w", err)
	}
	return nil
}

func (p *SimProvider) Terminate(ctx context.Context, providerID string) error {
	if err := p.store.Set(ctx, providerID, models.InstanceStatusTerminated, p.now().UTC()); err != nil {
		return fmt.Errorf("sim terminate: %w", err)
	}
	return nil
}

func (p *SimProvider) GetState(ctx context.Context, providerID string) (string, error) {
	desired, transitionAt, err := p.store.Get(ctx, providerID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return "", ErrProviderStateNotFound
		}
		return "", fmt.Errorf("get provider state: %w", err)
	}

	now := p.now().UTC()
	switch desired {
	case models.InstanceStatusPending:
		if now.Sub(transitionAt) >= p.provisionDelay {
			return models.InstanceStatusRunning, nil
		}
		return models.InstanceStatusPending, nil
	case models.InstanceStatusStopping:
		if now.Sub(transitionAt) >= p.stopDelay {
			return models.InstanceStatusStopped, nil
		}
		return models.InstanceStatusStopping, nil
	case models.InstanceStatusTerminated:
		return models.InstanceStatusTerminated, nil
	default:
		return "", fmt.Errorf("unexpected desired state %q", desired)
	}
}
