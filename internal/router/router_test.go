package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ogc16/iaas-platform/internal/auth"
	"github.com/ogc16/iaas-platform/internal/billing"
	"github.com/ogc16/iaas-platform/internal/compute"
	"github.com/ogc16/iaas-platform/internal/organizations"
)

func newTestRouter() http.Handler {
	jwt := auth.NewJWTService("test-secret", "test-issuer", 3600)
	authMW := auth.Middleware(auth.NewService(nil, jwt))

	return New(
		auth.NewHandler(auth.NewService(nil, jwt)),
		organizations.NewHandler(nil),
		compute.NewHandler(nil),
		billing.NewHandler(nil),
		authMW,
	)
}

func do(t *testing.T, r http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestRouter_ServesDashboard(t *testing.T) {
	r := newTestRouter()

	rec := do(t, r, http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("unexpected content type: %q", ct)
	}
}

func TestRouter_ServesStaticAssets(t *testing.T) {
	r := newTestRouter()

	rec := do(t, r, http.MethodGet, "/static/app.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for static asset, got %d", rec.Code)
	}
}

func TestRouter_ServesDocsAndOpenAPI(t *testing.T) {
	r := newTestRouter()

	rec := do(t, r, http.MethodGet, "/docs")
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect from /docs, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/static/docs.html" {
		t.Fatalf("expected redirect to /static/docs.html, got %q", loc)
	}

	rec = do(t, r, http.MethodGet, "/static/openapi.yaml")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for openapi.yaml, got %d", rec.Code)
	}
}

func TestRouter_PublicAuthRoutes(t *testing.T) {
	r := newTestRouter()

	// Empty body signup fails validation before touching the service.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader(""))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty signup body, got %d", rec.Code)
	}
}

func TestRouter_ProtectedRoutesRequireAuth(t *testing.T) {
	r := newTestRouter()

	for _, path := range []string{
		"/api/v1/me",
		"/api/v1/orgs",
		"/api/v1/orgs/1",
		"/api/v1/orgs/1/members",
		"/api/v1/orgs/1/instances",
		"/api/v1/orgs/1/instances/2",
		"/api/v1/orgs/1/instances/2/start",
		"/api/v1/orgs/1/billing/usage",
		"/api/v1/orgs/1/billing/invoices",
		"/api/v1/orgs/1/billing/invoices/1",
	} {
		rec := do(t, r, http.MethodGet, path)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s: expected 401 without token, got %d", path, rec.Code)
		}
	}
}

func TestRouter_UnknownRouteNotFound(t *testing.T) {
	r := newTestRouter()

	rec := do(t, r, http.MethodGet, "/api/v1/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRouter_BillingGenerateRouteExists(t *testing.T) {
	r := newTestRouter()

	// Without auth it must 401, proving the route is wired.
	rec := do(t, r, http.MethodPost, "/api/v1/orgs/1/billing/invoices/generate")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (route exists), got %d", rec.Code)
	}
}
