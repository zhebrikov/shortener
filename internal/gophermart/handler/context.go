package handler

import "context"

type ctxKey int

const userIDKey ctxKey = 1

// WithUserID returns a child context that carries the authenticated user id.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext returns the authenticated user id placed by auth middleware.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(userIDKey)
	s, ok := v.(string)
	return s, ok && s != ""
}
