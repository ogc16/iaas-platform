package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/ogc16/iaas-platform/internal/httpx"
	"github.com/ogc16/iaas-platform/internal/reqctx"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

func Middleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractToken(r)
			if tokenString == "" {
				httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization header"})
				return
			}

			claims, err := svc.Authenticate(r.Context(), tokenString)
			if err != nil {
				httpx.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
				return
			}

			ctx := reqctx.WithActorEmail(r.Context(), claims.Email)
			ctx = reqctx.WithUserID(ctx, claims.UserID)
			ctx = context.WithValue(ctx, UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}

	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	return ""
}

func GetClaims(ctx context.Context) *Claims {
	claims, ok := ctx.Value(UserContextKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}
