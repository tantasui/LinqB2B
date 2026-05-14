package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/fystack/b2b-merchant/internal/crypto"
	"github.com/fystack/b2b-merchant/internal/wallet"
	"github.com/tyler-smith/go-bip39"
)

// Service handles the business logic for creating pending orders.
type Service struct {
	db        *sql.DB
	encryptor *crypto.Encryptor
}

// NewService creates a new order service.
func NewService(db *sql.DB, encryptor *crypto.Encryptor) *Service {
	return &Service{db: db, encryptor: encryptor}
}

// CreatePendingOrder validates the request, fetches the exchange rate from Nomba,
// calculates the USDC amount, and inserts a pending order into the database.
func (s *Service) CreatePendingOrder(ctx context.Context, merchantID string, req CreateOrderRequest) (*CreateOrderResponse, error) {
	// ── 1. Validate ──────────────────────────────────────────────────────────
	if req.AmountNGN <= 0 {
		return nil, errors.New("amount_ngn must be greater than 0")
	}
	if req.AmountNGN > 10_000_000 {
		return nil, errors.New("amount_ngn must not exceed 10,000,000")
	}

	// ── 2. Look up merchant's business ID ────────────────────────────────────
	var businessID string
	err := s.db.QueryRowContext(ctx,
		`SELECT business_id FROM merchants WHERE id = $1 AND deleted_at IS NULL`,
		merchantID,
	).Scan(&businessID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("merchant not found")
	}
	if err != nil {
		return nil, fmt.Errorf("lookup merchant: %w", err)
	}

	// ── 3. Fetch exchange rate from Nomba ────────────────────────────────────
	exchangeRate, err := FetchExchangeRate()
	if err != nil {
		return nil, fmt.Errorf("exchange_rate_unavailable: %w", err)
	}

	// ── 4. Calculate USDC amount ─────────────────────────────────────────────
	// amount_usdc = amount_ngn / exchange_rate
	// Round to 6 decimal places (USDC precision)
	expectedAmountUSDC := math.Round((req.AmountNGN/exchangeRate)*1_000_000) / 1_000_000

	if expectedAmountUSDC <= 0 {
		return nil, errors.New("calculated USDC amount is too small")
	}

	// ── 5. Generate Unique Wallet ─────────────────────────────────────────────
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return nil, fmt.Errorf("failed to generate entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return nil, fmt.Errorf("failed to generate mnemonic: %w", err)
	}

	mgr, err := wallet.NewWalletManager(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet manager: %w", err)
	}

	suiAddress, err := mgr.DeriveAddress("sui", 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive address: %w", err)
	}

	privKeyBytes, err := mgr.DeriveSuiPrivateKeyBytes(0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive private key: %w", err)
	}

	encryptedKey, err := s.encryptor.Encrypt(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	// ── 6. Insert pending order & wallet_address inside a transaction ────────
	now := time.Now()
	expiresAt := now.Add(1 * time.Hour)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var orderID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO pending_orders
			(merchant_id, amount_ngn, expected_amount_usdc, exchange_rate, merchant_address, encrypted_private_key, customer_email, status, expires_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, 'pending', $8)
		RETURNING id
	`, merchantID, req.AmountNGN, expectedAmountUSDC, exchangeRate, suiAddress, encryptedKey, nullableString(req.CustomerEmail), expiresAt,
	).Scan(&orderID)

	if err != nil {
		return nil, fmt.Errorf("insert pending order: %w", err)
	}

	// Tell indexer to watch this new unique address
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_addresses (address, type, standard, business_id, asset_type, active, created_at, updated_at)
		VALUES ($1, 'sui', 'native', $2, 'USDC', true, NOW(), NOW())
	`, suiAddress, businessID)

	if err != nil {
		return nil, fmt.Errorf("insert wallet_address: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	log.Printf("[ORDER] Pending order created — id=%s merchant=%s unique_wallet=%s usdc=%.6f",
		orderID, merchantID, suiAddress, expectedAmountUSDC)

	// ── 6. Return response ───────────────────────────────────────────────────
	return &CreateOrderResponse{
		PendingOrderID:  orderID,
		AmountUSDC:      expectedAmountUSDC,
		MerchantAddress: suiAddress,
		ExchangeRate:    exchangeRate,
		ExpiresAt:       expiresAt.Format(time.RFC3339),
	}, nil
}

// GetOrderStatus looks up a pending order and any matched deposit to determine
// whether payment has been received.
func (s *Service) GetOrderStatus(ctx context.Context, pendingOrderID string) (*OrderStatusResponse, error) {
	var (
		orderID     string
		expiresAt   time.Time
		amountUSDC  float64
		orderStatus string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, expires_at, expected_amount_usdc, status
		FROM pending_orders
		WHERE id = $1
	`, pendingOrderID).Scan(&orderID, &expiresAt, &amountUSDC, &orderStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("order not found")
	}
	if err != nil {
		return nil, fmt.Errorf("lookup order: %w", err)
	}

	// Check for a matched deposit
	var depositStatus sql.NullString
	_ = s.db.QueryRowContext(ctx, `
		SELECT status FROM deposits WHERE pending_order_id = $1 LIMIT 1
	`, pendingOrderID).Scan(&depositStatus)

	status := "pending"
	if depositStatus.Valid {
		status = "received"
	} else if time.Now().After(expiresAt) {
		status = "expired"
	}

	var ds *string
	if depositStatus.Valid {
		ds = &depositStatus.String
	}

	return &OrderStatusResponse{
		PendingOrderID: orderID,
		Status:         status,
		DepositStatus:  ds,
		AmountUSDC:     amountUSDC,
		ExpiresAt:      expiresAt.Format(time.RFC3339),
	}, nil
}

// nullableString returns a sql.NullString for optional fields.
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
