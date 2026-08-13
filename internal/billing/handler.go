package billing

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

func (h *Handler) RecordUsage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": httpx.ErrInvalidOrgID})
		return
	}

	var req models.RecordUsageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.svc.RecordUsage(r.Context(), orgID, req.InstanceID, req.ResourceType, req.Quantity); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}

func (h *Handler) GenerateInvoice(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": httpx.ErrInvalidOrgID})
		return
	}

	inv, err := h.svc.GenerateInvoice(r.Context(), orgID)
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, inv)
}

func (h *Handler) GetUsage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": httpx.ErrInvalidOrgID})
		return
	}

	usage, err := h.svc.GetUsage(r.Context(), orgID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotInOrg) {
			httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, usage)
}

func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": httpx.ErrInvalidOrgID})
		return
	}

	limit, offset := httpx.PageParams(r)
	invoices, total, err := h.svc.GetInvoices(r.Context(), orgID, claims.UserID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrNotInOrg) {
			httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.SetTotalCount(w, total)
	httpx.WriteJSON(w, http.StatusOK, invoices)
}

func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	invoiceID, err := strconv.ParseInt(chi.URLParam(r, "invoiceID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid invoice id"})
		return
	}

	items, err := h.svc.GetInvoiceLineItems(r.Context(), invoiceID)
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, items)
}
