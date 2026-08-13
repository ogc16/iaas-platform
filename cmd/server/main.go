package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ogc16/iaas-platform/internal/auth"
	"github.com/ogc16/iaas-platform/internal/billing"
	"github.com/ogc16/iaas-platform/internal/compute"
	"github.com/ogc16/iaas-platform/internal/config"
	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/health"
	"github.com/ogc16/iaas-platform/internal/middleware"
	"github.com/ogc16/iaas-platform/internal/organizations"
	"github.com/ogc16/iaas-platform/internal/router"
)

func main() {
	// Initialize structured JSON logger for production readability.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Connect with the configured pool bounds. The context is scoped strictly
	// to the connection handshake so it never leaks past startup.
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := database.Connect(dbCtx, cfg.DatabaseURL, cfg.DBMaxConns, cfg.DBMinConns)
	dbCancel()
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	// Apply pending embedded migrations at startup. Each migration runs in its
	// own transaction and applied versions are tracked in schema_migrations.
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, err = database.Migrate(migrateCtx, pool)
	migrateCancel()
	if err != nil {
		slog.Error("failed to apply database migrations", "error", err)
		os.Exit(1)
	}

	// Dependency Injection Engine
	userRepo := database.NewUserRepository(pool)
	orgRepo := database.NewOrgRepository(pool)
	computeRepo := database.NewComputeRepository(pool)
	usageRepo := database.NewUsageRepository(pool)
	invoiceRepo := database.NewInvoiceRepository(pool)
	quotaRepo := database.NewQuotaRepository(pool)
	capacityRepo := database.NewCapacityRepository(pool)
	providerStateRepo := database.NewProviderStateRepository(pool)

	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTExpiresIn)
	orgSvc := organizations.NewService(orgRepo, userRepo)
	authSvc := auth.NewService(userRepo, jwtSvc,
		auth.WithBcryptCost(cfg.BCryptCost),
		auth.WithOrganizationCreator(orgSvc),
	)
	authHandler := auth.NewHandler(authSvc)
	orgHandler := organizations.NewHandler(orgSvc)

	provider := compute.NewSimProvider(providerStateRepo, cfg.ProvisioningDelay, cfg.StopDelay)
	computeSvc := compute.NewService(computeRepo, orgRepo, provider, quotaRepo, capacityRepo)
	computeHandler := compute.NewHandler(computeSvc)

	billingSvc := billing.NewService(usageRepo, invoiceRepo, orgRepo)
	billingHandler := billing.NewHandler(billingSvc)

	// Async lifecycle worker: advances pending/stopping/terminating instances
	// based on the provider's reported state. reconcileDone lets shutdown wait
	// for the worker to actually stop before the DB pool is closed.
	reconciler := compute.NewReconciler(computeRepo, provider, slog.Default())
	reconcileCtx, stopReconciler := context.WithCancel(context.Background())
	reconcileDone := make(chan struct{})
	go func() {
		defer close(reconcileDone)
		reconciler.Run(reconcileCtx, cfg.ReconcileInterval)
	}()

	r := router.New(authHandler, orgHandler, computeHandler, billingHandler, auth.Middleware(authSvc))

	// Health probes for orchestrators and load balancers. /healthz is pure
	// liveness; /readyz verifies the database is reachable.
	r.Get("/healthz", health.Liveness())
	r.Get("/readyz", health.Readiness(pool, 5*time.Second))

	// Baseline security headers on every response. HSTS is only advertised
	// when TLS is terminated by this process; otherwise an upstream proxy sets
	// it.
	hsts := cfg.TLSCertFile != ""
	handler := middleware.SecurityHeaders(hsts)(r)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start HTTP Server
	go func() {
		slog.Info("server starting", "port", cfg.Port, "tls", hsts)
		var serveErr error
		if hsts {
			serveErr = srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			serveErr = srv.ListenAndServe()
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("server encountered an unrecoverable error", "error", serveErr)
			os.Exit(1)
		}
	}()

	// Graceful Shutdown — explicit ordering:
	//   1. stop accepting new requests and drain active ones
	//   2. cancel the reconciler and wait for it to exit
	//   3. close the database pool
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down HTTP server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown before finishing active requests", "error", err)
	}
	shutdownCancel()

	stopReconciler()
	<-reconcileDone
	slog.Info("reconciler stopped")

	slog.Info("closing database connection pool...")
	pool.Close()

	slog.Info("server exited cleanly")
}
