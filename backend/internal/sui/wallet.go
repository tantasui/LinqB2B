package sui

import (
	"context"
	"fmt"
	"log"

	"github.com/block-vision/sui-go-sdk/models"
	"github.com/block-vision/sui-go-sdk/sui"
)

const (
	suiCoinType = "0x0000000000000000000000000000000000000000000000000000000000000002::sui::SUI"
)

// USDCCoinType returns the USDC coin type for the active network.
func USDCCoinType() string { return CoinType() }

// GetUSDCObjectIDs returns all USDC coin object IDs owned by address.
// Ported from linq-v2 walletHelper.GetUSDCObjectIds (JSON-RPC path only).
func GetUSDCObjectIDs(address string, client *sui.Client) ([]string, error) {
	ctx := context.Background()
	resp, err := client.SuiXGetCoins(ctx, models.SuiXGetCoinsRequest{
		Owner:    address,
		CoinType: USDCCoinType(),
	})
	if err != nil {
		return nil, fmt.Errorf("SuiXGetCoins USDC owner=%s: %w", address, err)
	}
	var ids []string
	for _, coin := range resp.Data {
		ids = append(ids, coin.CoinObjectId)
		log.Printf("[SUI_WALLET] USDC coin=%s balance=%s owner=%s", coin.CoinObjectId, coin.Balance, address)
	}
	return ids, nil
}

// GetSuiObjectIDs returns all SUI gas coin object IDs owned by address.
func GetSuiObjectIDs(address string, client *sui.Client) ([]string, error) {
	ctx := context.Background()
	resp, err := client.SuiXGetCoins(ctx, models.SuiXGetCoinsRequest{
		Owner:    address,
		CoinType: suiCoinType,
	})
	if err != nil {
		return nil, fmt.Errorf("SuiXGetCoins SUI owner=%s: %w", address, err)
	}
	var ids []string
	for _, coin := range resp.Data {
		ids = append(ids, coin.CoinObjectId)
	}
	return ids, nil
}
