package onboarding

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/enum"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/logger"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/model"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/repository"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/wallet"
)

// OnboardRequest is the JSON body expected at POST /admin/onboard.
type OnboardRequest struct {
	// BusinessID is a unique slug for the client (e.g. "acme_corp"). Immutable.
	BusinessID string `json:"businessId"`
	// Name is the human-readable display name.
	Name string `json:"name"`
	// WebhookURL is the HTTPS endpoint we deliver payment events to.
	WebhookURL string `json:"webhookUrl"`
	// WebhookSecret is the HMAC-SHA256 signing secret shared with the business.
	// If omitted a 32-byte random secret is generated and returned once.
	WebhookSecret string `json:"webhookSecret,omitempty"`
}

// OnboardResponse is returned after successful onboarding.
type OnboardResponse struct {
	BusinessID      string            `json:"businessId"`
	DerivationIndex uint32            `json:"derivationIndex"`
	Addresses       map[string]string `json:"addresses"`
	// WebhookSecret is included in the creation response only — store it safely,
	// it will not be returned again.
	WebhookSecret string    `json:"webhookSecret"`
	CreatedAt     time.Time `json:"createdAt"`
}

// supportedNetworks lists the chains for which we derive addresses at onboarding.
var supportedNetworks = []struct {
	key     string
	netType enum.NetworkType
}{
	{"sui", enum.NetworkTypeSui},
	{"solana", enum.NetworkTypeSol},
	{"ethereum", enum.NetworkTypeEVM},
}

// OnboardingService wires together wallet derivation and DB persistence.
type OnboardingService struct {
	walletMgr    *wallet.WalletManager
	businessRepo repository.Repository[model.Business]
	walletRepo   repository.Repository[model.WalletAddress]
}

func NewOnboardingService(
	mnemonic string,
	businessRepo repository.Repository[model.Business],
	walletRepo repository.Repository[model.WalletAddress],
) (*OnboardingService, error) {
	mgr, err := wallet.NewWalletManager(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("wallet manager init: %w", err)
	}
	return &OnboardingService{
		walletMgr:    mgr,
		businessRepo: businessRepo,
		walletRepo:   walletRepo,
	}, nil
}

// HandleOnboard handles POST /admin/onboard.
//
// Flow:
//  1. Validate the request fields.
//  2. Guard against duplicate businessId.
//  3. Claim the next derivation index (COUNT existing + unique DB constraint).
//  4. Derive wallet addresses for all supported networks.
//  5. Persist Business and WalletAddress rows.
//  6. Return the full onboarding response.
func (s *OnboardingService) HandleOnboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OnboardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	req.BusinessID = strings.TrimSpace(req.BusinessID)
	req.Name = strings.TrimSpace(req.Name)
	req.WebhookURL = strings.TrimSpace(req.WebhookURL)

	if req.BusinessID == "" || req.Name == "" || req.WebhookURL == "" {
		http.Error(w, "businessId, name, and webhookUrl are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// ── 1. Guard against duplicate businessId ─────────────────────────────────
	existing, err := s.businessRepo.FindOne(ctx, repository.FindOptions{
		Where: repository.WhereType{"business_id": req.BusinessID},
	})
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		logger.Error("onboard: lookup failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, fmt.Sprintf("business %q already exists", req.BusinessID), http.StatusConflict)
		return
	}

	// ── 2. Claim derivation index ─────────────────────────────────────────────
	// COUNT gives a safe monotonic index for a single-writer setup.
	// The unique index on derivation_index in the DB catches any concurrent race.
	count, err := s.businessRepo.Count(ctx, repository.FindOptions{})
	if err != nil {
		logger.Error("onboard: count businesses failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	derivationIndex := uint32(count)

	// ── 3. Derive wallet addresses ────────────────────────────────────────────
	addresses := make(map[string]string, len(supportedNetworks))
	for _, net := range supportedNetworks {
		addr, err := s.walletMgr.DeriveAddress(net.key, derivationIndex)
		if err != nil {
			logger.Error("onboard: derivation failed", "network", net.key, "err", err)
			http.Error(w, fmt.Sprintf("address derivation failed for %s", net.key), http.StatusInternalServerError)
			return
		}
		addresses[net.key] = addr
	}

	// ── 4. Persist Business ───────────────────────────────────────────────────
	secret := req.WebhookSecret
	if secret == "" {
		secret, err = generateWebhookSecret()
		if err != nil {
			logger.Error("onboard: generate secret failed", "err", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	biz := &model.Business{
		BusinessID:      req.BusinessID,
		Name:            req.Name,
		WebhookURL:      req.WebhookURL,
		WebhookSecret:   secret,
		DerivationIndex: derivationIndex,
		Active:          true,
	}
	if err := s.businessRepo.Save(ctx, biz); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			http.Error(w, fmt.Sprintf("business %q already exists (race)", req.BusinessID), http.StatusConflict)
			return
		}
		logger.Error("onboard: save business failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// ── 5. Persist WalletAddresses ────────────────────────────────────────────
	for _, net := range supportedNetworks {
		wa := &model.WalletAddress{
			Address:    addresses[net.key],
			Type:       net.netType,
			BusinessID: req.BusinessID,
			AssetType:  defaultAssets(net.key),
		}
		if err := s.walletRepo.Save(ctx, wa); err != nil {
			// Log and continue — the business row exists so a manual re-run can
			// re-derive and re-save any missing address rows.
			logger.Error("onboard: save wallet address failed",
				"network", net.key,
				"business_id", req.BusinessID,
				"err", err,
			)
		}
	}

	// ── 6. Respond ────────────────────────────────────────────────────────────
	resp := OnboardResponse{
		BusinessID:      req.BusinessID,
		DerivationIndex: derivationIndex,
		Addresses:       addresses,
		WebhookSecret:   secret,
		CreatedAt:       biz.CreatedAt,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error("onboard: encode response failed", "err", err)
	}
}

// defaultAssets returns the comma-separated list of assets we monitor per network.
func defaultAssets(network string) string {
	switch network {
	case "sui":
		return "USDC"
	case "solana":
		return "USDC,USDT"
	case "ethereum", "base":
		return "USDC,USDT"
	default:
		return "USDC"
	}
}

// generateWebhookSecret creates a cryptographically random 32-byte hex secret.
func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}
