package organizations

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ogc16/iaas-platform/internal/auth"
	"github.com/ogc16/iaas-platform/internal/models"
)

const testJWTSecret = "test-secret"

func newHandlerRouter() (*chi.Mux, *fakeOrgStore, *fakeUserStore) {
	svc, orgs, users := newTestService()
	handler := NewHandler(svc)

	authSvc := auth.NewService(nil, auth.NewJWTService(testJWTSecret, "test-issuer", 3600))

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(authSvc))
		r.Post("/orgs", handler.Create)
		r.Get("/orgs", handler.List)
		r.Route("/orgs/{orgID}", func(r chi.Router) {
			r.Get("/", handler.Get)
			r.Post("/members", handler.InviteMember)
			r.Get("/members", handler.ListMembers)
			r.Delete("/members/{userID}", handler.RemoveMember)
			r.Post("/members/{userID}/suspend", handler.SuspendMember)
			r.Post("/members/{userID}/unsuspend", handler.UnsuspendMember)
			r.Get("/requests", handler.ListJoinRequests)
			r.Post("/requests/{userID}/accept", handler.AcceptJoinRequest)
			r.Post("/requests/{userID}/revoke", handler.RevokeJoinRequest)
		})
	})
	return r, orgs, users
}

func bearerToken(t *testing.T, userID int64) string {
	t.Helper()
	token, err := auth.NewJWTService(testJWTSecret, "test-issuer", 3600).GenerateToken(userID, "alice@example.com", "user")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return token
}

func doJSON(t *testing.T, r http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestHandler_List_ReturnsJSONArray(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}

	rec := doJSON(t, r, http.MethodGet, "/orgs", bearerToken(t, 7), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// The dashboard expects a JSON array.
	var raw []json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("expected a JSON array, got %s", rec.Body.String())
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 org, got %d", len(raw))
	}
}

func TestHandler_List_RequiresAuth(t *testing.T) {
	r, _, _ := newHandlerRouter()

	rec := doJSON(t, r, http.MethodGet, "/orgs", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandler_Create_Success(t *testing.T) {
	r, _, _ := newHandlerRouter()

	rec := doJSON(t, r, http.MethodPost, "/orgs", bearerToken(t, 7), `{"name":"Acme","slug":"acme"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var org models.Organization
	if err := json.Unmarshal(rec.Body.Bytes(), &org); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if org.Name != "Acme" || org.Slug != "acme" {
		t.Fatalf("unexpected org: %+v", org)
	}
}

func TestHandler_Create_RequiresName(t *testing.T) {
	r, _, _ := newHandlerRouter()

	rec := doJSON(t, r, http.MethodPost, "/orgs", bearerToken(t, 7), `{"slug":"acme"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_Get_NonMemberReturnsNotFound(t *testing.T) {
	r, _, _ := newHandlerRouter()

	rec := doJSON(t, r, http.MethodGet, "/orgs/999", bearerToken(t, 7), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandler_InviteMember_NonAdminRejected(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "member"}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/members", bearerToken(t, 7), `{"email":"bob@example.com"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "only admins can invite members") {
		t.Fatalf("unexpected error body: %s", rec.Body.String())
	}
}

func TestHandler_InviteMember_Success(t *testing.T) {
	r, orgs, users := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}
	users.byEmail["bob@example.com"] = &models.User{ID: 2, Email: "bob@example.com"}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/members", bearerToken(t, 7), `{"email":"bob@example.com"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_AcceptJoinRequest_NonAdminRejected(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "member"}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/requests/2/accept", bearerToken(t, 7), "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "only admins can manage members") {
		t.Fatalf("unexpected error body: %s", rec.Body.String())
	}
}

func TestHandler_AcceptJoinRequest_Success(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}
	orgs.joinRequests["1:2"] = &models.JoinRequest{ID: 5, OrganizationID: 1, UserID: 2}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/requests/2/accept", bearerToken(t, 7), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := orgs.members["1:2"]; !ok {
		t.Fatal("expected user 2 to become a member")
	}
	if _, ok := orgs.joinRequests["1:2"]; ok {
		t.Fatal("expected join request to be removed")
	}
}

func TestHandler_RevokeJoinRequest_Success(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}
	orgs.joinRequests["1:2"] = &models.JoinRequest{ID: 5, OrganizationID: 1, UserID: 2}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/requests/2/revoke", bearerToken(t, 7), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := orgs.joinRequests["1:2"]; ok {
		t.Fatal("expected join request to be removed")
	}
}

func TestHandler_RemoveMember_Success(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}
	orgs.members["1:2"] = &models.OrgMember{ID: 2, OrganizationID: 1, UserID: 2, Role: "member"}

	rec := doJSON(t, r, http.MethodDelete, "/orgs/1/members/2", bearerToken(t, 7), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := orgs.members["1:2"]; ok {
		t.Fatal("expected member 2 to be removed")
	}
}

func TestHandler_ListJoinRequests_Success(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}
	orgs.joinRequests["1:2"] = &models.JoinRequest{ID: 5, OrganizationID: 1, UserID: 2, Email: "bob@example.com"}

	rec := doJSON(t, r, http.MethodGet, "/orgs/1/requests", bearerToken(t, 7), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("expected a JSON array, got %s", rec.Body.String())
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 join request, got %d", len(raw))
	}
}

func TestHandler_SuspendMember_Success(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}
	orgs.members["1:2"] = &models.OrgMember{ID: 2, OrganizationID: 1, UserID: 2, Role: "member"}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/members/2/suspend", bearerToken(t, 7), `{"days":30}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var member models.OrgMember
	if err := json.Unmarshal(rec.Body.Bytes(), &member); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if member.SuspendedUntil == nil {
		t.Fatal("expected suspended_until to be set in response")
	}
	if _, err := orgs.FindMember(t.Context(), 1, 2); err == nil {
		t.Fatal("expected suspended member to be treated as not a member")
	}
}

func TestHandler_SuspendMember_InvalidDays(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}
	orgs.members["1:2"] = &models.OrgMember{ID: 2, OrganizationID: 1, UserID: 2, Role: "member"}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/members/2/suspend", bearerToken(t, 7), `{"days":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for days=0, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_SuspendMember_NonAdminRejected(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "member"}
	orgs.members["1:2"] = &models.OrgMember{ID: 2, OrganizationID: 1, UserID: 2, Role: "member"}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/members/2/suspend", bearerToken(t, 7), `{"days":30}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "only admins can manage members") {
		t.Fatalf("unexpected error body: %s", rec.Body.String())
	}
}

func TestHandler_SuspendMember_SelfRejected(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/members/7/suspend", bearerToken(t, 7), `{"days":30}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "cannot modify your own membership") {
		t.Fatalf("unexpected error body: %s", rec.Body.String())
	}
}

func TestHandler_UnsuspendMember_Success(t *testing.T) {
	r, orgs, _ := newHandlerRouter()

	org := &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	orgs.orgs[1] = org
	orgs.bySlug["acme"] = org
	orgs.members["1:7"] = &models.OrgMember{ID: 1, OrganizationID: 1, UserID: 7, Role: "admin"}
	until := time.Now().UTC().Add(30 * 24 * time.Hour)
	orgs.members["1:2"] = &models.OrgMember{ID: 2, OrganizationID: 1, UserID: 2, Role: "member", SuspendedUntil: &until}

	rec := doJSON(t, r, http.MethodPost, "/orgs/1/members/2/unsuspend", bearerToken(t, 7), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var member models.OrgMember
	if err := json.Unmarshal(rec.Body.Bytes(), &member); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if member.SuspendedUntil != nil {
		t.Fatalf("expected suspended_until to be cleared, got %v", *member.SuspendedUntil)
	}
	if _, err := orgs.FindMember(t.Context(), 1, 2); err != nil {
		t.Fatalf("expected member to regain access after unsuspend: %v", err)
	}
}
