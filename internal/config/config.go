package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	DefaultEnvironment = "development"
	DefaultJWTSecret   = "change-me-in-production"
	MinJWTSecretLength = 32
)

// placeholderJWTSecrets are well-known placeholder values that must never be
// used to sign tokens outside development. The first entry matches the value
// documented in .env.example; the second is the code default.
var placeholderJWTSecrets = map[string]bool{
	"change-me-to-a-secure-random-string": true,
	DefaultJWTSecret:                      true,
	"":                                    true,
}

type Config struct {
	Port              int
	DatabaseURL       string
	JWTSecret         string
	JWTIssuer         string
	JWTExpiresIn      int
	Environment       string
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

	environment := getEnv("ENV", DefaultEnvironment)
	cfg := &Config{
		Port:              port,
		DatabaseURL:       databaseURL(),
		JWTSecret:         getEnv("JWT_SECRET", DefaultJWTSecret),
		JWTIssuer:         getEnv("JWT_ISSUER", "iaas-platform"),
		JWTExpiresIn:      jwtExpiresIn,
		Environment:       environment,
		ProvisioningDelay: provisioningDelay,
		StopDelay:         stopDelay,
		ReconcileInterval: reconcileInterval,
	}

	// Refuse to boot outside development with a weak or placeholder signing
	// secret. A leaked or default JWT secret would let anyone forge tokens.
	if environment != DefaultEnvironment {
		if err := validateProductionSecrets(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func validateProductionSecrets(cfg *Config) error {
	if placeholderJWTSecrets[cfg.JWTSecret] {
		return fmt.Errorf("JWT_SECRET must be a strong random value in %s mode; refusing to boot with a placeholder secret", cfg.Environment)
	}
	if len(cfg.JWTSecret) < MinJWTSecretLength {
		return fmt.Errorf("JWT_SECRET must be at least %d characters in %s mode (got %d)", MinJWTSecretLength, cfg.Environment, len(cfg.JWTSecret))
	}
	return nil
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
