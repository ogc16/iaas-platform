package billing

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/user/iaas-platform/internal/auth"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetUsage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	usage, err := h.svc.GetUsage(r.Context(), orgID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotInOrg) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, usage)
}

func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	invoices, err := h.svc.GetInvoices(r.Context(), orgID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotInOrg) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, invoices)
}

func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	invoiceID, err := strconv.ParseInt(chi.URLParam(r, "invoiceID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid invoice id"})
		return
	}

	items, err := h.svc.GetInvoiceLineItems(r.Context(), invoiceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
