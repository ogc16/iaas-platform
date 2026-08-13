package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIndexHTML_NotEmpty(t *testing.T) {
	b, err := IndexHTML()
	if err != nil {
		t.Fatalf("IndexHTML returned error: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("IndexHTML returned empty html")
	}
}

func TestIndexHTML_IsRenderedPage(t *testing.T) {
	b, err := IndexHTML()
	if err != nil {
		t.Fatalf("IndexHTML returned error: %v", err)
	}

	page := string(b)
	for _, want := range []string{"<!DOCTYPE html>", `<div id="app">`, "IaaS Platform"} {
		if !strings.Contains(page, want) {
			t.Fatalf("index.html missing expected marker %q", want)
		}
	}
}

func TestHandler_ServesStaticAssets(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for embedded asset, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "fetch") {
		t.Fatalf("expected JS asset content to be served")
	}
}
