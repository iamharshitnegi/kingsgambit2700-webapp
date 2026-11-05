package id

import (
	"regexp"
	"testing"
)

func TestNewUUIDv7(t *testing.T) {
	value := New()
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(value) {
		t.Fatalf("New() = %q, want UUIDv7", value)
	}
	if value == New() {
		t.Fatal("New() returned duplicate UUID")
	}
	if !Valid(value) {
		t.Fatalf("Valid(%q) = false", value)
	}
	if Valid("01900000-0000-4000-8000-000000000000") {
		t.Fatal("Valid() accepted a UUIDv4-shaped value")
	}
}

func TestValidReference(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"01900000-0000-4000-8000-000000000001", true},
		{"01900000-0000-7000-8000-000000000001", true},
		{"01900000-0000-7000-0000-000000000001", false},
		{"00000000-0000-0000-0000-000000000000", false},
		{"01900000-0000-7000-8000-000000000001' OR true", false},
		{"", false},
	} {
		if got := ValidReference(tc.value); got != tc.valid {
			t.Errorf("ValidReference(%q) = %v, want %v", tc.value, got, tc.valid)
		}
	}
}
