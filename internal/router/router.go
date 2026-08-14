package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/ogc16/iaas-platform/internal/audit"
	"github.com/ogc16/iaas-platform/internal/auth"
	"github.com/ogc16/iaas-platform/internal/billing"
	"github.com/ogc16/iaas-platform/internal/compute"
	"github.com/ogc16/iaas-platform/internal/dashboard"
	"github.com/ogc16/iaas-platform/internal/metrics"
	"github.com/ogc16/iaas-platform/internal/middleware"
	"github.com/ogc16/iaas-platform/internal/organizations"
	"github.com/ogc16/iaas-platform/internal/webhooks"
)

func New(
	authHandler *auth.Handler,
	orgHandler *organizations.Handler,
	computeHandler *compute.Handler,
	billingHandler *billing.Handler,
	webhooksHandler *webhooks.Handler,
	auditHandler *audit.Handler,
	registry *metrics.Registry,
	metricsToken string,
	maxBodyBytes int64,
	authMW func(http.Handler) http.Handler,
) *chi.Mux {
	r := chi.NewRouter()

	rl := middleware.NewRateLimiter(1, 10, time.Second)

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(audit.Middleware)
	r.Use(middleware.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.CORS)
	r.Use(middleware.BodyLimit(maxBodyBytes))
	r.Use(rl.Middleware)
	r.Use(metrics.Middleware(registry))

	r.Handle("/static/*", http.StripPrefix("/static/", dashboard.Handler()))
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/docs.html", http.StatusFound)
	})
	r.Handle("/metrics", metrics.Handler(registry, metricsToken))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		html, err := dashboard.IndexHTML()
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(html)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/signup", authHandler.Signup)
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/forgot-password", authHandler.ForgotPassword)
		r.Post("/auth/reset-password", authHandler.ResetPassword)

		r.Group(func(r chi.Router) {
			r.Use(authMW)

			r.Get("/me", authHandler.Me)

			r.Route("/orgs", func(r chi.Router) {
				r.Post("/", orgHandler.Create)
				r.Get("/", orgHandler.List)
				r.Route("/{orgID}", func(r chi.Router) {
					r.Get("/", orgHandler.Get)
					r.Post("/members", orgHandler.InviteMember)
					r.Get("/members", orgHandler.ListMembers)
					r.Delete("/members/{userID}", orgHandler.RemoveMember)
					r.Post("/members/{userID}/suspend", orgHandler.SuspendMember)
					r.Post("/members/{userID}/unsuspend", orgHandler.UnsuspendMember)
					r.Get("/requests", orgHandler.ListJoinRequests)
					r.Post("/requests/{userID}/accept", orgHandler.AcceptJoinRequest)
					r.Post("/requests/{userID}/revoke", orgHandler.RevokeJoinRequest)

					r.Route("/instances", func(r chi.Router) {
						r.Post("/", computeHandler.Create)
						r.Get("/", computeHandler.List)
						r.Route("/{instanceID}", func(r chi.Router) {
							r.Get("/", computeHandler.Get)
							r.Post("/start", computeHandler.Start)
							r.Post("/stop", computeHandler.Stop)
							r.Post("/terminate", computeHandler.Terminate)
						})
					})

					r.Route("/webhooks", func(r chi.Router) {
						r.Post("/", webhooksHandler.Create)
						r.Get("/", webhooksHandler.List)
						r.Route("/{webhookID}", func(r chi.Router) {
							r.Get("/", webhooksHandler.Get)
							r.Put("/", webhooksHandler.Update)
							r.Delete("/", webhooksHandler.Delete)
							r.Post("/ping", webhooksHandler.Ping)
						})
					})

					r.Get("/audit", auditHandler.List)

					r.Route("/billing", func(r chi.Router) {
						r.Get("/usage", billingHandler.GetUsage)
						r.Post("/usage", billingHandler.RecordUsage)
						r.Get("/invoices", billingHandler.ListInvoices)
						r.Post("/invoices/generate", billingHandler.GenerateInvoice)
						r.Get("/invoices/{invoiceID}", billingHandler.GetInvoice)
					})
				})
			})
		})
	})

	return r
}
