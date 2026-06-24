package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	APIKey       string    `json:"api_key,omitempty"`
	Organization string    `json:"organization"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SignupRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	Organization string `json:"organization"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
