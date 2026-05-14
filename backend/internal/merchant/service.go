package merchant

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fystack/b2b-merchant/internal/crypto"
	"github.com/fystack/b2b-merchant/internal/wallet"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/enum"
	indexermodel "github.com/fystack/multichain-indexer/b2b-platform/pkg/model"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Service struct {
	db           *sql.DB
	gormDB       *gorm.DB
	walletMgr    *wallet.WalletManager
	encryptor    *crypto.Encryptor
	businessRepo repository.Repository[indexermodel.Business]
	walletRepo   repository.Repository[indexermodel.WalletAddress]
	webhookURL   string
}

func NewService(
	db *sql.DB,
	gormDB *gorm.DB,
	mnemonic string,
	encryptor *crypto.Encryptor,
	businessRepo repository.Repository[indexermodel.Business],
	walletRepo repository.Repository[indexermodel.WalletAddress],
	webhookURL string,
) (*Service, error) {
	mgr, err := wallet.NewWalletManager(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("wallet manager: %w", err)
	}
	return &Service{
		db:           db,
		gormDB:       gormDB,
		walletMgr:    mgr,
		encryptor:    encryptor,
		businessRepo: businessRepo,
		walletRepo:   walletRepo,
		webhookURL:   webhookURL,
	}, nil
}

// Register creates a merchant, derives their Sui address, and persists
// all rows to the shared DB so the indexer can watch the address.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*Merchant, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.BankName = strings.TrimSpace(req.BankName)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)

	if req.Name == "" || req.Email == "" || req.Password == "" || req.BankName == "" || req.AccountNumber == "" {
		return nil, errors.New("name, email, password, bankName and accountNumber are required")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	businessID := slugify(req.Name)

	var m *Merchant
	err = s.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// ── 1. Duplicate guard ────────────────────────────────────────────────────
		var existing indexermodel.Business
		if err := tx.Where("business_id = ?", businessID).First(&existing).Error; err == nil {
			return fmt.Errorf("merchant %q already exists", businessID)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("lookup business: %w", err)
		}

		// ── 2. Safer Derivation index (MAX + 1) ───────────────────────────────────
		var maxIdx sql.NullInt64
		if err := tx.Table("businesses").Select("MAX(derivation_index)").Scan(&maxIdx).Error; err != nil {
			return fmt.Errorf("calculate next index: %w", err)
		}
		idx := uint32(0)
		if maxIdx.Valid {
			idx = uint32(maxIdx.Int64 + 1)
		}

		// ── 3. Derive Sui address + private key ──────────────────────────────────
		suiAddr, err := s.walletMgr.DeriveAddress("sui", idx)
		if err != nil {
			return fmt.Errorf("derive sui address: %w", err)
		}
		privKeyBytes, err := s.walletMgr.DeriveSuiPrivateKeyBytes(idx)
		if err != nil {
			return fmt.Errorf("derive sui private key: %w", err)
		}
		encryptedKey, err := s.encryptor.Encrypt(privKeyBytes)
		if err != nil {
			return fmt.Errorf("encrypt private key: %w", err)
		}

		// ── 4. Generate webhook secret ────────────────────────────────────────────
		secret, err := generateSecret()
		if err != nil {
			return fmt.Errorf("generate secret: %w", err)
		}

		// ── 5. Persist to shared DB (businesses + wallet_addresses) ───────────────
		biz := &indexermodel.Business{
			BusinessID:      businessID,
			Name:            req.Name,
			WebhookURL:      s.webhookURL,
			WebhookSecret:   secret,
			DerivationIndex: idx,
			Active:          true,
		}
		if err := tx.Save(biz).Error; err != nil {
			return fmt.Errorf("save business: %w", err)
		}

		wa := &indexermodel.WalletAddress{
			Address:    suiAddr,
			Type:       enum.NetworkTypeSui,
			Standard:   "native",
			BusinessID: businessID,
			AssetType:  "USDC",
		}
		if err := tx.Save(wa).Error; err != nil {
			return fmt.Errorf("save wallet_address: %w", err)
		}

		// ── 6. Save merchant profile ──────────────────────────────────────────────
		m = &Merchant{
			BusinessID:          businessID,
			Name:                req.Name,
			Email:               req.Email,
			BankName:            req.BankName,
			AccountNumber:       req.AccountNumber,
			SuiAddress:          suiAddr,
			EncryptedPrivateKey: encryptedKey,
			PasswordHash:        string(hashedPassword),
			Status:              "active",
		}
		if err := tx.Table("merchants").Create(m).Error; err != nil {
			return fmt.Errorf("save merchant profile: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return m, nil
}

// GetPrivateKey decrypts and returns the base64-encoded Sui private key for a merchant.
func (s *Service) GetPrivateKey(ctx context.Context, merchantID string) (string, error) {
	var encKey string
	err := s.db.QueryRowContext(ctx,
		`SELECT encrypted_private_key FROM merchants WHERE id = $1 AND deleted_at IS NULL`,
		merchantID,
	).Scan(&encKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("merchant not found")
	}
	if err != nil {
		return "", err
	}
	raw, err := s.encryptor.Decrypt(encKey)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	// Return in Sui wallet import format: base64(0x00 || seed)
	return base64.StdEncoding.EncodeToString(append([]byte{0x00}, raw...)), nil
}

func (s *Service) List(ctx context.Context) ([]*Merchant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, created_at, updated_at, business_id, name,
		       COALESCE(email,''), COALESCE(bank_name,''), COALESCE(account_number,''),
		       COALESCE(sui_address,''), status
		FROM merchants
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Merchant
	for rows.Next() {
		m := &Merchant{}
		if err := rows.Scan(
			&m.ID, &m.CreatedAt, &m.UpdatedAt, &m.BusinessID, &m.Name,
			&m.Email, &m.BankName, &m.AccountNumber, &m.SuiAddress, &m.Status,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Service) GetByID(ctx context.Context, id string) (*Merchant, error) {
	m := &Merchant{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, business_id, name,
		       COALESCE(email,''), COALESCE(bank_name,''), COALESCE(account_number,''),
		       COALESCE(sui_address,''), status
		FROM merchants
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&m.ID, &m.CreatedAt, &m.UpdatedAt, &m.BusinessID, &m.Name,
		&m.Email, &m.BankName, &m.AccountNumber, &m.SuiAddress, &m.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		return nil, errors.New("email and password are required")
	}

	var m Merchant
	err := s.gormDB.WithContext(ctx).Table("merchants").Where("email = ?", req.Email).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(m.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	token, err := s.generateToken(m.ID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &AuthResponse{
		Merchant: &m,
		Token:    token,
	}, nil
}

func (s *Service) GetStats(ctx context.Context, merchantID string) (*MerchantStats, error) {
	stats := &MerchantStats{}

	err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(amount_usdc), 0),
			COALESCE(SUM(CASE WHEN created_at::date = CURRENT_DATE THEN amount_usdc ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= NOW() - INTERVAL '7 days' THEN amount_usdc ELSE 0 END), 0),
			COUNT(*) FILTER (WHERE status NOT IN ('completed','refunded','failed')),
			COUNT(*) FILTER (WHERE status = 'failed')
		FROM deposits
		WHERE merchant_id = $1
	`, merchantID).Scan(
		&stats.USDCTotalReceived, &stats.USDCToday, &stats.USDCThisWeek,
		&stats.PendingCount, &stats.FailedCount,
	)
	if err != nil {
		return nil, fmt.Errorf("stats deposits query: %w", err)
	}

	err = s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(amount_ngn), 0),
			COALESCE(SUM(CASE WHEN created_at::date = CURRENT_DATE THEN amount_ngn ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= NOW() - INTERVAL '7 days' THEN amount_ngn ELSE 0 END), 0)
		FROM settlements
		WHERE merchant_id = $1 AND status = 'completed'
	`, merchantID).Scan(&stats.NGNTotalSettled, &stats.NGNToday, &stats.NGNThisWeek)
	if err != nil {
		return nil, fmt.Errorf("stats settlements query: %w", err)
	}

	return stats, nil
}

func (s *Service) UpdatePassword(ctx context.Context, merchantID, currentPw, newPw string) error {
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT password_hash FROM merchants WHERE id = $1 AND deleted_at IS NULL`, merchantID,
	).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("merchant not found")
	}
	if err != nil {
		return fmt.Errorf("lookup merchant: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPw)); err != nil {
		return errors.New("invalid credentials")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE merchants SET password_hash = $1, updated_at = NOW() WHERE id = $2`,
		string(newHash), merchantID,
	)
	return err
}

func (s *Service) generateToken(merchantID string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "b2b-merchant-secret-key-change-this-in-prod"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": merchantID,
		"exp": time.Now().Add(time.Hour * 72).Unix(),
		"iat": time.Now().Unix(),
	})

	return token.SignedString([]byte(secret))
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnum.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
