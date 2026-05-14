package merchant

import (
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Acme Corp", "acme_corp"},
		{"  hello world  ", "hello_world"},
		{"UPPER CASE", "upper_case"},
		{"hello-world", "hello_world"},
		{"hello---world", "hello_world"},
		{"hello_world", "hello_world"},
		{"abc123", "abc123"},
		{"  _leading_  ", "leading"},
		{"special!@#chars", "special_chars"},
		{"", ""},
		{"   ", ""},
		{"MerchantName123", "merchantname123"},
	}
	for _, tc := range tests {
		got := slugify(tc.input)
		if got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGenerateSecret_Length(t *testing.T) {
	s, err := generateSecret()
	if err != nil {
		t.Fatalf("generateSecret: %v", err)
	}
	// 32 random bytes → 64 hex chars
	if len(s) != 64 {
		t.Errorf("secret length = %d, want 64", len(s))
	}
}

func TestGenerateSecret_IsHex(t *testing.T) {
	s, _ := generateSecret()
	for _, c := range s {
		if !('0' <= c && c <= '9') && !('a' <= c && c <= 'f') {
			t.Errorf("secret contains non-hex character %q", c)
		}
	}
}

func TestGenerateSecret_UniqueAcrossCalls(t *testing.T) {
	s1, _ := generateSecret()
	s2, _ := generateSecret()
	if s1 == s2 {
		t.Error("two generateSecret calls returned the same value (possible entropy issue)")
	}
}
