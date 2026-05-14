package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fystack/b2b-merchant/internal/wallet"
)

func main() {
	index := flag.Uint("index", 0, "derivation index (0 = first merchant)")
	flag.Parse()

	mnemonic := os.Getenv("MASTER_MNEMONIC")
	if mnemonic == "" {
		fmt.Fprintln(os.Stderr, "MASTER_MNEMONIC env var is required")
		os.Exit(1)
	}

	mgr, err := wallet.NewWalletManager(mnemonic)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid mnemonic:", err)
		os.Exit(1)
	}

	addr, err := mgr.DeriveAddress("sui", uint32(*index))
	if err != nil {
		fmt.Fprintln(os.Stderr, "derive address:", err)
		os.Exit(1)
	}

	privKey, err := mgr.DeriveSuiPrivateKey(uint32(*index))
	if err != nil {
		fmt.Fprintln(os.Stderr, "derive key:", err)
		os.Exit(1)
	}

	fmt.Printf("Index:       %d\n", *index)
	fmt.Printf("Address:     %s\n", addr)
	fmt.Printf("Private Key: %s\n", privKey)
	fmt.Println("(Import the private key into Sui Wallet: Settings → Accounts → Import Private Key)")
}
