package handler

import (
	"net/http"
	"strings"

	"github.com/zhebrikov/shortener/internal/auth"
)

// CookieName is the HTTP cookie name used for the signed JWT session.
const CookieName = "gophermart_token"

// AuthMiddleware enforces a valid JWT from the cookie or Authorization: Bearer header.
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := tokenFromRequest(r)
			if tok == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			uid, ok := auth.ParseAndVerify(tok, secret)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), uid)))
		})
	}
}

func tokenFromRequest(r *http.Request) string {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(authz) > 7 && strings.EqualFold(authz[:7], "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	c, err := r.Cookie(CookieName)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return ""
	}
	return c.Value
}

// SetSessionCookie writes the signed JWT session cookie.
func SetSessionCookie(w http.ResponseWriter, secret, userID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    auth.SignUserID(userID, secret),
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365,
		HttpOnly: true,
	})
}
