package middleware

import "net/http"

// SecurityHeaders sets baseline HTTP security headers on every response.
//
// The CSP allows the embedded single-page dashboard (its own scripts and
// styles, inline handlers/styles it uses, and same-origin API calls) while
// blocking everything else, including framing and plugin content.
//
// HSTS is only emitted when hsts is true, i.e. the service is served over
// TLS. Advertising HSTS over plain HTTP is a no-op at best and can lock
// clients out of a misconfigured deployment at worst.
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}

			next.ServeHTTP(w, r)
		})
	}
}
