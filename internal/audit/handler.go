package audit

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ogc16/iaas-platform/internal/httpx"
	"github.com/ogc16/iaas-platform/internal/reqctx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// List returns a page of audit events filtered by the optional query
// parameters action, user_id, since (RFC3339), limit and offset.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := reqctx.UserID(r.Context())
	if !ok {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": httpx.ErrInvalidOrgID})
		return
	}

	f := Filter{Action: r.URL.Query().Get("action"), Resource: r.URL.Query().Get("resource")}
	f.Limit, f.Offset = httpx.PageParams(r)

	if raw := r.URL.Query().Get("user_id"); raw != "" {
		uid, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
			return
		}
		f.UserID = &uid
	}
	if raw := r.URL.Query().Get("since"); raw != "" {
		since, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid since (want RFC3339)"})
			return
		}
		f.Since = &since
	}

	events, total, err := h.svc.List(r.Context(), orgID, userID, f)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.SetTotalCount(w, total)
	httpx.WriteJSON(w, http.StatusOK, events)
}
