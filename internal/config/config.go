package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port         int
	DatabaseURL  string
	JWTSecret    string
	JWTIssuer    string
	JWTExpiresIn int
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

	return &Config{
		Port:         port,
		DatabaseURL:  databaseURL(),
		JWTSecret:    getEnv("JWT_SECRET", "change-me-in-production"),
		JWTIssuer:    getEnv("JWT_ISSUER", "iaas-platform"),
		JWTExpiresIn: jwtExpiresIn,
	}, nil
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
