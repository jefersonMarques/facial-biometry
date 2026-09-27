package cnh

import "testing"

func TestValidCPF(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"390.533.447-05", true},
		{"39053344705", true},
		{"11111111111", false},
		{"39053344706", false},
		{"123", false},
	}

	for _, test := range tests {
		if got := ValidCPF(test.value); got != test.valid {
			t.Fatalf("ValidCPF(%q) = %v, want %v", test.value, got, test.valid)
		}
	}
}
