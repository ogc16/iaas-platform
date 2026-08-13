package organizations

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
