package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// Middleware records request count and latency per method and matched route.
// The route pattern (e.g. /api/v1/orgs/{orgID}) comes from chi's routing
// context, so unrelated IDs do not explode the label cardinality.
func Middleware(reg *Registry) func(http.Handler) http.Handler {
	if reg == nil {
		reg = NewRegistry()
	}
	counter := reg.NewCounter("iaas_http_requests_total", "Total HTTP requests handled", []string{"method", "route", "status"})
	latency := reg.NewHistogram("iaas_http_request_duration_seconds", "HTTP request latency in seconds", []string{"method", "route"}, nil)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			route := r.URL.Path
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				route = rc.RoutePattern()
			}
			counter.WithLabelValues(r.Method, route, strconv.Itoa(sw.status)).Add(1)
			latency.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		})
	}
}

// statusWriter captures the response status code written by the wrapped
// handler. It defaults to 200 and is updated by WriteHeader.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
