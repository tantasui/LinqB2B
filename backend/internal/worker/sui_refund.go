package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	suisdk "github.com/block-vision/sui-go-sdk/sui"
	"github.com/fystack/b2b-merchant/internal/crypto"
	suipkg "github.com/fystack/b2b-merchant/internal/sui"

	"github.com/block-vision/sui-go-sdk/signer"
)

type SuiRefundWorker struct {
	db        *sql.DB
	enc       *crypto.Encryptor
	suiClient *suisdk.Client
	interval  time.Duration
}

func NewSuiRefundWorker(db *sql.DB, enc *crypto.Encryptor) *SuiRefundWorker {
	cli := suisdk.NewSuiClient(suipkg.RPCURL())
	suiClient, ok := cli.(*suisdk.Client)
	if !ok {
		panic("SuiRefundWorker: failed to cast Sui client")
	}
	return &SuiRefundWorker{
		db:        db,
		enc:       enc,
		suiClient: suiClient,
		interval:  10 * time.Second,
	}
}

func (w *SuiRefundWorker) Start(ctx context.Context) {
	log.Println("SuiRefundWorker: starting...")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("SuiRefundWorker: shutting down...")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *SuiRefundWorker) processBatch(ctx context.Context) {
	rows, err := w.db.QueryContext(ctx, `
		SELECT id, deposit_id, refund_amount_usdc, recipient_wallet
		FROM refunds
		WHERE status = 'pending' AND network_id LIKE 'sui%'
		FOR UPDATE SKIP LOCKED
		LIMIT 10
	`)
	if err != nil {
		log.Printf("SuiRefundWorker: query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var refundID, depositID, recipient string
		var amountUSDC float64
		if err := rows.Scan(&refundID, &depositID, &amountUSDC, &recipient); err != nil {
			log.Printf("SuiRefundWorker: scan error: %v", err)
			continue
		}
		w.processRefund(ctx, refundID, depositID, recipient)
	}
}

func (w *SuiRefundWorker) processRefund(ctx context.Context, refundID, depositID, recipient string) {
	log.Printf("SuiRefundWorker: processing refund=%s deposit=%s recipient=%s", refundID, depositID, recipient)

	txHash, err := w.executeSuiRefund(ctx, depositID, recipient)
	if err != nil {
		log.Printf("SuiRefundWorker: refund=%s failed: %v", refundID, err)
		w.db.ExecContext(ctx,
			`UPDATE refunds SET status = 'failed', failure_reason = $1, updated_at = NOW() WHERE id = $2`,
			err.Error(), refundID,
		)
		return
	}

	w.db.ExecContext(ctx,
		`UPDATE refunds SET status = 'completed', tx_hash = $1, updated_at = NOW() WHERE id = $2`,
		txHash, refundID,
	)
	w.db.ExecContext(ctx,
		`UPDATE deposits SET status = 'refunded', updated_at = NOW() WHERE id = $1`,
		depositID,
	)
	log.Printf("SuiRefundWorker: refund=%s completed digest=%s", refundID, txHash)
}

// executeSuiRefund decrypts the temp wallet key from the pending_order linked to the
// deposit, then sweeps all USDC back to the recipient (customer's address).
// The sponsor wallet (SPONSOR_SEED) pays gas — same as the treasury sweep flow.
func (w *SuiRefundWorker) executeSuiRefund(ctx context.Context, depositID, recipient string) (string, error) {
	// 1. Load temp wallet address + encrypted key from the deposit's pending order
	var tempWallet, encryptedKey string
	err := w.db.QueryRowContext(ctx, `
		SELECT d.sui_address, po.encrypted_private_key
		FROM deposits d
		JOIN pending_orders po ON po.id = d.pending_order_id
		WHERE d.id = $1
	`, depositID).Scan(&tempWallet, &encryptedKey)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("permanent: deposit %s has no linked pending order", depositID)
	}
	if err != nil {
		return "", fmt.Errorf("db lookup deposit %s: %w", depositID, err)
	}

	// 2. Decrypt Ed25519 seed → build signer
	seedBytes, err := w.enc.Decrypt(encryptedKey)
	if err != nil {
		return "", fmt.Errorf("permanent: decrypt key for deposit %s: %w", depositID, err)
	}
	if len(seedBytes) != 32 {
		return "", fmt.Errorf("permanent: expected 32-byte seed, got %d for deposit %s", len(seedBytes), depositID)
	}
	rawSigner := signer.NewSigner(seedBytes)

	// 3. Find all USDC coins in the temp wallet
	coinIDs, err := suipkg.GetUSDCObjectIDs(tempWallet, w.suiClient)
	if err != nil {
		return "", fmt.Errorf("get USDC coins for temp wallet %s: %w", tempWallet, err)
	}
	if len(coinIDs) == 0 {
		return "", fmt.Errorf("permanent: no USDC in temp wallet %s (already swept?)", tempWallet)
	}

	// 4. Sponsor-sweep all USDC → recipient (customer's address)
	digest, err := suipkg.SponsoredSweep(ctx, w.suiClient, rawSigner, coinIDs, recipient)
	if err != nil {
		return "", fmt.Errorf("sponsored sweep to %s: %w", recipient, err)
	}

	return digest, nil
}
