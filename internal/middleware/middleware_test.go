package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func TestSecurityHeaders(t *testing.T) {
	var got http.Header
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = w.Header()
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	SecurityHeaders(false)(next).ServeHTTP(rec, req)

	for _, key := range []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Referrer-Policy",
		"Content-Security-Policy",
	} {
		if got.Get(key) == "" {
			t.Fatalf("expected header %q to be set", key)
		}
	}
	if got.Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS must not be set when hsts=false")
	}
}

func TestSecurityHeaders_HSTSOnlyWhenEnabled(t *testing.T) {
	var got http.Header
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = w.Header()
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	SecurityHeaders(true)(next).ServeHTTP(rec, req)

	if got.Get("Strict-Transport-Security") == "" {
		t.Fatal("expected HSTS header when hsts=true")
	}
}

func TestLogger_EchoesRequestID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chimw.RequestIDKey, "req-123"))
	Logger(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "req-123" {
		t.Fatalf("expected X-Request-ID header %q, got %q", "req-123", got)
	}
}
