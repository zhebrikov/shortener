package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseTrustedSubnet(t *testing.T) {
	t.Run("empty is allowed", func(t *testing.T) {
		ts, err := ParseTrustedSubnet("")
		if err != nil {
			t.Fatalf("ParseTrustedSubnet: %v", err)
		}
		if ts.IsConfigured() {
			t.Fatal("empty subnet must not be configured")
		}
		if ts.Contains("127.0.0.1") {
			t.Fatal("empty subnet must deny all")
		}
	})

	t.Run("invalid CIDR", func(t *testing.T) {
		if _, err := ParseTrustedSubnet("not-cidr"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("valid CIDR", func(t *testing.T) {
		ts, err := ParseTrustedSubnet("127.0.0.0/8")
		if err != nil {
			t.Fatalf("ParseTrustedSubnet: %v", err)
		}
		if !ts.IsConfigured() {
			t.Fatal("expected configured subnet")
		}
		if !ts.Contains("127.0.0.1") {
			t.Fatal("expected ip in subnet")
		}
	})
}

func TestTrustedSubnetContains(t *testing.T) {
	tests := []struct {
		name          string
		trustedSubnet string
		ip            string
		want          bool
	}{
		{"empty subnet denies", "", "127.0.0.1", false},
		{"missing ip denies", "127.0.0.0/8", "", false},
		{"invalid ip denies", "127.0.0.0/8", "not-ip", false},
		{"ip in subnet", "127.0.0.0/8", "127.0.0.1", true},
		{"ip outside subnet", "10.0.0.0/8", "192.168.1.1", false},
		{"exact host", "192.168.1.10/32", "192.168.1.10", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := ParseTrustedSubnet(tt.trustedSubnet)
			if err != nil {
				t.Fatalf("ParseTrustedSubnet(%q): %v", tt.trustedSubnet, err)
			}
			if got := ts.Contains(tt.ip); got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestTrustedSubnetOnly(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("forbidden when IP outside subnet", func(t *testing.T) {
		ts, err := ParseTrustedSubnet("10.0.0.0/8")
		if err != nil {
			t.Fatalf("ParseTrustedSubnet: %v", err)
		}
		h := TrustedSubnetOnly(ts)(okHandler)

		req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
		req.Header.Set("X-Real-IP", "192.168.1.1")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("success when IP in subnet", func(t *testing.T) {
		ts, err := ParseTrustedSubnet("127.0.0.0/8")
		if err != nil {
			t.Fatalf("ParseTrustedSubnet: %v", err)
		}
		h := TrustedSubnetOnly(ts)(okHandler)

		req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
		req.Header.Set("X-Real-IP", "127.0.0.1")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
}
