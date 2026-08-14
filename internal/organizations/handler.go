package organizations

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

	var req models.CreateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	org, err := h.svc.Create(r.Context(), claims.UserID, req)
	if err != nil {
		if errors.Is(err, ErrSlugTaken) {
			httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, org)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	limit, offset := httpx.PageParams(r)
	orgs, total, err := h.svc.List(r.Context(), claims.UserID, limit, offset)
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.SetTotalCount(w, total)
	httpx.WriteJSON(w, http.StatusOK, orgs)
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

	org, err := h.svc.GetByID(r.Context(), orgID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotMember) || errors.Is(err, ErrNotFound) {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, org)
}

func (h *Handler) InviteMember(w http.ResponseWriter, r *http.Request) {
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

	var req models.InviteMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	member, err := h.svc.InviteMember(r.Context(), orgID, claims.UserID, req)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, member)
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
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
	members, total, err := h.svc.ListMembers(r.Context(), orgID, claims.UserID, limit, offset)
	if err != nil {
		httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	httpx.SetTotalCount(w, total)
	httpx.WriteJSON(w, http.StatusOK, members)
}

func (h *Handler) ListJoinRequests(w http.ResponseWriter, r *http.Request) {
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
	requests, total, err := h.svc.ListJoinRequests(r.Context(), orgID, claims.UserID, limit, offset)
	if err != nil {
		if isAdminError(err) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.SetTotalCount(w, total)
	httpx.WriteJSON(w, http.StatusOK, requests)
}

func (h *Handler) AcceptJoinRequest(w http.ResponseWriter, r *http.Request) {
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
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	member, err := h.svc.AcceptJoinRequest(r.Context(), orgID, userID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrRequestNotFound) {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrUserAlreadyMember) {
			httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if isAdminError(err) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, member)
}

func (h *Handler) RevokeJoinRequest(w http.ResponseWriter, r *http.Request) {
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
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	err = h.svc.RevokeJoinRequest(r.Context(), orgID, userID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrRequestNotFound) {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		if isAdminError(err) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
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
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	err = h.svc.RemoveMember(r.Context(), orgID, userID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		if isAdminError(err) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// SuspendMember temporarily revokes a member's access for the given number of
// days. Admins only.
func (h *Handler) SuspendMember(w http.ResponseWriter, r *http.Request) {
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
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	var req models.SuspendMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := validate.Struct(&req); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	member, err := h.svc.SuspendMember(r.Context(), orgID, userID, req.Days, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		if isAdminError(err) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, member)
}

// UnsuspendMember immediately restores a suspended member's access. Admins
// only.
func (h *Handler) UnsuspendMember(w http.ResponseWriter, r *http.Request) {
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
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	member, err := h.svc.UnsuspendMember(r.Context(), orgID, userID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			httpx.WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		if isAdminError(err) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": httpx.ErrInternalServer})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, member)
}

// isAdminError reports whether err is a membership/authorization failure that
// should be surfaced to the caller as a bad request.
func isAdminError(err error) bool {
	return errors.Is(err, ErrNotAdmin) || errors.Is(err, ErrSelfAction)
}
