// Package luhn implements the Luhn checksum algorithm for validating numeric identifiers such as order numbers.
package luhn

import (
	"strings"
	"unicode"
)

// Valid reports whether s consists only of decimal digits and satisfies the Luhn algorithm.
func Valid(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	sum := 0
	double := false
	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
