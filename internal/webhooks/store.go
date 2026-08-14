package webhooks

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

var (
	// ErrNotAdmin is returned when a non-admin caller attempts a management
	// action on an organization's webhooks.
	ErrNotAdmin = errors.New("only admins can manage webhooks")
	// ErrNotFound is returned when the webhook does not exist or does not
	// belong to the organization.
	ErrNotFound = errors.New("webhook not found")
	// ErrInvalidURL is returned when the endpoint is not an absolute http(s)
	// URL.
	ErrInvalidURL = errors.New("webhook url must be an absolute http(s) URL")
	// ErrInvalidEvent is returned when a request references an event that is
	// not subscribable.
	ErrInvalidEvent = errors.New("unknown webhook event")
	// ErrEmptyEvents is returned when a request does not subscribe to any
	// event.
	ErrEmptyEvents = errors.New("at least one event is required")
)

// Event aliases for the domain names used by emitters. They mirror the
// canonical names in models so services can reference webhooks.* uniformly.
const (
	EventInstanceCreated         = models.EventInstanceCreated
	EventInstanceUpdated         = models.EventInstanceUpdated
	EventInstanceTerminated      = models.EventInstanceTerminated
	EventOrgMemberAdded          = models.EventMemberAdded
	EventOrgMemberRemoved        = models.EventMemberRemoved
	EventOrgMemberSuspended      = models.EventMemberSuspended
	EventOrgMemberUnsuspended    = models.EventMemberUnsuspended
	EventOrgJoinRequestAccepted  = models.EventJoinRequestAccepted
	EventOrgJoinRequestRevoked   = models.EventJoinRequestRevoked
	EventBillingUsageRecorded    = models.EventUsageRecorded
	EventBillingInvoiceGenerated = models.EventInvoiceGenerated
)

// WebhookStore is the persistence boundary for webhook endpoints.
type WebhookStore interface {
	Create(ctx context.Context, w *models.Webhook) error
	FindByID(ctx context.Context, id int64) (*models.Webhook, error)
	ListByOrg(ctx context.Context, orgID int64, limit, offset int) ([]models.Webhook, error)
	CountByOrg(ctx context.Context, orgID int64) (int64, error)
	Update(ctx context.Context, w *models.Webhook) error
	Delete(ctx context.Context, id int64) error
}

// DeliveryStore is the persistence boundary for queued deliveries.
type DeliveryStore interface {
	// CreateDeliveries enqueues one delivery for every active webhook in the
	// organization subscribed to the event, returning how many were created.
	CreateDeliveries(ctx context.Context, orgID int64, event string, payload []byte) (int, error)
	// CreateSingleDelivery enqueues one delivery for a specific webhook
	// (used by the ping endpoint).
	CreateSingleDelivery(ctx context.Context, webhookID int64, event string, payload []byte) (int64, error)
	// ListDue returns deliveries that are pending or failed and due for
	// another attempt.
	ListDue(ctx context.Context, limit int) ([]models.WebhookDelivery, error)
	// MarkDelivered records a successful delivery.
	MarkDelivered(ctx context.Context, id int64) error
	// MarkFailed records a failed attempt, incrementing the attempt counter
	// and scheduling the next attempt (or marking the delivery failed when
	// status is WebhookDeliveryFailed).
	MarkFailed(ctx context.Context, id int64, status, lastErr string, nextAttemptAt time.Time) error
}

// MembershipStore lets the service resolve the caller's role.
type MembershipStore interface {
	FindMember(ctx context.Context, orgID, userID int64) (*models.OrgMember, error)
}

// Emitter enqueues a delivery for every active webhook subscribed to an event.
// It is the hook services use to publish domain events.
type Emitter interface {
	Emit(ctx context.Context, orgID int64, event string, payload any) error
}

// validateRequest checks that the URL is absolute http(s) and every event is
// subscribable.
func validateRequest(urlStr string, events []string) error {
	u, err := url.Parse(urlStr)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrInvalidURL
	}
	if len(events) == 0 {
		return ErrEmptyEvents
	}
	for _, e := range events {
		if !validEvent(e) {
			return ErrInvalidEvent
		}
	}
	return nil
}

func validEvent(event string) bool {
	for _, e := range models.AllEvents {
		if e == event {
			return true
		}
	}
	return false
}

func requireAdmin(ctx context.Context, members MembershipStore, orgID, userID int64) error {
	member, err := members.FindMember(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return ErrNotAdmin
		}
		return err
	}
	if member.Role != "admin" {
		return ErrNotAdmin
	}
	return nil
}

func normalizeEvents(events []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, e := range events {
		e = strings.TrimSpace(e)
		if e != "" && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out
}
