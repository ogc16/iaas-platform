package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ogc16/iaas-platform/internal/httpx"
	"github.com/ogc16/iaas-platform/internal/models"
	"github.com/ogc16/iaas-platform/internal/validate"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req models.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	resp, err := h.svc.Signup(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrOrgNotFound) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCreds) {
			httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	user, err := h.svc.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, user)
}

// ForgotPassword emails a reset link for the given address. It always reports
// success for valid requests, even when no account exists, so callers cannot
// enumerate registered emails.
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.svc.RequestPasswordReset(r.Context(), req.Email); err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// ResetPassword sets a new password using a single-use token from the reset
// email.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		if errors.Is(err, ErrInvalidResetToken) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "password reset"})
}
