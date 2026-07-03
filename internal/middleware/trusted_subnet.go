package middleware

import (
	"fmt"
	"net"
	"net/http"
)

// TrustedSubnet — доверенная подсеть CIDR для внутренних API.
// Нулевое значение означает, что подсеть не задана.
type TrustedSubnet struct {
	network *net.IPNet
}

// ParseTrustedSubnet парсит CIDR. Пустая строка допустима; невалидный CIDR — ошибка.
func ParseTrustedSubnet(cidr string) (TrustedSubnet, error) {
	if cidr == "" {
		return TrustedSubnet{}, nil
	}
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return TrustedSubnet{}, fmt.Errorf("invalid trusted subnet %q: %w", cidr, err)
	}
	return TrustedSubnet{network: network}, nil
}

// IsConfigured сообщает, задана ли доверенная подсеть.
func (t TrustedSubnet) IsConfigured() bool {
	return t.network != nil
}

// TrustedSubnetOnly ограничивает доступ по заголовку X-Real-IP.
func TrustedSubnetOnly(trustedSubnet TrustedSubnet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !trustedSubnet.Contains(r.Header.Get("X-Real-IP")) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Contains проверяет, что ip входит в доверенную подсеть.
func (t TrustedSubnet) Contains(ip string) bool {
	if t.network == nil {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return t.network.Contains(parsed)
}
