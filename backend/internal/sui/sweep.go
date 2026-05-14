package sui

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/block-vision/sui-go-sdk/models"
	"github.com/block-vision/sui-go-sdk/signer"
	"github.com/block-vision/sui-go-sdk/sui"
	"github.com/block-vision/sui-go-sdk/transaction"
)

// SponsoredSweep transfers all USDC coins from rawSigner's wallet to receiverAddress.
// Gas fees are paid by the wallet identified by the SPONSOR_SEED env var.
// Returns the on-chain transaction digest on success.
//
// Ported from linq-v2 wallet/txnBuilder/sponsortrnx.go with the key difference that
// b2b-merchant passes a pre-built *signer.Signer (from a raw 32-byte Ed25519 seed)
// rather than deriving one from a mnemonic inside this function.
func SponsoredSweep(ctx context.Context, client *sui.Client, rawSigner *signer.Signer, coinObjectIDs []string, receiverAddress string) (string, error) {
	if len(coinObjectIDs) == 0 {
		return "", fmt.Errorf("SponsoredSweep: no USDC coin objects provided")
	}

	sponsorMnemonic := os.Getenv("SPONSOR_SEED")
	if sponsorMnemonic == "" {
		return "", fmt.Errorf("SponsoredSweep: SPONSOR_SEED env var not set")
	}
	sponsoredSigner, err := signer.NewSignertWithMnemonic(sponsorMnemonic)
	if err != nil {
		return "", fmt.Errorf("SponsoredSweep: create sponsor signer: %w", err)
	}

	log.Printf("[SPONSORED_SWEEP] sponsor=%s sender=%s receiver=%s coins=%d",
		sponsoredSigner.Address, rawSigner.Address, receiverAddress, len(coinObjectIDs))

	// Get sponsor SUI gas coins
	gasCoinIDs, err := GetSuiObjectIDs(sponsoredSigner.Address, client)
	if err != nil {
		return "", fmt.Errorf("SponsoredSweep: get sponsor gas coins: %w", err)
	}
	if len(gasCoinIDs) == 0 {
		return "", fmt.Errorf("SponsoredSweep: no SUI gas coins in sponsor wallet %s", sponsoredSigner.Address)
	}

	// Build programmable transaction
	tx := transaction.NewTransaction().SetSuiClient(client)

	// Resolve object refs for every USDC coin
	var coinArgs []transaction.Argument
	for _, coinID := range coinObjectIDs {
		obj, err := client.SuiGetObject(ctx, models.SuiGetObjectRequest{ObjectId: coinID})
		if err != nil {
			return "", fmt.Errorf("SponsoredSweep: get USDC object %s: %w", coinID, err)
		}
		if obj.Data == nil {
			return "", fmt.Errorf("SponsoredSweep: USDC object %s has nil data", coinID)
		}
		ref, err := transaction.NewSuiObjectRef(
			models.SuiAddress(obj.Data.ObjectId),
			obj.Data.Version,
			models.ObjectDigest(obj.Data.Digest),
		)
		if err != nil {
			return "", fmt.Errorf("SponsoredSweep: create object ref %s: %w", coinID, err)
		}
		coinArgs = append(coinArgs, tx.Object(transaction.CallArg{
			Object: &transaction.ObjectArg{ImmOrOwnedObject: ref},
		}))
	}

	// Merge all USDC coins into the first one if there are multiple
	coinToTransfer := coinArgs[0]
	if len(coinArgs) > 1 {
		tx.MergeCoins(coinArgs[0], coinArgs[1:])
	}
	tx.TransferObjects([]transaction.Argument{coinToTransfer}, tx.Pure(receiverAddress))

	// Promote to sponsored transaction
	newTx, err := tx.NewTransactionFromKind()
	if err != nil {
		return "", fmt.Errorf("SponsoredSweep: NewTransactionFromKind: %w", err)
	}
	newTx.SetSuiClient(client)

	// Resolve gas coin object ref
	gasCoinObj, err := client.SuiGetObject(ctx, models.SuiGetObjectRequest{ObjectId: gasCoinIDs[0]})
	if err != nil {
		return "", fmt.Errorf("SponsoredSweep: get gas coin object: %w", err)
	}
	if gasCoinObj.Data == nil {
		return "", fmt.Errorf("SponsoredSweep: gas coin object %s has nil data", gasCoinIDs[0])
	}
	gasCoin, err := transaction.NewSuiObjectRef(
		models.SuiAddress(gasCoinIDs[0]),
		gasCoinObj.Data.Version,
		models.ObjectDigest(gasCoinObj.Data.Digest),
	)
	if err != nil {
		return "", fmt.Errorf("SponsoredSweep: create gas coin ref: %w", err)
	}

	newTx.
		SetSigner(rawSigner).
		SetSponsoredSigner(sponsoredSigner).
		SetSender(models.SuiAddress(rawSigner.Address)).
		SetGasPrice(1000).
		SetGasBudget(50_000_000).
		SetGasPayment([]transaction.SuiObjectRef{*gasCoin}).
		SetGasOwner(models.SuiAddress(sponsoredSigner.Address))

	resp, err := newTx.Execute(ctx,
		models.SuiTransactionBlockOptions{
			ShowInput:   true,
			ShowEffects: true,
		},
		"WaitForLocalExecution",
	)
	if err != nil {
		return "", fmt.Errorf("SponsoredSweep: execute tx: %w", err)
	}
	if resp.Effects.Status.Status != "success" {
		return "", fmt.Errorf("SponsoredSweep: on-chain failure status=%s", resp.Effects.Status.Status)
	}

	log.Printf("[SPONSORED_SWEEP] SUCCESS digest=%s sender=%s receiver=%s", resp.Digest, rawSigner.Address, receiverAddress)
	return resp.Digest, nil
}
