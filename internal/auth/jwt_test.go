package auth

import (
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTService_RoundTrip(t *testing.T) {
	svc := NewJWTService("test-secret", "test-issuer", 3600)

	token, err := svc.GenerateToken(42, "alice@example.com", "admin")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != 42 || claims.Email != "alice@example.com" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.Issuer != "test-issuer" {
		t.Fatalf("unexpected issuer: %q", claims.Issuer)
	}
}

func TestJWTService_WrongSecretRejected(t *testing.T) {
	issuer := NewJWTService("secret-a", "issuer", 3600)
	other := NewJWTService("secret-b", "issuer", 3600)

	token, err := issuer.GenerateToken(1, "a@b.c", "user")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if _, err := other.ValidateToken(token); err == nil {
		t.Fatal("expected token signed with a different secret to be rejected")
	}
}

func TestJWTService_ExpiredTokenRejected(t *testing.T) {
	svc := NewJWTService("secret", "issuer", -10)

	token, err := svc.GenerateToken(1, "a@b.c", "user")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if _, err := svc.ValidateToken(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestJWTService_NonHMACTokenRejected(t *testing.T) {
	svc := NewJWTService("secret", "issuer", 3600)

	token := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{UserID: 1})
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	if _, err := svc.ValidateToken(signed); err == nil {
		t.Fatal("expected token using a non-HMAC signing method to be rejected")
	}
}

func TestJWTService_GarbageTokenRejected(t *testing.T) {
	svc := NewJWTService("secret", "issuer", 3600)

	for _, bad := range []string{"", "not-a-jwt", "a.b.c"} {
		if _, err := svc.ValidateToken(bad); err == nil {
			t.Fatalf("expected token %q to be rejected", bad)
		}
	}
}

func TestGenerateAPIKey_FormatAndUniqueness(t *testing.T) {
	k1, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generateAPIKey: %v", err)
	}
	k2, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generateAPIKey: %v", err)
	}

	if !strings.HasPrefix(k1, "iaas_") || len(k1) != len("iaas_")+32 {
		t.Fatalf("unexpected api key format: %q", k1)
	}
	if k1 == k2 {
		t.Fatal("expected API keys to be unique")
	}
}
