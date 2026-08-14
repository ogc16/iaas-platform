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
	t.Setenv("ENV", "")
	t.Setenv("PROVISIONING_DELAY_SECONDS", "")
	t.Setenv("STOP_DELAY_SECONDS", "")
	t.Setenv("RECONCILE_INTERVAL_SECONDS", "")
	t.Setenv("BCRYPT_COST", "")
	t.Setenv("DB_MAX_CONNS", "")
	t.Setenv("DB_MIN_CONNS", "")
	t.Setenv("TLS_CERT_FILE", "")
	t.Setenv("TLS_KEY_FILE", "")
	t.Setenv("APP_BASE_URL", "")
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("PASSWORD_RESET_TTL_HOURS", "")

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
	if cfg.Environment != DefaultEnvironment {
		t.Fatalf("expected default environment %q, got %q", DefaultEnvironment, cfg.Environment)
	}
	if cfg.BCryptCost != DefaultBCryptCost {
		t.Fatalf("expected default bcrypt cost %d, got %d", DefaultBCryptCost, cfg.BCryptCost)
	}
	if cfg.DBMaxConns != DefaultDBMaxConns || cfg.DBMinConns != DefaultDBMinConns {
		t.Fatalf("unexpected pool defaults: max=%d min=%d", cfg.DBMaxConns, cfg.DBMinConns)
	}
	if !strings.HasPrefix(cfg.DatabaseURL, "postgres://iaas:@localhost:5432/iaas") {
		t.Fatalf("unexpected default database URL: %q", cfg.DatabaseURL)
	}
	if cfg.AppBaseURL != DefaultAppBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultAppBaseURL, cfg.AppBaseURL)
	}
	if cfg.SMTPHost != "" || cfg.SMTPPort != DefaultSMTPPort || cfg.SMTPFrom != "" {
		t.Fatalf("unexpected smtp defaults: %+v", cfg)
	}
	if cfg.PasswordResetTTL.Hours() != DefaultResetTTLHours {
		t.Fatalf("expected default reset TTL %dh, got %v", DefaultResetTTLHours, cfg.PasswordResetTTL)
	}
}

func TestLoad_RespectsEnvironment(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/mydb?sslmode=disable")
	t.Setenv("JWT_SECRET", "super-secret")
	t.Setenv("JWT_ISSUER", "my-issuer")
	t.Setenv("JWT_EXPIRES_IN", "3600")
	t.Setenv("APP_BASE_URL", "https://iaas.example.com")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "smtp-user")
	t.Setenv("SMTP_PASSWORD", "smtp-pass")
	t.Setenv("SMTP_FROM", "noreply@example.com")
	t.Setenv("PASSWORD_RESET_TTL_HOURS", "4")

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
	if cfg.AppBaseURL != "https://iaas.example.com" {
		t.Fatalf("unexpected base URL: %q", cfg.AppBaseURL)
	}
	if cfg.SMTPHost != "smtp.example.com" || cfg.SMTPPort != 465 ||
		cfg.SMTPUsername != "smtp-user" || cfg.SMTPPassword != "smtp-pass" ||
		cfg.SMTPFrom != "noreply@example.com" {
		t.Fatalf("unexpected smtp config: %+v", cfg)
	}
	if cfg.PasswordResetTTL.Hours() != 4 {
		t.Fatalf("expected reset TTL 4h, got %v", cfg.PasswordResetTTL)
	}
}

func TestLoad_InvalidSMTPPort(t *testing.T) {
	t.Setenv("SMTP_PORT", "not-a-number")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid SMTP_PORT to fail")
	}
}

func TestLoad_InvalidResetTTL(t *testing.T) {
	t.Setenv("PASSWORD_RESET_TTL_HOURS", "not-a-number")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid PASSWORD_RESET_TTL_HOURS to fail")
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

func TestLoad_ProductionRejectsEnvExamplePlaceholder(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "change-me-to-a-secure-random-string")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for the .env.example placeholder secret in production")
	}
}

func TestLoad_ProductionRejectsCodeDefaultSecret(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for the code-default secret in production")
	}
}

func TestLoad_ProductionRejectsShortSecret(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "short-secret")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a short secret in production")
	}
}

func TestLoad_ProductionAcceptsStrongSecret(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Environment != "production" {
		t.Fatalf("expected environment production, got %q", cfg.Environment)
	}
}

func TestLoad_DevelopmentAllowsPlaceholder(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("JWT_SECRET", "change-me-to-a-secure-random-string")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Environment != "development" {
		t.Fatalf("expected environment development, got %q", cfg.Environment)
	}
}

func TestLoad_InvalidBCryptCost(t *testing.T) {
	t.Setenv("BCRYPT_COST", "99")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for an out-of-range BCRYPT_COST")
	}
}

func TestLoad_InvalidPoolSize(t *testing.T) {
	t.Setenv("DB_MIN_CONNS", "50")
	t.Setenv("DB_MAX_CONNS", "10")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when DB_MIN_CONNS exceeds DB_MAX_CONNS")
	}
}

func TestLoad_TLSRequiresBothFiles(t *testing.T) {
	t.Setenv("TLS_CERT_FILE", "/tls/server.crt")
	t.Setenv("TLS_KEY_FILE", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when only TLS_CERT_FILE is set")
	}
}

func TestLoad_RespectsNewEnvironment(t *testing.T) {
	t.Setenv("BCRYPT_COST", "14")
	t.Setenv("DB_MAX_CONNS", "100")
	t.Setenv("DB_MIN_CONNS", "5")
	t.Setenv("TLS_CERT_FILE", "/tls/server.crt")
	t.Setenv("TLS_KEY_FILE", "/tls/server.key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BCryptCost != 14 || cfg.DBMaxConns != 100 || cfg.DBMinConns != 5 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.TLSCertFile != "/tls/server.crt" || cfg.TLSKeyFile != "/tls/server.key" {
		t.Fatalf("unexpected TLS config: %+v", cfg)
	}
}
