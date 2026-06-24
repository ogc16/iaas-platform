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
	ID             int64     `json:"id"`
	OrganizationID int64     `json:"organization_id"`
	UserID         int64     `json:"user_id"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateOrgRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type InviteMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}
