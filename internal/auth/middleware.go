package auth

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const cookieMaxAge = 60 * 60 * 24 * 365 // 1 год

// Middleware проверяет или выдаёт подписанную куку с user id.
func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			c, err := r.Cookie(CookieName)
			if err == nil && strings.TrimSpace(c.Value) == "" {
				next.ServeHTTP(w, r.WithContext(WithEmptyAuthCookie(ctx)))
				return
			}
			if err == nil {
				if userID, ok := ParseAndVerify(c.Value, secret); ok {
					next.ServeHTTP(w, r.WithContext(WithUserID(ctx, userID)))
					return
				}
			}
			newID := uuid.NewString()
			signed := SignUserID(newID, secret)
			http.SetCookie(w, &http.Cookie{
				Name:     CookieName,
				Value:    signed,
				Path:     "/",
				MaxAge:   cookieMaxAge,
				HttpOnly: true,
			})
			next.ServeHTTP(w, r.WithContext(WithUserID(ctx, newID)))
		})
	}
}
