package metrics

import (
	"net/http"

	"github.com/ogc16/iaas-platform/internal/httpx"
)

// Handler serves the Prometheus text exposition format at /metrics. When a
// token is configured, scrapes must present it as `Authorization: Bearer
// <token>`; with an empty token the endpoint is public (the default).
func Handler(reg *Registry, token string) http.Handler {
	if reg == nil {
		reg = NewRegistry()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or missing metrics token"})
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = reg.Write(w)
	})
}
