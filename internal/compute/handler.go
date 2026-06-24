package compute

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/user/iaas-platform/internal/auth"
	"github.com/user/iaas-platform/internal/models"
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
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	var req models.CreateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	inst, err := h.svc.Create(r.Context(), orgID, claims.UserID, req)
	if err != nil {
		if errors.Is(err, ErrNotInOrg) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, inst)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
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

	instances, err := h.svc.List(r.Context(), orgID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotInOrg) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, instances)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
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

	instanceID, err := strconv.ParseInt(chi.URLParam(r, "instanceID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid instance id"})
		return
	}

	inst, err := h.svc.Get(r.Context(), orgID, instanceID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "instance not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, inst)
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, _ := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	instanceID, _ := strconv.ParseInt(chi.URLParam(r, "instanceID"), 10, 64)

	if err := h.svc.Start(r.Context(), orgID, instanceID, claims.UserID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, _ := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	instanceID, _ := strconv.ParseInt(chi.URLParam(r, "instanceID"), 10, 64)

	if err := h.svc.Stop(r.Context(), orgID, instanceID, claims.UserID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *Handler) Terminate(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, _ := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	instanceID, _ := strconv.ParseInt(chi.URLParam(r, "instanceID"), 10, 64)

	if err := h.svc.Terminate(r.Context(), orgID, instanceID, claims.UserID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "terminated"})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
