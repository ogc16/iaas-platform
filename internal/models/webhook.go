package models

import "time"

// Webhook is an outbound endpoint registered by an organization to receive
// realtime notifications. Deliveries are HMAC-SHA256 signed with Secret so
// receivers can verify payload authenticity.
type Webhook struct {
	ID             int64     `json:"id"`
	OrganizationID int64     `json:"organization_id"`
	URL            string    `json:"url"`
	Secret         string    `json:"-"`
	Events         []string  `json:"events"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Webhook event names. These are the subscribable event identifiers sent in
// the X-IaaS-Event header and inside the delivery envelope.
const (
	EventInstanceCreated     = "instance.created"
	EventInstanceUpdated     = "instance.updated"
	EventInstanceTerminated  = "instance.terminated"
	EventMemberAdded         = "org.member_added"
	EventMemberRemoved       = "org.member_removed"
	EventMemberSuspended     = "org.member_suspended"
	EventMemberUnsuspended   = "org.member_unsuspended"
	EventJoinRequestAccepted = "org.join_request_accepted"
	EventJoinRequestRevoked  = "org.join_request_revoked"
	EventUsageRecorded       = "billing.usage_recorded"
	EventInvoiceGenerated    = "billing.invoice_generated"

	// EventPing is a synthetic event used by the "test" button to verify an
	// endpoint is reachable. It cannot be subscribed to.
	EventPing = "ping"
)

// AllEvents is the canonical, ordered list of subscribable events. It drives
// validation, the OpenAPI enum, and the dashboard's subscription checkboxes.
var AllEvents = []string{
	EventInstanceCreated,
	EventInstanceUpdated,
	EventInstanceTerminated,
	EventMemberAdded,
	EventMemberRemoved,
	EventMemberSuspended,
	EventMemberUnsuspended,
	EventJoinRequestAccepted,
	EventJoinRequestRevoked,
	EventUsageRecorded,
	EventInvoiceGenerated,
}

// CreateWebhookRequest is the body for registering a webhook endpoint. Secret
// is optional; when omitted the platform generates one (and it is returned
// once, in the webhook object).
type CreateWebhookRequest struct {
	URL    string   `json:"url" validate:"required,max=512"`
	Events []string `json:"events" validate:"required,max=20"`
	Secret string   `json:"secret" validate:"omitempty,min=16,max=128"`
}

// UpdateWebhookRequest is the body for updating a webhook. Only the fields
// that are set are changed; nil pointers mean "leave unchanged".
type UpdateWebhookRequest struct {
	URL    *string   `json:"url"`
	Events *[]string `json:"events"`
	Active *bool     `json:"active"`
}

// WebhookDelivery is one delivery attempt queue for an event to a webhook.
// The dispatcher worker reads rows whose next_attempt_at has passed, POSTs the
// signed envelope, and retries with exponential backoff on failure.
type WebhookDelivery struct {
	ID             int64      `json:"id"`
	WebhookID      int64      `json:"webhook_id"`
	OrganizationID int64      `json:"organization_id"`
	Event          string     `json:"event"`
	Payload        []byte     `json:"payload"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	MaxAttempts    int        `json:"max_attempts"`
	LastError      *string    `json:"last_error,omitempty"`
	NextAttemptAt  time.Time  `json:"next_attempt_at"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	// URL and Secret are populated by ListDue via a join onto webhooks so the
	// dispatcher can deliver without a second query. Never serialized.
	URL    string `json:"-"`
	Secret string `json:"-"`
}

// Webhook delivery states.
const (
	WebhookDeliveryPending   = "pending"
	WebhookDeliveryDelivered = "delivered"
	WebhookDeliveryFailed    = "failed"
)
