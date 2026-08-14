package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	APIKey       string    `json:"-"`
	Organization string    `json:"organization"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SignupRequest struct {
	Email        string `json:"email" validate:"required,email"`
	Password     string `json:"password" validate:"required,min=8"`
	Name         string `json:"name" validate:"required,min=2,max=64"`
	Organization string `json:"organization" validate:"omitempty,max=64"`
	// OrgSlug, when set, requests to join an existing organization. The
	// organization's admin must accept the request before access is granted.
	OrgSlug string `json:"org_slug" validate:"omitempty,min=3,max=32"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// ForgotPasswordRequest requests a password reset link for an account. The
// endpoint always reports success for valid requests so callers cannot probe
// which emails have accounts.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest completes a password reset with a token from the
// password reset email. Tokens are single-use and expire.
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required,min=20"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// PasswordReset is a single-use, expiring password reset token. Only the
// SHA-256 hash of the token is stored; the raw token is delivered by email.
type PasswordReset struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type AuthResponse struct {
	Token string `json:"token"`
	// APIKey is the raw API key, returned only once at signup. It is never
	// stored; the database keeps a SHA-256 digest instead.
	APIKey string `json:"api_key,omitempty"`
	// OrgJoinPending is true when the account was created with an org_slug and
	// a join request is awaiting the organization admin's approval.
	OrgJoinPending bool `json:"org_join_pending,omitempty"`
	User           User `json:"user"`
}
