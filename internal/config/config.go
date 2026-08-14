package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const (
	DefaultEnvironment   = "development"
	DefaultJWTSecret     = "change-me-in-production"
	MinJWTSecretLength   = 32
	DefaultBCryptCost    = 12
	MinBCryptCost        = 4
	MaxBCryptCost        = 15
	DefaultDBMaxConns    = 20
	DefaultDBMinConns    = 2
	DefaultAppBaseURL    = "http://localhost:8080"
	DefaultSMTPPort      = 587
	DefaultResetTTLHours = 24
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
	DBMaxConns        int32
	DBMinConns        int32
	JWTSecret         string
	JWTIssuer         string
	JWTExpiresIn      int
	BCryptCost        int
	Environment       string
	ProvisioningDelay time.Duration
	StopDelay         time.Duration
	ReconcileInterval time.Duration
	TLSCertFile       string
	TLSKeyFile        string
	AppBaseURL        string
	SMTPHost          string
	SMTPPort          int
	SMTPUsername      string
	SMTPPassword      string
	SMTPFrom          string
	PasswordResetTTL  time.Duration
}

func Load() (*Config, error) {
	// Load .env into the process environment. Real environment variables take
	// precedence (godotenv never overwrites them), and a missing .env is fine
	// when variables come from the shell, CI, or an orchestrator.
	_ = godotenv.Load()

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

	bcryptCost, err := strconv.Atoi(getEnv("BCRYPT_COST", strconv.Itoa(DefaultBCryptCost)))
	if err != nil {
		return nil, fmt.Errorf("invalid BCRYPT_COST: %w", err)
	}
	if bcryptCost < MinBCryptCost || bcryptCost > MaxBCryptCost {
		return nil, fmt.Errorf("BCRYPT_COST must be between %d and %d", MinBCryptCost, MaxBCryptCost)
	}

	dbMaxConns, err := strconv.Atoi(getEnv("DB_MAX_CONNS", strconv.Itoa(DefaultDBMaxConns)))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MAX_CONNS: %w", err)
	}
	dbMinConns, err := strconv.Atoi(getEnv("DB_MIN_CONNS", strconv.Itoa(DefaultDBMinConns)))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MIN_CONNS: %w", err)
	}
	if dbMinConns > dbMaxConns {
		return nil, fmt.Errorf("DB_MIN_CONNS (%d) cannot exceed DB_MAX_CONNS (%d)", dbMinConns, dbMaxConns)
	}

	tlsCertFile := getEnv("TLS_CERT_FILE", "")
	tlsKeyFile := getEnv("TLS_KEY_FILE", "")
	if (tlsCertFile == "") != (tlsKeyFile == "") {
		return nil, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must be set together")
	}

	smtpPort, err := strconv.Atoi(getEnv("SMTP_PORT", strconv.Itoa(DefaultSMTPPort)))
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}
	resetTTLHours, err := strconv.Atoi(getEnv("PASSWORD_RESET_TTL_HOURS", strconv.Itoa(DefaultResetTTLHours)))
	if err != nil {
		return nil, fmt.Errorf("invalid PASSWORD_RESET_TTL_HOURS: %w", err)
	}

	environment := getEnv("ENV", DefaultEnvironment)
	cfg := &Config{
		Port:              port,
		DatabaseURL:       databaseURL(),
		DBMaxConns:        int32(dbMaxConns),
		DBMinConns:        int32(dbMinConns),
		JWTSecret:         getEnv("JWT_SECRET", DefaultJWTSecret),
		JWTIssuer:         getEnv("JWT_ISSUER", "iaas-platform"),
		JWTExpiresIn:      jwtExpiresIn,
		BCryptCost:        bcryptCost,
		Environment:       environment,
		ProvisioningDelay: provisioningDelay,
		StopDelay:         stopDelay,
		ReconcileInterval: reconcileInterval,
		TLSCertFile:       tlsCertFile,
		TLSKeyFile:        tlsKeyFile,
		AppBaseURL:        getEnv("APP_BASE_URL", DefaultAppBaseURL),
		SMTPHost:          getEnv("SMTP_HOST", ""),
		SMTPPort:          smtpPort,
		SMTPUsername:      getEnv("SMTP_USERNAME", ""),
		SMTPPassword:      getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:          getEnv("SMTP_FROM", ""),
		PasswordResetTTL:  time.Duration(resetTTLHours) * time.Hour,
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
