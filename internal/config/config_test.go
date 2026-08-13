package config

import (
	"strings"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("JWT_EXPIRES_IN", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_USER", "")
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("POSTGRES_HOST", "")
	t.Setenv("POSTGRES_PORT", "")
	t.Setenv("POSTGRES_DB", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_ISSUER", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.JWTExpiresIn != 86400 {
		t.Fatalf("expected default JWT expiry 86400, got %d", cfg.JWTExpiresIn)
	}
	if !strings.HasPrefix(cfg.DatabaseURL, "postgres://iaas:@localhost:5432/iaas") {
		t.Fatalf("unexpected default database URL: %q", cfg.DatabaseURL)
	}
}

func TestLoad_RespectsEnvironment(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/mydb?sslmode=disable")
	t.Setenv("JWT_SECRET", "super-secret")
	t.Setenv("JWT_ISSUER", "my-issuer")
	t.Setenv("JWT_EXPIRES_IN", "3600")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://user:pass@db:5432/mydb?sslmode=disable" {
		t.Fatalf("unexpected database URL: %q", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "super-secret" || cfg.JWTIssuer != "my-issuer" || cfg.JWTExpiresIn != 3600 {
		t.Fatalf("unexpected jwt config: %+v", cfg)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("PORT", "not-a-number")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for an invalid PORT")
	}
}

func TestLoad_InvalidJWTExpiry(t *testing.T) {
	t.Setenv("PORT", "8080")
	t.Setenv("JWT_EXPIRES_IN", "soon")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for an invalid JWT_EXPIRES_IN")
	}
}
