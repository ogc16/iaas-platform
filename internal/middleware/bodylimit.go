package middleware

import (
	"net/http"
	"strconv"
)

// BodyLimit returns a middleware that caps the request body size using
// http.MaxBytesReader. When the client sends a body larger than maxBytes,
// the reader returns an error on the first Read call, which the JSON
// decoder in httpx will surface as a 413.
//
// A zero or negative maxBytes disables the limit (passthrough).
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if maxBytes <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// ParseBodyBytes parses a human-readable byte size string (e.g. "1048576",
// "1M", "2m") into bytes. It is used to load the limit from env config.
// Plain integers are returned as-is. Suffix "m" or "M" multiplies by 1 MiB.
func ParseBodyBytes(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	suffix := s[len(s)-1]
	if suffix == 'm' || suffix == 'M' {
		n, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
		if err != nil {
			return 0, err
		}
		return n * 1024 * 1024, nil
	}
	return strconv.ParseInt(s, 10, 64)
}
