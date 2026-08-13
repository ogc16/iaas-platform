package compute

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

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": httpx.ErrInvalidOrgID})
		return
	}

	var req models.CreateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	inst, err := h.svc.Create(r.Context(), orgID, claims.UserID, req)
	if err != nil {
		httpx.WriteError(w, err, map[error]int{
			ErrNotInOrg:             http.StatusForbidden,
			ErrUnknownRegion:        http.StatusBadRequest,
			ErrQuotaExceeded:        http.StatusConflict,
			ErrInsufficientCapacity: http.StatusConflict,
		})
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, inst)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
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

	instances, err := h.svc.List(r.Context(), orgID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotInOrg) {
			httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, instances)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
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

	instanceID, err := strconv.ParseInt(chi.URLParam(r, "instanceID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid instance id"})
		return
	}

	inst, err := h.svc.Get(r.Context(), orgID, instanceID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "instance not found"})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, inst)
}

// Start, Stop and Terminate accept the transition and return 202 Accepted:
// the target state is reached asynchronously by the reconciler.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "start")
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "stop")
}

func (h *Handler) Terminate(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "terminate")
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request, action string) {
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
	instanceID, err := strconv.ParseInt(chi.URLParam(r, "instanceID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid instance id"})
		return
	}

	var actionErr error
	switch action {
	case "start":
		actionErr = h.svc.Start(r.Context(), orgID, instanceID, claims.UserID)
	case "stop":
		actionErr = h.svc.Stop(r.Context(), orgID, instanceID, claims.UserID)
	case "terminate":
		actionErr = h.svc.Terminate(r.Context(), orgID, instanceID, claims.UserID)
	}
	if actionErr != nil {
		httpx.WriteError(w, actionErr, map[error]int{
			ErrNotInOrg:          http.StatusForbidden,
			ErrNotFound:          http.StatusNotFound,
			ErrInvalidTransition: http.StatusConflict,
		})
		return
	}

	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "action": action})
}
