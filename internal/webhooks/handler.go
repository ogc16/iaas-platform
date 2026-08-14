package webhooks

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ogc16/iaas-platform/internal/auth"
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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	orgID, ok := orgIDParam(w, r)
	if !ok {
		return
	}

	var req models.CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	webhook, err := h.svc.Create(r.Context(), orgID, claims.UserID, req)
	if err != nil {
		httpx.WriteError(w, err, map[error]int{
			ErrNotAdmin:     http.StatusForbidden,
			ErrInvalidURL:   http.StatusBadRequest,
			ErrInvalidEvent: http.StatusBadRequest,
			ErrEmptyEvents:  http.StatusBadRequest,
		})
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, webhook)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	orgID, ok := orgIDParam(w, r)
	if !ok {
		return
	}

	limit, offset := httpx.PageParams(r)
	webhooks, total, err := h.svc.List(r.Context(), orgID, claims.UserID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrNotAdmin) {
			httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}
	httpx.SetTotalCount(w, total)
	httpx.WriteJSON(w, http.StatusOK, webhooks)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	orgID, ok := orgIDParam(w, r)
	if !ok {
		return
	}
	webhookID, ok := webhookIDParam(w, r)
	if !ok {
		return
	}

	webhook, err := h.svc.Get(r.Context(), orgID, claims.UserID, webhookID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, webhook)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	orgID, ok := orgIDParam(w, r)
	if !ok {
		return
	}
	webhookID, ok := webhookIDParam(w, r)
	if !ok {
		return
	}

	var req models.UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	webhook, err := h.svc.Update(r.Context(), orgID, claims.UserID, webhookID, req)
	if err != nil {
		httpx.WriteError(w, err, map[error]int{
			ErrNotAdmin:     http.StatusForbidden,
			ErrNotFound:     http.StatusNotFound,
			ErrInvalidURL:   http.StatusBadRequest,
			ErrInvalidEvent: http.StatusBadRequest,
			ErrEmptyEvents:  http.StatusBadRequest,
		})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, webhook)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	orgID, ok := orgIDParam(w, r)
	if !ok {
		return
	}
	webhookID, ok := webhookIDParam(w, r)
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), orgID, claims.UserID, webhookID); err != nil {
		httpx.WriteError(w, err, map[error]int{
			ErrNotAdmin: http.StatusForbidden,
			ErrNotFound: http.StatusNotFound,
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	orgID, ok := orgIDParam(w, r)
	if !ok {
		return
	}
	webhookID, ok := webhookIDParam(w, r)
	if !ok {
		return
	}

	if err := h.svc.Ping(r.Context(), orgID, claims.UserID, webhookID); err != nil {
		httpx.WriteError(w, err, map[error]int{
			ErrNotAdmin: http.StatusForbidden,
			ErrNotFound: http.StatusNotFound,
		})
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "action": "ping"})
}

func orgIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": httpx.ErrInvalidOrgID})
		return 0, false
	}
	return id, true
}

func webhookIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "webhookID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook id"})
		return 0, false
	}
	return id, true
}
