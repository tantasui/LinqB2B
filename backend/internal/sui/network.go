package sui

import "os"

const (
	usdcMainnet = "0xdba34672e30cb065b1f93e3ab55318768fd6fef66c15942c9f7cb846e2f900e7::usdc::USDC"
	usdcTestnet = "0xa1ec7fc00a6f40db9693ad1415d0c193ad3906494428cf252621037bd7117e29::usdc::USDC"

	rpcMainnet = "https://fullnode.mainnet.sui.io"
	rpcTestnet = "https://fullnode.testnet.sui.io"
)

// IsTestnet returns true when SUI_NETWORK=testnet.
func IsTestnet() bool {
	return os.Getenv("SUI_NETWORK") == "testnet"
}

// RPCURL returns the Sui RPC endpoint for the configured network.
func RPCURL() string {
	if IsTestnet() {
		return rpcTestnet
	}
	return rpcMainnet
}

// CoinType returns the USDC coin type for the configured network.
func CoinType() string {
	if IsTestnet() {
		return usdcTestnet
	}
	return usdcMainnet
}
