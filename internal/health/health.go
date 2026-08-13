// Package health exposes liveness and readiness probes for load balancers and
// container orchestration platforms.
//
//	/healthz — liveness: the process is up. Always 200.
//	/readyz  — readiness: the process can serve traffic, i.e. the database is
//	           reachable. 503 while it is not.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/ogc16/iaas-platform/internal/httpx"
)

// Pinger reports whether a downstream dependency is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Liveness returns a handler that reports the process is alive. It never
// depends on external state; a running process is by definition "live".
func Liveness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// Readiness returns a handler that reports whether the process can serve
// traffic by pinging the given dependency within the timeout.
func Readiness(pinger Pinger, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := pinger.Ping(ctx); err != nil {
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"reason": "database unreachable",
			})
			return
		}

		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
