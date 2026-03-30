package auth

import "context"

type ctxKey int

const (
	keyUserID ctxKey = iota + 1
	keyEmptyAuthCookie
)

// WithUserID кладёт идентификатор пользователя в контекст.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, keyUserID, userID)
}

// UserIDFromContext возвращает userID, если middleware его установил.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(keyUserID)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

// WithEmptyAuthCookie помечает запрос: кука с именем CookieName передана, но без значения.
func WithEmptyAuthCookie(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyEmptyAuthCookie, true)
}

// EmptyAuthCookieFromContext — true, если нужно ответить 401 на /api/user/urls.
func EmptyAuthCookieFromContext(ctx context.Context) bool {
	v := ctx.Value(keyEmptyAuthCookie)
	b, _ := v.(bool)
	return b
}
