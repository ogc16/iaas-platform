package httpx

import (
	"net/http"
	"strconv"
)

const (
	// DefaultPageLimit is the number of items returned when limit is not set.
	DefaultPageLimit = 50
	// MaxPageLimit caps limit so a single page can never be unbounded.
	MaxPageLimit = 100
)

// PageParams parses and bounds the limit/offset query parameters used by list
// endpoints. Non-numeric or negative values fall back to the defaults, and
// limit is capped at MaxPageLimit.
func PageParams(r *http.Request) (limit, offset int) {
	limit = DefaultPageLimit
	offset = 0

	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}

	if raw := r.URL.Query().Get("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			offset = n
		}
	}

	return limit, offset
}

// SetTotalCount records the total number of matching items on the response via
// the X-Total-Count header, so clients can compute page counts without
// changing the array response shape.
func SetTotalCount(w http.ResponseWriter, total int64) {
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
}
