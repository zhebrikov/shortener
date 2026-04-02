package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CookieName — имя куки с подписанным идентификатором пользователя.
const CookieName = "user_id"

const tokenMaxAgeSeconds = 60 * 60 * 24 * 365 // 1 год

type userClaims struct {
	jwt.RegisteredClaims
}

// SignUserID возвращает значение куки в формате JWT.
func SignUserID(id, secret string) string {
	// Подписываем токен симметричным ключом (HS256) и кладём userID в `sub` (Subject).
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, userClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenMaxAgeSeconds * time.Second)),
		},
	})

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		// Для HS256 и строки-ключа ошибки при подписи не ожидаются.
		panic(err)
	}
	return signed
}

// ParseAndVerify проверяет JWT и возвращает userID при успехе.
func ParseAndVerify(value, secret string) (userID string, ok bool) {
	claims := &userClaims{}
	token, err := jwt.ParseWithClaims(
		value,
		claims,
		func(token *jwt.Token) (any, error) {
			// Явно проверяем алгоритм, чтобы исключить атаки вида `alg=none` и др.
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unexpected JWT alg %q", token.Method.Alg())
			}
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || token == nil || !token.Valid || strings.TrimSpace(claims.Subject) == "" {
		return "", false
	}

	return claims.Subject, true
}
