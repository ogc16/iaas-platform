package main

import (
	"context"
	"fmt"
	"log/slog" // Upgraded to structured logging
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
	"github.com/ogc16/iaas-platform/internal/organizations"
	"github.com/ogc16/iaas-platform/internal/router"
)

func main() {
	// Initialize structured json logger for production readability
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		panic(err)
	}

	// Fix 1: Short-lived context scoped strictly to the connection handshake
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := database.Connect(dbCtx, cfg.DatabaseURL)
	dbCancel() // Liberate context resources immediately
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer func() {
		slog.Info("closing database connection pool...")
		pool.Close()
	}()

	// Apply pending embedded migrations at startup. Each migration runs in its
	// own transaction and applied versions are tracked in schema_migrations.
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer migrateCancel()
	if _, err := database.Migrate(migrateCtx, pool); err != nil {
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
	authSvc := auth.NewService(userRepo, jwtSvc)
	authHandler := auth.NewHandler(authSvc)

	orgSvc := organizations.NewService(orgRepo, userRepo)
	orgHandler := organizations.NewHandler(orgSvc)

	provider := compute.NewSimProvider(providerStateRepo, cfg.ProvisioningDelay, cfg.StopDelay)
	computeSvc := compute.NewService(computeRepo, orgRepo, provider, quotaRepo, capacityRepo)
	computeHandler := compute.NewHandler(computeSvc)

	billingSvc := billing.NewService(usageRepo, invoiceRepo, orgRepo)
	billingHandler := billing.NewHandler(billingSvc)

	// Async lifecycle worker: advances pending/stopping/terminating instances
	// based on the provider's reported state.
	reconciler := compute.NewReconciler(computeRepo, provider, slog.Default())
	reconcileCtx, stopReconciler := context.WithCancel(context.Background())
	defer stopReconciler()
	go reconciler.Run(reconcileCtx, cfg.ReconcileInterval)

	r := router.New(authHandler, orgHandler, computeHandler, billingHandler, auth.Middleware(authSvc))

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP Server
	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server encountered an unrecoverable error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful Shutdown Listening
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down HTTP server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Fix 2: active requests drain out completely while DB pool is still alive
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown before finishing active requests", "error", err)
	}

	slog.Info("server exited cleanly")
}
