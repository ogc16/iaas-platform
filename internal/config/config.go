package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port              int
	DatabaseURL       string
	JWTSecret         string
	JWTIssuer         string
	JWTExpiresIn      int
	ProvisioningDelay time.Duration
	StopDelay         time.Duration
	ReconcileInterval time.Duration
}

func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %w", err)
	}

	jwtExpiresIn, err := strconv.Atoi(getEnv("JWT_EXPIRES_IN", "86400"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRES_IN: %w", err)
	}

	provisioningDelay, err := seconds(getEnv("PROVISIONING_DELAY_SECONDS", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid PROVISIONING_DELAY_SECONDS: %w", err)
	}
	stopDelay, err := seconds(getEnv("STOP_DELAY_SECONDS", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid STOP_DELAY_SECONDS: %w", err)
	}
	reconcileInterval, err := seconds(getEnv("RECONCILE_INTERVAL_SECONDS", "2"))
	if err != nil {
		return nil, fmt.Errorf("invalid RECONCILE_INTERVAL_SECONDS: %w", err)
	}

	return &Config{
		Port:              port,
		DatabaseURL:       databaseURL(),
		JWTSecret:         getEnv("JWT_SECRET", "change-me-in-production"),
		JWTIssuer:         getEnv("JWT_ISSUER", "iaas-platform"),
		JWTExpiresIn:      jwtExpiresIn,
		ProvisioningDelay: provisioningDelay,
		StopDelay:         stopDelay,
		ReconcileInterval: reconcileInterval,
	}, nil
}

func seconds(s string) (time.Duration, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return time.Duration(n) * time.Second, nil
}

func databaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	user := getEnv("POSTGRES_USER", "iaas")
	password := os.Getenv("POSTGRES_PASSWORD")
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	db := getEnv("POSTGRES_DB", "iaas")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, db)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
