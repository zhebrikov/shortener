package middleware

import "testing"

func TestIPInTrustedSubnet(t *testing.T) {
	tests := []struct {
		name          string
		trustedSubnet string
		ip            string
		want          bool
	}{
		{"empty subnet denies", "", "127.0.0.1", false},
		{"missing ip denies", "127.0.0.0/8", "", false},
		{"invalid subnet denies", "not-cidr", "127.0.0.1", false},
		{"invalid ip denies", "127.0.0.0/8", "not-ip", false},
		{"ip in subnet", "127.0.0.0/8", "127.0.0.1", true},
		{"ip outside subnet", "10.0.0.0/8", "192.168.1.1", false},
		{"exact host", "192.168.1.10/32", "192.168.1.10", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IPInTrustedSubnet(tt.trustedSubnet, tt.ip); got != tt.want {
				t.Errorf("IPInTrustedSubnet(%q, %q) = %v, want %v", tt.trustedSubnet, tt.ip, got, tt.want)
			}
		})
	}
}
