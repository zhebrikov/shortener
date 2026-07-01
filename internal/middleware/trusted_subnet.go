package middleware

import "net"

// IPInTrustedSubnet проверяет, что ip входит в доверенную подсеть CIDR.
// При пустом trustedSubnet или невалидных данных доступ запрещён.
func IPInTrustedSubnet(trustedSubnet, ip string) bool {
	if trustedSubnet == "" {
		return false
	}
	_, network, err := net.ParseCIDR(trustedSubnet)
	if err != nil {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return network.Contains(parsed)
}
