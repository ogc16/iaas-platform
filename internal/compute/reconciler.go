package compute

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/ogc16/iaas-platform/internal/audit"
	"github.com/ogc16/iaas-platform/internal/models"
	"github.com/ogc16/iaas-platform/internal/webhooks"
)

// Reconciler advances instance states to match what the provider reports.
// Transient states (pending, stopping, terminating) settle here, which is what
// makes lifecycle transitions asynchronous and believable.
type Reconciler struct {
	store    InstanceStore
	provider Provider
	logger   *slog.Logger
	auditer  audit.Recorder
	emitter  webhooks.Emitter
}

func NewReconciler(store InstanceStore, provider Provider, logger *slog.Logger) *Reconciler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reconciler{store: store, provider: provider, logger: logger}
}

// SetObservability wires optional audit recording and webhook emission into
// the reconciler. System-originated changes carry no user id or request
// metadata.
func (r *Reconciler) SetObservability(auditer audit.Recorder, emitter webhooks.Emitter) {
	r.auditer = auditer
	r.emitter = emitter
}

// notify records a status change and emits a webhook event. It is best-effort:
// observability failures must never break reconciliation.
func (r *Reconciler) notify(ctx context.Context, inst *models.ComputeInstance, to string) {
	orgID := inst.OrganizationID
	audit.Record(ctx, r.auditer, &models.AuditEvent{
		OrganizationID: &orgID,
		Action:         audit.ActionInstanceStatus,
		ResourceType:   "instance",
		ResourceID:     strconv.FormatInt(inst.ID, 10),
		Metadata:       map[string]any{"from": inst.Status, "to": to, "name": inst.Name},
	})
	webhooks.Emit(ctx, r.emitter, orgID, webhooks.EventInstanceUpdated, inst)
}

// Tick reconciles all non-terminal instances against the provider. It returns
// the number of instances whose state changed.
func (r *Reconciler) Tick(ctx context.Context) (int, error) {
	instances, err := r.store.ListActive(ctx)
	if err != nil {
		return 0, err
	}

	changed := 0
	for i := range instances {
		inst := instances[i]
		state, err := r.provider.GetState(ctx, inst.ProviderID)
		if err != nil {
			if errors.Is(err, ErrProviderStateNotFound) {
				if inst.Status != models.InstanceStatusFailed {
					if uerr := r.store.UpdateStatus(ctx, inst.ID, models.InstanceStatusFailed); uerr != nil {
						r.logger.Error("reconcile: marking failed", "instance_id", inst.ID, "error", uerr)
						continue
					}
					r.logger.Warn("reconcile: provider state lost, instance failed",
						"instance_id", inst.ID, "provider_id", inst.ProviderID)
					r.notify(ctx, &inst, models.InstanceStatusFailed)
					changed++
				}
				continue
			}
			r.logger.Error("reconcile: provider state query failed",
				"instance_id", inst.ID, "provider_id", inst.ProviderID, "error", err)
			continue
		}

		if state != inst.Status {
			if uerr := r.store.UpdateStatus(ctx, inst.ID, state); uerr != nil {
				r.logger.Error("reconcile: status update failed", "instance_id", inst.ID, "error", uerr)
				continue
			}
			r.logger.Info("reconcile: instance state changed",
				"instance_id", inst.ID, "from", inst.Status, "to", state)
			r.notify(ctx, &inst, state)
			changed++
		}
	}
	return changed, nil
}

// Run ticks every interval until the context is cancelled.
func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	r.logger.Info("reconciler started", "interval", interval.String())
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("reconciler stopped")
			return
		case <-ticker.C:
			if _, err := r.Tick(ctx); err != nil {
				r.logger.Error("reconcile tick failed", "error", err)
			}
		}
	}
}
