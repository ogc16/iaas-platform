// Package audit records an immutable, per-organization trail of
// security-relevant actions and serves it to admins. Recording happens in
// services via the optional Recorder interface; request metadata (client IP,
// request ID) is stashed in the context by Middleware.
package audit

import (
	"context"
	"net"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/ogc16/iaas-platform/internal/models"
	"github.com/ogc16/iaas-platform/internal/reqctx"
)

// Recorder persists audit events.
type Recorder interface {
	Record(ctx context.Context, e *models.AuditEvent) error
}

type contextKey string

const (
	ipKey        contextKey = "audit.ip"
	requestIDKey contextKey = "audit.request_id"
)

// Middleware stashes the client IP and request ID into the context so
// audit.Record can attach them without plumbing request metadata through
// services. It must run after the request-ID and real-IP middlewares.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ip := ""
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			ip = host
		} else if r.RemoteAddr != "" {
			ip = r.RemoteAddr
		}
		ctx = context.WithValue(ctx, ipKey, ip)

		if rid := chimw.GetReqID(ctx); rid != "" {
			ctx = context.WithValue(ctx, requestIDKey, rid)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Record enriches an audit event with request metadata from the context and
// persists it. It is a no-op when rec is nil so optional instrumentation never
// breaks request handling, and it never fails the caller on a write error
// (the trail must not take the control plane down).
func Record(ctx context.Context, rec Recorder, e *models.AuditEvent) {
	if rec == nil {
		return
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if ip, ok := ctx.Value(ipKey).(string); ok {
		e.IP = ip
	}
	if rid, ok := ctx.Value(requestIDKey).(string); ok {
		e.RequestID = rid
	}
	if email := reqctx.ActorEmail(ctx); email != "" {
		e.ActorEmail = email
	}
	_ = rec.Record(ctx, e)
}
