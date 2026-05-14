package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/block-vision/sui-go-sdk/signer"
	suisdk "github.com/block-vision/sui-go-sdk/sui"
	"github.com/fystack/b2b-merchant/internal/crypto"
	"github.com/fystack/b2b-merchant/internal/queue"
	suipkg "github.com/fystack/b2b-merchant/internal/sui"
)

const (
	maxSweepAttempts = 3
)

// StartB2BSuiTreasuryWorker consumes sweep instructions from the Sui treasury queue,
// decrypts the temporary wallet key, and moves all USDC to the treasury wallet via
// a sponsor-paid transaction.
//
// Ack strategy:
//
//	permanent error (bad message, no pending order)  → Nack(false, false) → DLQ
//	transient error (RPC failure, insufficient gas)  → Nack(false, true)  → requeue
//	after maxSweepAttempts transient failures         → Nack(false, false) → DLQ
//	success                                           → Ack(false)
func StartB2BSuiTreasuryWorker(q *queue.Queue, db *sql.DB, enc *crypto.Encryptor) {
	log.Println("[B2B_SUI_TREASURY_WORKER] Starting...")

	rpcURL := os.Getenv("SUI_RPC_URL")
	if rpcURL == "" {
		rpcURL = suipkg.RPCURL()
	}

	treasuryWallet := os.Getenv("TREASURY_WALLET_SUI")
	if treasuryWallet == "" {
		log.Fatalf("[B2B_SUI_TREASURY_WORKER] TREASURY_WALLET_SUI env var not set")
	}

	// NewSuiClient returns ISuiAPI; the transaction package needs *sui.Client.
	cli := suisdk.NewSuiClient(rpcURL)
	suiClient, ok := cli.(*suisdk.Client)
	if !ok {
		log.Fatalf("[B2B_SUI_TREASURY_WORKER] Failed to cast Sui client")
	}

	ch, err := q.NewConsumerChannel()
	if err != nil {
		log.Fatalf("[B2B_SUI_TREASURY_WORKER] Failed to open consumer channel: %v", err)
	}
	defer ch.Close()

	deliveries, err := ch.Consume(
		queue.QueueSuiTreasury,
		"b2b-sui-treasury-worker",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("[B2B_SUI_TREASURY_WORKER] Failed to register consumer: %v", err)
	}

	log.Printf("[B2B_SUI_TREASURY_WORKER] Listening on %s → treasury=%s rpc=%s",
		queue.QueueSuiTreasury, treasuryWallet, rpcURL)

	// attempts tracks per-delivery retry counts (keyed by AMQP delivery tag).
	attempts := make(map[uint64]int)

	for d := range deliveries {
		start := time.Now()

		// Parse the incoming body — the order worker forwards raw OrderMessage JSON.
		var msg queue.OrderMessage
		if err := json.Unmarshal(d.Body, &msg); err != nil {
			log.Printf("[B2B_SUI_TREASURY_WORKER] Permanent error: unmarshal failed: %v — DLQ", err)
			delete(attempts, d.DeliveryTag)
			_ = d.Nack(false, false)
			continue
		}

		log.Printf("[B2B_SUI_TREASURY_WORKER] Processing deposit_id=%s amount=%.6f", msg.OrderID, msg.AmountUSDC)

		err := executeSweep(context.Background(), db, enc, suiClient, msg.OrderID, treasuryWallet)
		if err != nil {
			attempts[d.DeliveryTag]++
			n := attempts[d.DeliveryTag]
			log.Printf("[B2B_SUI_TREASURY_WORKER] Sweep attempt %d/%d failed deposit_id=%s: %v",
				n, maxSweepAttempts, msg.OrderID, err)

			if n >= maxSweepAttempts {
				log.Printf("[B2B_SUI_TREASURY_WORKER] Max attempts reached deposit_id=%s — DLQ", msg.OrderID)
				if pubErr := queue.PublishB2BDLQ(q, queue.ChainSui, msg.OrderID, err.Error()); pubErr != nil {
					log.Printf("[B2B_SUI_TREASURY_WORKER] DLQ publish failed: %v", pubErr)
				}
				delete(attempts, d.DeliveryTag)
				_ = d.Nack(false, false)
			} else {
				_ = d.Nack(false, true)
			}
			continue
		}

		delete(attempts, d.DeliveryTag)
		_ = d.Ack(false)
		log.Printf("[B2B_SUI_TREASURY_WORKER] Swept deposit_id=%s in %v", msg.OrderID, time.Since(start))
	}

	log.Println("[B2B_SUI_TREASURY_WORKER] Delivery channel closed — worker exiting")
}

// executeSweep performs a single sweep attempt:
// 1. Look up the deposit and its linked pending_order to get the temp wallet + encrypted key
// 2. Decrypt key → Ed25519 seed → signer
// 3. Find USDC coins in temp wallet
// 4. Execute sponsored transfer to treasury
// 5. Mark deposit as swept
func executeSweep(ctx context.Context, db *sql.DB, enc *crypto.Encryptor, client *suisdk.Client, depositID, treasuryWallet string) error {
	// ── 1. Load deposit + pending order ───────────────────────────────────────
	var tempWallet string
	var encryptedKey string
	err := db.QueryRowContext(ctx, `
		SELECT d.sui_address, po.encrypted_private_key
		FROM deposits d
		JOIN pending_orders po ON po.id = d.pending_order_id
		WHERE d.id = $1
	`, depositID).Scan(&tempWallet, &encryptedKey)

	if err == sql.ErrNoRows {
		return fmt.Errorf("permanent: deposit %s not found or has no linked pending order", depositID)
	}
	if err != nil {
		return fmt.Errorf("db lookup deposit %s: %w", depositID, err)
	}

	// ── 2. Decrypt Ed25519 seed ───────────────────────────────────────────────
	seedBytes, err := enc.Decrypt(encryptedKey)
	if err != nil {
		return fmt.Errorf("permanent: decrypt key for deposit %s: %w", depositID, err)
	}
	if len(seedBytes) != 32 {
		return fmt.Errorf("permanent: expected 32-byte seed, got %d for deposit %s", len(seedBytes), depositID)
	}

	rawSigner := signer.NewSigner(seedBytes)

	log.Printf("[B2B_SUI_TREASURY_WORKER] Signer address=%s deposit=%s", rawSigner.Address, depositID)

	// ── 3. Get USDC coins in temp wallet ──────────────────────────────────────
	coinIDs, err := suipkg.GetUSDCObjectIDs(tempWallet, client)
	if err != nil {
		return fmt.Errorf("get USDC coins for %s: %w", tempWallet, err)
	}
	if len(coinIDs) == 0 {
		return fmt.Errorf("no USDC coins found in temp wallet %s (deposit %s)", tempWallet, depositID)
	}

	// ── 4. Execute sponsored sweep ────────────────────────────────────────────
	digest, err := suipkg.SponsoredSweep(ctx, client, rawSigner, coinIDs, treasuryWallet)
	if err != nil {
		return fmt.Errorf("sponsored sweep deposit %s: %w", depositID, err)
	}

	// ── 5. Mark deposit swept ─────────────────────────────────────────────────
	_, err = db.ExecContext(ctx,
		`UPDATE deposits SET status = 'swept', updated_at = NOW() WHERE id = $1`,
		depositID,
	)
	if err != nil {
		// Sweep succeeded on-chain; log the digest so it's recoverable even if the DB write fails.
		log.Printf("[B2B_SUI_TREASURY_WORKER] WARNING: sweep confirmed on-chain (digest=%s) but DB update failed for deposit %s: %v",
			digest, depositID, err)
		return fmt.Errorf("db update swept status deposit %s (digest=%s): %w", depositID, digest, err)
	}

	log.Printf("[B2B_SUI_TREASURY_WORKER] deposit=%s swept digest=%s treasury=%s", depositID, digest, treasuryWallet)
	return nil
}
