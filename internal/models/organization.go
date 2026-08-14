package models

import "time"

type Organization struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OrgMember struct {
	ID             int64      `json:"id"`
	OrganizationID int64      `json:"organization_id"`
	UserID         int64      `json:"user_id"`
	Name           string     `json:"name"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	SuspendedUntil *time.Time `json:"suspended_until,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// IsSuspended reports whether the member's access is currently revoked. A
// suspension automatically expires once suspended_until passes, restoring the
// member's rights without any admin action.
func (m *OrgMember) IsSuspended(now time.Time) bool {
	return m.SuspendedUntil != nil && m.SuspendedUntil.After(now)
}

// SuspendMemberRequest is the body for temporarily revoking a member's
// access. Days must be between 1 and 365; rights are restored automatically
// once the period elapses.
type SuspendMemberRequest struct {
	Days int `json:"days" validate:"required,min=1,max=365"`
}

type CreateOrgRequest struct {
	Name string `json:"name" validate:"required,min=3,max=64"`
	Slug string `json:"slug" validate:"required,min=3,max=32"`
}

type InviteMemberRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"omitempty,oneof=admin member"`
}

// JoinRequest is a request by a user to join an organization, created at
// signup when an existing org slug is supplied. Access is only granted once
// an organization admin accepts it.
type JoinRequest struct {
	ID             int64     `json:"id"`
	OrganizationID int64     `json:"organization_id"`
	UserID         int64     `json:"user_id"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
}
