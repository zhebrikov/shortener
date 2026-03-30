package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// CookieName — имя куки с подписанным идентификатором пользователя.
const CookieName = "user_id"

// SignUserID возвращает значение куки: userID и HMAC-SHA256(secret, userID) в hex.
func SignUserID(id, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(id))
	sig := hex.EncodeToString(mac.Sum(nil))
	return id + "." + sig
}

// ParseAndVerify проверяет подпись и возвращает userID при успехе.
func ParseAndVerify(value, secret string) (userID string, ok bool) {
	i := strings.LastIndex(value, ".")
	if i <= 0 || i >= len(value)-1 {
		return "", false
	}
	id := value[:i]
	sigHex := value[i+1:]
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(id))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sigHex), []byte(expected)) {
		return "", false
	}
	return id, true
}
