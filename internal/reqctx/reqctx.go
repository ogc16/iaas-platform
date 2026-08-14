// Package reqctx carries cross-cutting request identity (authenticated user
// id and email) in the request context without coupling packages to auth. The
// auth middleware writes these values; audit and others read them. Keeping the
// keys here avoids import cycles between auth, audit and database.
package reqctx

import "context"

type contextKey string

const (
	userIDKey contextKey = "reqctx.user_id"
	emailKey  contextKey = "reqctx.email"
)

func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserID returns the authenticated user id, if any.
func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

func WithActorEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, emailKey, email)
}

// ActorEmail returns the authenticated user's email, if any.
func ActorEmail(ctx context.Context) string {
	email, _ := ctx.Value(emailKey).(string)
	return email
}
