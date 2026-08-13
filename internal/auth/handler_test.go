package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ogc16/iaas-platform/internal/models"
)

func postJSON(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func newHandlerRouter() http.Handler {
	svc, _, _ := newTestService()
	handler := NewHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("/signup", handler.Signup)
	mux.HandleFunc("/login", handler.Login)
	return mux
}

func TestHandler_Signup_ShortPasswordRejected(t *testing.T) {
	rec := postJSON(t, newHandlerRouter(), "/signup", `{"email":"dev@example.com","password":"short","name":"Dev"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "at least 8") {
		t.Fatalf("expected password length error, got %s", rec.Body.String())
	}
}

func TestHandler_Signup_InvalidEmailRejected(t *testing.T) {
	rec := postJSON(t, newHandlerRouter(), "/signup", `{"email":"not-an-email","password":"hunter2hunter2","name":"Dev"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "valid email") {
		t.Fatalf("expected email error, got %s", rec.Body.String())
	}
}

func TestHandler_Signup_ValidRequestSucceeds(t *testing.T) {
	rec := postJSON(t, newHandlerRouter(), "/signup", `{"email":"dev@example.com","password":"hunter2hunter2","name":"Dev"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp models.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected a token in the response")
	}
}

func TestHandler_Login_RejectsBadEmail(t *testing.T) {
	rec := postJSON(t, newHandlerRouter(), "/login", `{"email":"not-an-email","password":"whatever"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
