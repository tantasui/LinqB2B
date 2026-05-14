package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"time"
)

// ValidatorWorker periodically scans deposits linked to pending orders
// and validates if the received amount matches the expected amount.
type ValidatorWorker struct {
	db       *sql.DB
	interval time.Duration
}

func NewValidatorWorker(db *sql.DB) *ValidatorWorker {
	return &ValidatorWorker{
		db:       db,
		interval: 10 * time.Second,
	}
}

func (w *ValidatorWorker) Start(ctx context.Context) {
	log.Println("ValidatorWorker: starting...")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("ValidatorWorker: shutting down...")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *ValidatorWorker) processBatch(ctx context.Context) {
	// We select deposits that are received and have a linked pending order.
	// Using FOR UPDATE SKIP LOCKED to prevent concurrent workers from processing the same rows.
	rows, err := w.db.QueryContext(ctx, `
		SELECT 
			d.id, 
			d.amount_usdc, 
			d.network_id, 
			d.tx_hash, 
			d.raw_payload,
			p.id, 
			p.expected_amount_usdc 
		FROM deposits d
		JOIN pending_orders p ON d.pending_order_id = p.id
		WHERE d.status = 'received' AND d.pending_order_id IS NOT NULL
		FOR UPDATE SKIP LOCKED
		LIMIT 50
	`)
	if err != nil {
		log.Printf("ValidatorWorker: error querying deposits: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			depositID         string
			amountUSDC        float64
			networkID         sql.NullString
			txHash            string
			rawPayload        []byte
			pendingOrderID    string
			expectedAmount    float64
		)

		if err := rows.Scan(
			&depositID, &amountUSDC, &networkID, &txHash, &rawPayload, 
			&pendingOrderID, &expectedAmount,
		); err != nil {
			log.Printf("ValidatorWorker: error scanning row: %v", err)
			continue
		}

		w.processDeposit(ctx, depositID, amountUSDC, expectedAmount, networkID.String, txHash, rawPayload)
	}
}

func (w *ValidatorWorker) processDeposit(
	ctx context.Context, 
	depositID string, amount float64, expected float64, 
	network string, txHash string, rawPayload []byte,
) {
	// Tolerance of 0.01 USDC
	const tolerance = 0.01
	diff := amount - expected

	if math.Abs(diff) <= tolerance {
		// Amount is correct
		_, err := w.db.ExecContext(ctx, `UPDATE deposits SET status = 'amount_validated', updated_at = NOW() WHERE id = $1`, depositID)
		if err != nil {
			log.Printf("ValidatorWorker: error updating validated deposit %s: %v", depositID, err)
			return
		}
		log.Printf("ValidatorWorker: deposit %s validated successfully (%.2f USDC)", depositID, amount)
		return
	}

	// Mismatch detected
	var refundAmount float64
	if amount > expected {
		refundAmount = amount - expected
		log.Printf("ValidatorWorker: deposit %s overpaid. Expected %.2f, got %.2f. Queuing refund for %.2f", depositID, expected, amount, refundAmount)
	} else {
		refundAmount = amount
		log.Printf("ValidatorWorker: deposit %s underpaid. Expected %.2f, got %.2f. Queuing full refund for %.2f", depositID, expected, amount, refundAmount)
	}

	// Extract sender address from payload for refund
	var payload struct {
		Transaction struct {
			FromAddress string `json:"fromAddress"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		log.Printf("ValidatorWorker: failed to unmarshal payload for deposit %s: %v", depositID, err)
		// We still mark it mismatched, but we might not have the recipient address
		// Fallback to empty string, manual intervention required
	}

	recipient := payload.Transaction.FromAddress
	if recipient == "" {
		log.Printf("ValidatorWorker: warning: no sender address found in payload for deposit %s", depositID)
	}

	if network == "" {
		network = "sui" // Fallback default
	}

	// Begin transaction to insert refund and update deposit status
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("ValidatorWorker: error starting tx: %v", err)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO refunds (deposit_id, network_id, refund_amount_usdc, recipient_wallet, status)
		VALUES ($1, $2, $3, $4, 'pending')
	`, depositID, network, refundAmount, recipient)
	if err != nil {
		log.Printf("ValidatorWorker: error inserting refund for deposit %s: %v", depositID, err)
		return
	}

	_, err = tx.ExecContext(ctx, `UPDATE deposits SET status = 'mismatch_detected', updated_at = NOW() WHERE id = $1`, depositID)
	if err != nil {
		log.Printf("ValidatorWorker: error updating mismatched deposit %s: %v", depositID, err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ValidatorWorker: error committing mismatch transaction for deposit %s: %v", depositID, err)
		return
	}
}
