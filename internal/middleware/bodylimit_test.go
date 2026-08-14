package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimit_RejectsOversizedBody(t *testing.T) {
	// 10-byte limit; send 11 bytes.
	handler := BodyLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reading the body should fail.
		buf := make([]byte, 100)
		n, err := r.Body.Read(buf)
		if err == nil {
			t.Fatalf("expected error reading oversized body, got %d bytes", n)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader("1234567890x"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// The handler is called (MaxBytesReader doesn't prevent the handler from
	// running), but the body read fails. The status is 200 because the handler
	// wrote it before attempting to read.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from handler, got %d", rec.Code)
	}
}

func TestBodyLimit_AllowsNormalBody(t *testing.T) {
	handler := BodyLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 100)
		n, err := r.Body.Read(buf)
		if err != nil {
			t.Fatalf("unexpected error reading body: %v", err)
		}
		if n != 5 {
			t.Fatalf("expected 5 bytes, got %d", n)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader("hello"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestBodyLimit_PassthroughWhenZero(t *testing.T) {
	handler := BodyLimit(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 100)
		n, err := r.Body.Read(buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 5 {
			t.Fatalf("expected 5 bytes, got %d", n)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hello"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestParseBodyBytes(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		err   bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"1048576", 1048576, false},
		{"1M", 1048576, false},
		{"2m", 2097152, false},
		{"5M", 5242880, false},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseBodyBytes(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("ParseBodyBytes(%q) error = %v, wantErr %v", tt.input, err, tt.err)
		}
		if got != tt.want {
			t.Errorf("ParseBodyBytes(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
