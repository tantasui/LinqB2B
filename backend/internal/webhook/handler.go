package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	orderqueue "github.com/fystack/b2b-merchant/internal/queue"
)

// ErrMerchantNotFound is returned by insertDeposit when no merchant matches the inbound businessID.
var ErrMerchantNotFound = errors.New("merchant not found")

const depositCacheTTL = 24 * time.Hour

// Payload mirrors the WebhookPayload sent by the dispatcher.
type Payload struct {
	EventID        string      `json:"eventId"`
	EventType      string      `json:"eventType"`
	BusinessID     string      `json:"businessId"`
	WebhookVersion string      `json:"webhookVersion"`
	SentAt         time.Time   `json:"sentAt"`
	Transaction    Transaction `json:"transaction"`
}

type Transaction struct {
	TxHash        string `json:"txHash"`
	NetworkId     string `json:"networkId"`
	BlockNumber   uint64 `json:"blockNumber"`
	FromAddress   string `json:"fromAddress"`
	ToAddress     string `json:"toAddress"`
	AssetAddress  string `json:"assetAddress"`
	Amount        string `json:"amount"`
	Type          string `json:"type"`
	TxFee         string `json:"txFee"`
	Timestamp     int64  `json:"timestamp"`
	Confirmations uint64 `json:"confirmations"`
	Status        string `json:"status"`
}

type Handler struct {
	db    *sql.DB
	queue orderqueue.Publisher
	cache *redis.Client
}

func NewHandler(db *sql.DB, q orderqueue.Publisher, rdb *redis.Client) *Handler {
	return &Handler{db: db, queue: q, cache: rdb}
}

// Receive handles POST /webhook from the dispatcher.
func (h *Handler) Receive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	sigHeader := r.Header.Get("X-Webhook-Signature")
	tsHeader := r.Header.Get("X-Webhook-Timestamp")
	log.Printf("webhook: received request headers: sig=%s ts=%s", sigHeader, tsHeader)

	if sigHeader == "" || tsHeader == "" {
		log.Printf("webhook: missing required headers")
		http.Error(w, "missing signature headers", http.StatusUnauthorized)
		return
	}

	ts, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil || time.Now().Unix()-ts > 300 {
		log.Printf("webhook: invalid or expired timestamp: %s", tsHeader)
		http.Error(w, "timestamp too old or invalid", http.StatusUnauthorized)
		return
	}

	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("webhook: failed to unmarshal body: %v | body: %s", err, string(body))
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	log.Printf("webhook: payload unmarshaled for business %q event %q", payload.BusinessID, payload.EventType)

	ctx := r.Context()

	// ── HMAC verification ─────────────────────────────────────────────────────
	secret, err := h.getWebhookSecret(ctx, payload.BusinessID)
	if err != nil {
		log.Printf("webhook: error fetching secret for %q: %v", payload.BusinessID, err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if secret == "" {
		log.Printf("webhook: business %q not found in 'businesses' table or has no secret", payload.BusinessID)
		http.Error(w, "unknown business", http.StatusUnauthorized)
		return
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sigHeader)) {
		log.Printf("webhook: signature mismatch for business %q. Expected: %s", payload.BusinessID, expected)
		http.Error(w, "signature mismatch", http.StatusUnauthorized)
		return
	}
	log.Printf("webhook: signature verified for business %q", payload.BusinessID)

	// ── Idempotency ───────────────────────────────────────────────────────────
	exists, err := h.depositExists(ctx, payload.Transaction.TxHash)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if exists {
		w.WriteHeader(http.StatusOK)
		return
	}

	// ── Validate chain before any writes ─────────────────────────────────────
	chain, ok := orderqueue.ParseChain(payload.Transaction.NetworkId)
	if !ok {
		log.Printf("webhook: unknown chain — tx_hash=%s chain=%s",
			payload.Transaction.TxHash, payload.Transaction.NetworkId)
		http.Error(w, "unsupported chain", http.StatusBadRequest)
		return
	}

	// ── 1. Postgres ───────────────────────────────────────────────────────────
	amountUSDC := rawToUSDC(payload.Transaction.Amount)
	rawJSON, _ := json.Marshal(payload)

	depositID, merchantID, err := h.insertDeposit(ctx,
		payload.BusinessID,
		payload.Transaction.TxHash,
		payload.Transaction.NetworkId,
		payload.Transaction.Amount,
		amountUSDC,
		payload.Transaction.ToAddress,
		rawJSON,
	)
	if err != nil {
		if errors.Is(err, ErrMerchantNotFound) {
			log.Printf("webhook: merchant not found — tx_hash=%s business_id=%s chain=%s",
				payload.Transaction.TxHash, payload.BusinessID, chain)
			http.Error(w, "merchant not found", http.StatusNotFound)
			return
		}
		log.Printf("webhook: insert deposit failed — tx_hash=%s chain=%s: %v",
			payload.Transaction.TxHash, chain, err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	log.Printf("webhook: deposit inserted — deposit_id=%s tx_hash=%s merchant_id=%s chain=%s amount=%.6f",
		depositID, payload.Transaction.TxHash, merchantID, chain, amountUSDC)

	msg := orderqueue.OrderMessage{
		OrderID:        depositID,
		MerchantID:     merchantID,
		Chain:          chain,
		AmountUSDC:     amountUSDC,
		TxHash:         payload.Transaction.TxHash,
		Timestamp:      time.Unix(payload.Transaction.Timestamp, 0).UTC(),
		PendingOrderID: payload.EventID,
	}

	// ── 2. Redis (optional) ───────────────────────────────────────────────────
	if h.cache != nil {
		cacheKey := "deposit:" + depositID
		cacheVal, _ := json.Marshal(msg)
		if err := h.cache.Set(ctx, cacheKey, cacheVal, depositCacheTTL).Err(); err != nil {
			log.Printf("webhook: redis write failed — tx_hash=%s merchant_id=%s chain=%s: %v",
				payload.Transaction.TxHash, merchantID, chain, err)
		} else {
			log.Printf("webhook: redis cached — key=%s tx_hash=%s merchant_id=%s chain=%s",
				cacheKey, payload.Transaction.TxHash, merchantID, chain)
		}
	}

	// ── 3. RabbitMQ ───────────────────────────────────────────────────────────
	if err := h.queue.PublishB2BOrder(ctx, msg); err != nil {
		log.Printf("webhook: rabbitmq publish failed — tx_hash=%s merchant_id=%s chain=%s: %v",
			payload.Transaction.TxHash, merchantID, chain, err)
		http.Error(w, "queue error", http.StatusInternalServerError)
		return
	}
	log.Printf("webhook: order published — deposit_id=%s tx_hash=%s merchant_id=%s chain=%s",
		depositID, payload.Transaction.TxHash, merchantID, chain)

	w.WriteHeader(http.StatusOK)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *Handler) getWebhookSecret(ctx context.Context, businessID string) (string, error) {
	var secret string
	err := h.db.QueryRowContext(ctx,
		`SELECT webhook_secret FROM businesses WHERE business_id = $1 AND deleted_at IS NULL`,
		businessID,
	).Scan(&secret)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return secret, err
}

func (h *Handler) depositExists(ctx context.Context, txHash string) (bool, error) {
	var count int
	err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM deposits WHERE tx_hash = $1`, txHash,
	).Scan(&count)
	return count > 0, err
}

// insertDeposit persists a new deposit and returns (depositID, merchantID).
// Returns ErrMerchantNotFound if no merchant matches the given businessID.
func (h *Handler) insertDeposit(
	ctx context.Context,
	businessID, txHash, networkID, amountRaw string,
	amountUSDC float64,
	suiAddress string,
	rawJSON []byte,
) (depositID, merchantID string, err error) {
	err = h.db.QueryRowContext(ctx, `
		WITH m AS (
			SELECT id FROM merchants WHERE business_id = $1::text AND deleted_at IS NULL LIMIT 1
		),
		po AS (
			SELECT id FROM pending_orders
			WHERE merchant_address = $6::text
			  AND status = 'pending'
			  AND expires_at > NOW()
			LIMIT 1
		)
		INSERT INTO deposits (merchant_id, business_id, tx_hash, amount_raw, amount_usdc, sui_address, status, raw_payload, network_id, pending_order_id)
		SELECT m.id, $1::text, $2::text, $4::text, $5::float8, $6::text, 'pending_order_validation', $7::jsonb, $3::text, po.id
		FROM m LEFT JOIN po ON true
		ON CONFLICT (tx_hash) DO NOTHING
		RETURNING id, merchant_id::text
	`, businessID, txHash, networkID, amountRaw, amountUSDC, suiAddress, rawJSON,
	).Scan(&depositID, &merchantID)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("%w: business %s", ErrMerchantNotFound, businessID)
	}
	if err != nil {
		return "", "", err
	}

	// Tell indexer to stop watching this temporary address
	_, err = h.db.ExecContext(ctx, `UPDATE wallet_addresses SET active = false WHERE address = $1`, suiAddress)
	if err != nil {
		log.Printf("webhook: warning: failed to deactivate wallet %s: %v", suiAddress, err)
		// Non-fatal, continue processing
	}

	return depositID, merchantID, nil
}

// rawToUSDC converts a raw on-chain USDC amount (6 decimal places) to float64.
func rawToUSDC(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return 0
	}
	for len(raw) <= 6 {
		raw = "0" + raw
	}
	intPart := raw[:len(raw)-6]
	fracPart := raw[len(raw)-6:]
	val, _ := strconv.ParseFloat(intPart+"."+fracPart, 64)
	return val
}
