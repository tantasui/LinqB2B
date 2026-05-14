package webhook

import (
	"testing"
)

func TestRawToUSDC(t *testing.T) {
	tests := []struct {
		raw  string
		want float64
	}{
		{"", 0},
		{"0", 0},
		{"1000000", 1.0},
		{"500000", 0.5},
		{"100000", 0.1},
		{"1", 0.000001},
		{"123456789", 123.456789},
		{"1000000000", 1000.0},
		{"  1000000  ", 1.0}, // trims whitespace
	}
	for _, tc := range tests {
		got := rawToUSDC(tc.raw)
		if got != tc.want {
			t.Errorf("rawToUSDC(%q) = %f, want %f", tc.raw, got, tc.want)
		}
	}
}
