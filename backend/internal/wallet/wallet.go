package wallet

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/mr-tron/base58"
	"github.com/tyler-smith/go-bip39"
	"encoding/base64"

	"golang.org/x/crypto/blake2b"
)

type WalletManager struct {
	seed []byte
}

func NewWalletManager(mnemonic string) (*WalletManager, error) {
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return nil, fmt.Errorf("invalid mnemonic: %w", err)
	}
	return &WalletManager{seed: seed}, nil
}

func (m *WalletManager) DeriveAddress(network string, index uint32) (string, error) {
	switch strings.ToLower(network) {
	case "sui":
		return m.deriveSui(index)
	case "solana":
		return m.deriveSolana(index)
	case "base", "ethereum", "evm":
		return m.deriveEVM(index)
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}

func (m *WalletManager) deriveSui(index uint32) (string, error) {
	_, addr, _ := m.suiKeyMaterial(index)
	return addr, nil
}

// DeriveSuiPrivateKey returns the base64-encoded private key for the given index.
// Import into Sui Wallet: Settings → Accounts → Import Private Key → paste this.
func (m *WalletManager) DeriveSuiPrivateKey(index uint32) (string, error) {
	b64key, _, _ := m.suiKeyMaterial(index)
	return b64key, nil
}

// DeriveSuiPrivateKeyBytes returns the raw 32-byte Ed25519 seed for encryption/storage.
func (m *WalletManager) DeriveSuiPrivateKeyBytes(index uint32) ([]byte, error) {
	_, _, seed := m.suiKeyMaterial(index)
	return seed, nil
}

// suiKeyMaterial derives (base64ImportKey, address, rawSeed) for the given index.
func (m *WalletManager) suiKeyMaterial(index uint32) (b64key, address string, seed []byte) {
	data := make([]byte, len(m.seed)+4)
	copy(data, m.seed)
	binary.BigEndian.PutUint32(data[len(m.seed):], index)
	hash := sha256.Sum256(data)
	seed = make([]byte, 32)
	copy(seed, hash[:32])
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	// Sui address = blake2b_256(0x00 || pubkey), flag 0x00 = Ed25519
	payload := append([]byte{0x00}, pub...)
	addrHash := blake2b.Sum256(payload)
	address = "0x" + hex.EncodeToString(addrHash[:])
	// Sui wallet import format: base64(flag_byte || 32-byte-seed)
	b64key = base64.StdEncoding.EncodeToString(append([]byte{0x00}, seed...))
	return
}

func (m *WalletManager) deriveSolana(index uint32) (string, error) {
	data := make([]byte, len(m.seed)+4)
	copy(data, m.seed)
	binary.BigEndian.PutUint32(data[len(m.seed):], index)
	hash := sha256.Sum256(data)
	priv := ed25519.NewKeyFromSeed(hash[:32])
	pub := priv.Public().(ed25519.PublicKey)
	return base58.Encode(pub), nil
}

func (m *WalletManager) deriveEVM(index uint32) (string, error) {
	masterKey, err := hdkeychain.NewMaster(m.seed, &chaincfg.MainNetParams)
	if err != nil {
		return "", err
	}
	purpose, err := masterKey.Derive(hdkeychain.HardenedKeyStart + 44)
	if err != nil {
		return "", err
	}
	coin, err := purpose.Derive(hdkeychain.HardenedKeyStart + 60)
	if err != nil {
		return "", err
	}
	account, err := coin.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return "", err
	}
	change, err := account.Derive(0)
	if err != nil {
		return "", err
	}
	child, err := change.Derive(index)
	if err != nil {
		return "", err
	}
	pubKey, err := child.ECPubKey()
	if err != nil {
		return "", err
	}
	pub := pubKey.SerializeUncompressed()
	return "0x" + hex.EncodeToString(pub[len(pub)-20:]), nil
}
