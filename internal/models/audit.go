package models

import "time"

// AuditEvent is an immutable, append-only record of a security-relevant
// action. OrganizationID is nil for global events (none are recorded today);
// UserID is the acting account, kept even after the user is removed so the
// trail is never rewritten.
type AuditEvent struct {
	ID             int64          `json:"id"`
	OrganizationID *int64         `json:"organization_id,omitempty"`
	UserID         *int64         `json:"user_id,omitempty"`
	ActorEmail     string         `json:"actor_email,omitempty"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resource_type,omitempty"`
	ResourceID     string         `json:"resource_id,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	IP             string         `json:"ip,omitempty"`
	RequestID      string         `json:"request_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// AuditFilter narrows an audit query. Nil values mean "no filter".
type AuditFilter struct {
	UserID   *int64
	Action   string
	Resource string
	Since    *time.Time
	Limit    int
	Offset   int
}
