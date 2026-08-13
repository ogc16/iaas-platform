package billing

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ogc16/iaas-platform/internal/auth"
	"github.com/ogc16/iaas-platform/internal/models"
)

func newHandlerRouter() (*chi.Mux, *fakeUsageStore, *fakeInvoiceStore, *fakeMembershipStore) {
	svc, usage, invoices, members := newTestService()
	handler := NewHandler(svc)

	authSvc := auth.NewService(nil, auth.NewJWTService("test-secret", "test-issuer", 3600))

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(authSvc))
		r.Get("/orgs/{orgID}/billing/usage", handler.GetUsage)
		r.Post("/orgs/{orgID}/billing/usage", handler.RecordUsage)
		r.Get("/orgs/{orgID}/billing/invoices", handler.ListInvoices)
		r.Post("/orgs/{orgID}/billing/invoices/generate", handler.GenerateInvoice)
	})
	return r, usage, invoices, members
}

func token(t *testing.T) string {
	t.Helper()
	tok, err := auth.NewJWTService("test-secret", "test-issuer", 3600).GenerateToken(7, "alice@example.com", "admin")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return tok
}

func postJSON(t *testing.T, r http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token(t))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestHandler_RecordUsage_Success(t *testing.T) {
	r, usage, _, _ := newHandlerRouter()

	rec := postJSON(t, r, "/orgs/1/billing/usage", `{"instance_id":5,"resource_type":"cpu_hours","quantity":2.5}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(usage.records) != 1 || usage.records[0].ResourceType != models.ResourceTypeCPUHours {
		t.Fatalf("usage not recorded: %+v", usage.records)
	}
}

func TestHandler_RecordUsage_UnknownResourceType(t *testing.T) {
	r, _, _, _ := newHandlerRouter()

	rec := postJSON(t, r, "/orgs/1/billing/usage", `{"instance_id":5,"resource_type":"banana","quantity":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_RecordUsage_RequiresAuth(t *testing.T) {
	r, _, _, _ := newHandlerRouter()

	req := httptest.NewRequest(http.MethodPost, "/orgs/1/billing/usage", bytes.NewBufferString(`{"instance_id":5,"resource_type":"cpu_hours","quantity":1}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandler_GenerateInvoice_Success(t *testing.T) {
	r, usage, invoices, _ := newHandlerRouter()
	usage.summary = &models.UsageSummary{CPUHours: 10}

	rec := postJSON(t, r, "/orgs/1/billing/invoices/generate", "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(invoices.created) != 1 || invoices.created[0].AmountCents != 500 {
		t.Fatalf("unexpected invoice: %+v", invoices.created)
	}

	var inv models.Invoice
	if err := json.Unmarshal(rec.Body.Bytes(), &inv); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if inv.AmountCents != 500 || inv.Status != models.InvoiceStatusPending {
		t.Fatalf("unexpected invoice body: %+v", inv)
	}
}

func TestHandler_GetUsage_NotInOrg(t *testing.T) {
	r, _, _, members := newHandlerRouter()
	members.ok = false

	req := httptest.NewRequest(http.MethodGet, "/orgs/1/billing/usage", nil)
	req.Header.Set("Authorization", "Bearer "+token(t))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
