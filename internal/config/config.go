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
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://iaas:iaas@localhost:5432/iaas?sslmode=disable"),
		JWTSecret:    getEnv("JWT_SECRET", "change-me-in-production"),
		JWTIssuer:    getEnv("JWT_ISSUER", "iaas-platform"),
		JWTExpiresIn: jwtExpiresIn,
	}, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
