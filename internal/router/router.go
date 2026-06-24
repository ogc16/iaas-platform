package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/user/iaas-platform/internal/auth"
	"github.com/user/iaas-platform/internal/billing"
	"github.com/user/iaas-platform/internal/compute"
	"github.com/user/iaas-platform/internal/dashboard"
	"github.com/user/iaas-platform/internal/middleware"
	"github.com/user/iaas-platform/internal/organizations"
)

func New(
	authHandler *auth.Handler,
	orgHandler *organizations.Handler,
	computeHandler *compute.Handler,
	billingHandler *billing.Handler,
	authMW func(http.Handler) http.Handler,
) *chi.Mux {
	r := chi.NewRouter()

	rl := middleware.NewRateLimiter(1, 10, time.Second)

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.CORS)
	r.Use(rl.Middleware)

	r.Handle("/static/*", http.StripPrefix("/static/", dashboard.Handler()))
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

					r.Route("/billing", func(r chi.Router) {
						r.Get("/usage", billingHandler.GetUsage)
						r.Get("/invoices", billingHandler.ListInvoices)
						r.Get("/invoices/{invoiceID}", billingHandler.GetInvoice)
					})
				})
			})
		})
	})

	return r
}
