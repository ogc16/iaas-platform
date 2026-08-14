package auth

import (
	"bytes"
	"context"
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
	mux.HandleFunc("/forgot-password", handler.ForgotPassword)
	mux.HandleFunc("/reset-password", handler.ResetPassword)
	return mux
}

func newResetHandlerRouter() http.Handler {
	store := newFakeUserStore()
	resets := newFakeResetStore()
	mail := &fakeMailer{}
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	svc := NewService(store, jwt, WithResetStore(resets), WithMailer(mail))
	handler := NewHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("/forgot-password", handler.ForgotPassword)
	mux.HandleFunc("/reset-password", handler.ResetPassword)
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

func TestHandler_ForgotPassword_ValidRequestSucceeds(t *testing.T) {
	rec := postJSON(t, newResetHandlerRouter(), "/forgot-password", `{"email":"alice@example.com"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if resp["status"] != "sent" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestHandler_ForgotPassword_RejectsBadEmail(t *testing.T) {
	rec := postJSON(t, newResetHandlerRouter(), "/forgot-password", `{"email":"not-an-email"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ResetPassword_ValidRequestSucceeds(t *testing.T) {
	store := newFakeUserStore()
	resets := newFakeResetStore()
	mail := &fakeMailer{}
	user := seedUser(t, store, "alice@example.com", "hunter2")

	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	svc := NewService(store, jwt, WithResetStore(resets), WithMailer(mail))
	if err := svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	token := resetTokenFromEmail(mail)

	handler := NewHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("/reset-password", handler.ResetPassword)
	rec := postJSON(t, mux, "/reset-password",
		`{"token":"`+token+`","new_password":"brand-new-password"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ResetPassword_RejectsBadToken(t *testing.T) {
	rec := postJSON(t, newResetHandlerRouter(), "/reset-password",
		`{"token":"short","new_password":"brand-new-password"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ResetPassword_RejectsWeakPassword(t *testing.T) {
	rec := postJSON(t, newResetHandlerRouter(), "/reset-password",
		`{"token":"pwr_`+strings.Repeat("a", 60)+`","new_password":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for weak password, got %d: %s", rec.Code, rec.Body.String())
	}
}
