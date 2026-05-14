package crypto_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fystack/b2b-merchant/internal/crypto"
)

func makeKey(b byte) []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = b
	}
	return k
}

func TestNewEncryptor_Valid(t *testing.T) {
	_, err := crypto.NewEncryptor(makeKey(0x01))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewEncryptor_WrongLength(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		_, err := crypto.NewEncryptor(make([]byte, n))
		if err == nil {
			t.Errorf("expected error for key length %d", n)
		}
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	enc, _ := crypto.NewEncryptor(makeKey(0xAB))
	plaintexts := [][]byte{
		[]byte("hello world"),
		[]byte(""),
		bytes.Repeat([]byte{0xFF}, 1024),
		[]byte("special chars: !@#$%^&*()"),
	}
	for _, pt := range plaintexts {
		ct, err := enc.Encrypt(pt)
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		got, err := enc.Decrypt(ct)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if !bytes.Equal(got, pt) {
			t.Errorf("roundtrip mismatch: got %q want %q", got, pt)
		}
	}
}

func TestEncrypt_ProducesDistinctCiphertexts(t *testing.T) {
	enc, _ := crypto.NewEncryptor(makeKey(0x42))
	pt := []byte("same plaintext")
	ct1, _ := enc.Encrypt(pt)
	ct2, _ := enc.Encrypt(pt)
	if ct1 == ct2 {
		t.Error("two encryptions of the same plaintext produced identical ciphertext (nonce reuse)")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	enc1, _ := crypto.NewEncryptor(makeKey(0x01))
	enc2, _ := crypto.NewEncryptor(makeKey(0x02))

	ct, _ := enc1.Encrypt([]byte("secret"))
	_, err := enc2.Decrypt(ct)
	if err == nil {
		t.Error("expected decryption error with wrong key")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	enc, _ := crypto.NewEncryptor(makeKey(0xCC))
	ct, _ := enc.Encrypt([]byte("tamper me"))

	// Flip a byte in the middle of the base64 payload.
	b := []byte(ct)
	b[len(b)/2] ^= 0xFF
	_, err := enc.Decrypt(string(b))
	if err == nil {
		t.Error("expected decryption error for tampered ciphertext")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	enc, _ := crypto.NewEncryptor(makeKey(0xDD))
	_, err := enc.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64 input")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	enc, _ := crypto.NewEncryptor(makeKey(0xEE))
	// Base64 of a single byte — shorter than the GCM nonce size.
	_, err := enc.Decrypt("AA==")
	if err == nil {
		t.Error("expected error for ciphertext shorter than nonce size")
	}
}

func TestEncrypt_OutputIsBase64(t *testing.T) {
	enc, _ := crypto.NewEncryptor(makeKey(0x10))
	ct, _ := enc.Encrypt([]byte("test"))
	// base64 uses [A-Za-z0-9+/=] — no whitespace or special chars outside that set.
	for _, ch := range ct {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=", ch) {
			t.Errorf("ciphertext contains non-base64 character %q", ch)
		}
	}
}
