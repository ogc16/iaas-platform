package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		apiKey     string
		want       string
	}{
		{"bearer token", "Bearer abc.def.ghi", "", "abc.def.ghi"},
		{"bearer lowercase prefix", "bearer xyz", "", ""},
		{"api key header", "", "iaas_1234", "iaas_1234"},
		{"empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Authorization", tt.authHeader)
			r.Header.Set("X-API-Key", tt.apiKey)
			if got := extractToken(r); got != tt.want {
				t.Fatalf("extractToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractToken_PrefersBearerOverAPIKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer abc.def.ghi")
	r.Header.Set("X-API-Key", "iaas_1234")

	if got := extractToken(r); got != "abc.def.ghi" {
		t.Fatalf("expected bearer token to win, got %q", got)
	}
}

func TestExtractToken_FallsBackToAPIKeyHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "iaas_1234")

	if got := extractToken(r); got != "iaas_1234" {
		t.Fatalf("expected api key from header, got %q", got)
	}
}

func TestMiddleware_MissingToken(t *testing.T) {
	svc := NewService(nil, NewJWTService("secret", "issuer", 3600))
	handler := Middleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	svc := NewService(nil, NewJWTService("secret", "issuer", 3600))
	handler := Middleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_ValidJWT(t *testing.T) {
	jwt := NewJWTService("secret", "issuer", 3600)
	svc := NewService(nil, jwt)

	var gotClaims *Claims
	handler := Middleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims = GetClaims(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	token, err := jwt.GenerateToken(5, "carol@example.com", "admin")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotClaims == nil || gotClaims.UserID != 5 || gotClaims.Email != "carol@example.com" {
		t.Fatalf("expected claims in context, got %+v", gotClaims)
	}
}

func TestGetClaims_NilWithoutMiddleware(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if claims := GetClaims(r.Context()); claims != nil {
		t.Fatalf("expected nil claims, got %+v", claims)
	}
}
