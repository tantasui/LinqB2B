package wallet_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/fystack/b2b-merchant/internal/wallet"
)

const testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

func newManager(t *testing.T) *wallet.WalletManager {
	t.Helper()
	m, err := wallet.NewWalletManager(testMnemonic)
	if err != nil {
		t.Fatalf("NewWalletManager: %v", err)
	}
	return m
}

func TestNewWalletManager_ValidMnemonic(t *testing.T) {
	_, err := wallet.NewWalletManager(testMnemonic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewWalletManager_InvalidMnemonic(t *testing.T) {
	_, err := wallet.NewWalletManager("not a valid mnemonic phrase at all here ok")
	if err == nil {
		t.Error("expected error for invalid mnemonic")
	}
}

func TestDeriveAddress_SuiFormat(t *testing.T) {
	m := newManager(t)
	addr, err := m.DeriveAddress("sui", 0)
	if err != nil {
		t.Fatalf("DeriveAddress sui: %v", err)
	}
	if !strings.HasPrefix(addr, "0x") {
		t.Errorf("sui address should start with 0x, got %q", addr)
	}
	// Sui address is 0x + 64 hex chars (32 bytes)
	if len(addr) != 66 {
		t.Errorf("sui address length = %d, want 66", len(addr))
	}
}

func TestDeriveAddress_SolanaFormat(t *testing.T) {
	m := newManager(t)
	addr, err := m.DeriveAddress("solana", 0)
	if err != nil {
		t.Fatalf("DeriveAddress solana: %v", err)
	}
	// Solana addresses are base58 encoded, 32–44 chars
	if len(addr) < 32 || len(addr) > 44 {
		t.Errorf("solana address length = %d, want 32–44", len(addr))
	}
}

func TestDeriveAddress_EVMFormat(t *testing.T) {
	m := newManager(t)
	for _, network := range []string{"base", "ethereum", "evm"} {
		addr, err := m.DeriveAddress(network, 0)
		if err != nil {
			t.Fatalf("DeriveAddress %s: %v", network, err)
		}
		if !strings.HasPrefix(addr, "0x") {
			t.Errorf("%s address should start with 0x, got %q", network, addr)
		}
	}
}

func TestDeriveAddress_UnsupportedNetwork(t *testing.T) {
	m := newManager(t)
	_, err := m.DeriveAddress("bitcoin", 0)
	if err == nil {
		t.Error("expected error for unsupported network")
	}
}

func TestDeriveAddress_Deterministic(t *testing.T) {
	m1 := newManager(t)
	m2 := newManager(t)

	for _, network := range []string{"sui", "solana", "base"} {
		a1, _ := m1.DeriveAddress(network, 0)
		a2, _ := m2.DeriveAddress(network, 0)
		if a1 != a2 {
			t.Errorf("non-deterministic: %s index 0 gave %q and %q", network, a1, a2)
		}
	}
}

func TestDeriveAddress_DifferentIndicesAreDifferent(t *testing.T) {
	m := newManager(t)
	for _, network := range []string{"sui", "solana", "base"} {
		a0, _ := m.DeriveAddress(network, 0)
		a1, _ := m.DeriveAddress(network, 1)
		if a0 == a1 {
			t.Errorf("%s: index 0 and index 1 produced the same address", network)
		}
	}
}

func TestDeriveSuiPrivateKey_Format(t *testing.T) {
	m := newManager(t)
	key, err := m.DeriveSuiPrivateKey(0)
	if err != nil {
		t.Fatalf("DeriveSuiPrivateKey: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("private key is not valid base64: %v", err)
	}
	// Sui import format: flag_byte (0x00) + 32-byte seed = 33 bytes
	if len(raw) != 33 {
		t.Errorf("private key decoded length = %d, want 33", len(raw))
	}
	if raw[0] != 0x00 {
		t.Errorf("first byte (flag) = 0x%02x, want 0x00", raw[0])
	}
}

func TestDeriveSuiPrivateKeyBytes_Length(t *testing.T) {
	m := newManager(t)
	seed, err := m.DeriveSuiPrivateKeyBytes(0)
	if err != nil {
		t.Fatalf("DeriveSuiPrivateKeyBytes: %v", err)
	}
	if len(seed) != 32 {
		t.Errorf("seed length = %d, want 32", len(seed))
	}
}

func TestDeriveSuiPrivateKeyBytes_MatchesPrivateKey(t *testing.T) {
	m := newManager(t)
	seed, _ := m.DeriveSuiPrivateKeyBytes(0)
	keyB64, _ := m.DeriveSuiPrivateKey(0)
	raw, _ := base64.StdEncoding.DecodeString(keyB64)
	// raw = [0x00] + seed
	if len(raw) < 33 {
		t.Fatal("private key too short")
	}
	for i, b := range seed {
		if b != raw[i+1] {
			t.Errorf("seed byte %d: got 0x%02x, want 0x%02x", i, raw[i+1], b)
		}
	}
}

func TestDeriveSuiPrivateKeyBytes_Deterministic(t *testing.T) {
	m1 := newManager(t)
	m2 := newManager(t)
	s1, _ := m1.DeriveSuiPrivateKeyBytes(5)
	s2, _ := m2.DeriveSuiPrivateKeyBytes(5)
	for i := range s1 {
		if s1[i] != s2[i] {
			t.Errorf("non-deterministic at byte %d", i)
		}
	}
}
