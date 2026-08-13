package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/ogc16/iaas-platform/internal/config"
	"github.com/ogc16/iaas-platform/internal/database"
)

// migrate applies pending embedded migrations to the target database and
// exits. The server also applies migrations at boot; this command exists for
// explicit, staged rollouts (e.g. run before deploying the new binary).
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	applied, err := database.Migrate(ctx, pool)
	if err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	slog.Info("migrations complete", "applied", applied)
}
