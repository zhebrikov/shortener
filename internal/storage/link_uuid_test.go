package storage

import "testing"

func TestNewLinkUUID(t *testing.T) {
	seen := make(map[int]struct{}, 100)
	for range 100 {
		n, err := NewLinkUUID()
		if err != nil {
			t.Fatalf("NewLinkUUID() err = %v", err)
		}
		if n <= 0 {
			t.Fatalf("NewLinkUUID() = %d, want positive", n)
		}
		if _, dup := seen[n]; dup {
			t.Fatalf("duplicate uuid %d in 100 draws (unlikely but checked)", n)
		}
		seen[n] = struct{}{}
	}
}
