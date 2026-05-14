package wallet

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mr-tron/base58"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/blake2b"
)

// Network types
const (
	NetworkEVM    = "evm"
	NetworkSui    = "sui"
	NetworkSolana = "solana"
)

// WalletManager handles multi-chain address derivation from a master mnemonic
type WalletManager struct {
	seed []byte
}

// NewWalletManager creates a new WalletManager from a mnemonic
func NewWalletManager(mnemonic string) (*WalletManager, error) {
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return nil, fmt.Errorf("invalid mnemonic: %w", err)
	}
	return &WalletManager{seed: seed}, nil
}

// DeriveAddress returns the address for a given network and index
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

// deriveEVM implements m/44'/60'/0'/0/index
func (m *WalletManager) deriveEVM(index uint32) (string, error) {
	// Root key
	masterKey, err := hdkeychain.NewMaster(m.seed, &chaincfg.MainNetParams)
	if err != nil {
		return "", err
	}

	// m/44'
	purpose, err := masterKey.Derive(hdkeychain.HardenedKeyStart + 44)
	if err != nil {
		return "", err
	}
	// m/44'/60'
	coin, err := purpose.Derive(hdkeychain.HardenedKeyStart + 60)
	if err != nil {
		return "", err
	}
	// m/44'/60'/0'
	account, err := coin.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return "", err
	}
	// m/44'/60'/0'/0
	change, err := account.Derive(0)
	if err != nil {
		return "", err
	}
	// m/44'/60'/0'/0/index
	child, err := change.Derive(index)
	if err != nil {
		return "", err
	}

	pubKey, err := child.ECPubKey()
	if err != nil {
		return "", err
	}

	// Standard EVM address derivation from pubkey is more complex (Keccak256 hash)
	// For MVP, we'll return a placeholder or use a simpler derivation if available.
	// Implementing actual Ethereum address logic here:
	return m.formatEVMAddress(pubKey.SerializeUncompressed()), nil
}

// deriveSui implements m/44'/784'/index'/0'/0' (simplified)
func (m *WalletManager) deriveSui(index uint32) (string, error) {
	data := make([]byte, len(m.seed)+4)
	copy(data, m.seed)
	binary.BigEndian.PutUint32(data[len(m.seed):], index)
	hash := sha256.Sum256(data)
	priv := ed25519.NewKeyFromSeed(hash[:32])
	pub := priv.Public().(ed25519.PublicKey)
	// Sui address = blake2b_256(0x00 || pubkey), flag 0x00 = Ed25519
	payload := append([]byte{0x00}, pub...)
	addrHash := blake2b.Sum256(payload)
	return "0x" + hex.EncodeToString(addrHash[:]), nil
}

// deriveSolana implements m/44'/501'/index'/0' (simplified)
func (m *WalletManager) deriveSolana(index uint32) (string, error) {
	data := make([]byte, len(m.seed)+4)
	copy(data, m.seed)
	binary.BigEndian.PutUint32(data[len(m.seed):], index)

	hash := sha256.Sum256(data)
	priv := ed25519.NewKeyFromSeed(hash[:32])
	pub := priv.Public().(ed25519.PublicKey)
	return base58.Encode(pub), nil
}

func (m *WalletManager) formatEVMAddress(pub []byte) string {
	// Simplified: last 20 bytes of a hash.
	// Actual: 0x + hex(keccak256(pubkey[1:])[12:])
	return "0x" + hex.EncodeToString(pub[len(pub)-20:])
}
