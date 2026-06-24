package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/user/iaas-platform/internal/auth"
	"github.com/user/iaas-platform/internal/billing"
	"github.com/user/iaas-platform/internal/compute"
	"github.com/user/iaas-platform/internal/config"
	"github.com/user/iaas-platform/internal/database"
	"github.com/user/iaas-platform/internal/organizations"
	"github.com/user/iaas-platform/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	userRepo := database.NewUserRepository(pool)
	orgRepo := database.NewOrgRepository(pool)
	computeRepo := database.NewComputeRepository(pool)
	usageRepo := database.NewUsageRepository(pool)
	invoiceRepo := database.NewInvoiceRepository(pool)

	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTExpiresIn)
	authSvc := auth.NewService(userRepo, jwtSvc)
	authHandler := auth.NewHandler(authSvc)

	orgSvc := organizations.NewService(orgRepo, userRepo)
	orgHandler := organizations.NewHandler(orgSvc)

	computeSvc := compute.NewService(computeRepo, orgRepo)
	computeHandler := compute.NewHandler(computeSvc)

	billingSvc := billing.NewService(usageRepo, invoiceRepo, orgRepo)
	billingHandler := billing.NewHandler(billingSvc)

	r := router.New(authHandler, orgHandler, computeHandler, billingHandler, auth.Middleware(authSvc))

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server starting on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exited")
}
