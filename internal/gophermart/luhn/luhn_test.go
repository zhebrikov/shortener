package luhn

import "testing"

func TestValid(t *testing.T) {
	tests := []struct {
		in    string
		valid bool
	}{
		{"12345678903", true},
		{"4532015112830366", true},
		{"79927398713", true},
		{"123", false},
		{"", false},
		{"12a34", false},
		{" 12345678903 ", true},
	}
	for _, tc := range tests {
		if got := Valid(tc.in); got != tc.valid {
			t.Fatalf("Valid(%q) = %v, want %v", tc.in, got, tc.valid)
		}
	}
}
