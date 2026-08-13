package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestPageParams_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)

	limit, offset := PageParams(r)
	if limit != DefaultPageLimit || offset != 0 {
		t.Fatalf("expected defaults (limit=%d offset=0), got (limit=%d offset=%d)", DefaultPageLimit, limit, offset)
	}
}

func TestPageParams_ParsesAndBounds(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=10&offset=25", nil)

	limit, offset := PageParams(r)
	if limit != 10 || offset != 25 {
		t.Fatalf("expected (10, 25), got (%d, %d)", limit, offset)
	}
}

func TestPageParams_ClampsLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=10000", nil)

	limit, _ := PageParams(r)
	if limit != MaxPageLimit {
		t.Fatalf("expected limit clamped to %d, got %d", MaxPageLimit, limit)
	}
}

func TestPageParams_IgnoresInvalidValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=abc&offset=-5", nil)

	limit, offset := PageParams(r)
	if limit != DefaultPageLimit || offset != 0 {
		t.Fatalf("expected defaults, got (limit=%d offset=%d)", limit, offset)
	}
}

func TestSetTotalCount(t *testing.T) {
	w := httptest.NewRecorder()
	SetTotalCount(w, 137)

	if got := w.Header().Get("X-Total-Count"); got != "137" {
		t.Fatalf("expected X-Total-Count 137, got %q", got)
	}
}
