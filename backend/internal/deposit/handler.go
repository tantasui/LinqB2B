package deposit

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type Deposit struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	MerchantID string    `json:"merchantId"`
	BusinessID string    `json:"businessId"`
	TxHash     string    `json:"txHash"`
	AmountRaw  string    `json:"amountRaw"`
	AmountUSDC float64   `json:"amountUsdc"`
	SuiAddress string    `json:"suiAddress"`
	Status     string    `json:"status"`
	// MerchantName is joined from the merchants table for display.
	MerchantName string `json:"merchantName"`
}

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// List handles GET /api/deposits — returns all deposits across all merchants,
// newest first, joined with merchant name.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	deposits, err := h.list(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if deposits == nil {
		deposits = []*Deposit{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deposits)
}

// ListByMerchant handles GET /api/merchants/{id}/deposits.
func (h *Handler) ListByMerchant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// path: /api/merchants/{id}/deposits
	parts := strings.Split(r.URL.Path, "/")
	// ["", "api", "merchants", "{id}", "deposits"]
	var merchantID string
	for i, p := range parts {
		if p == "merchants" && i+1 < len(parts) {
			merchantID = parts[i+1]
			break
		}
	}
	if merchantID == "" {
		http.Error(w, "missing merchant id", http.StatusBadRequest)
		return
	}

	deposits, err := h.listByMerchant(r.Context(), merchantID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if deposits == nil {
		deposits = []*Deposit{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deposits)
}

func (h *Handler) list(ctx context.Context) ([]*Deposit, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT d.id, d.created_at, d.updated_at,
		       COALESCE(d.merchant_id::text, ''), d.business_id,
		       d.tx_hash, d.amount_raw, COALESCE(d.amount_usdc, 0),
		       d.sui_address, d.status,
		       COALESCE(m.name, d.business_id)
		FROM deposits d
		LEFT JOIN merchants m ON m.id = d.merchant_id
		ORDER BY d.created_at DESC
		LIMIT 200
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeposits(rows)
}

func (h *Handler) listByMerchant(ctx context.Context, merchantID string) ([]*Deposit, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT d.id, d.created_at, d.updated_at,
		       COALESCE(d.merchant_id::text, ''), d.business_id,
		       d.tx_hash, d.amount_raw, COALESCE(d.amount_usdc, 0),
		       d.sui_address, d.status,
		       COALESCE(m.name, d.business_id)
		FROM deposits d
		LEFT JOIN merchants m ON m.id = d.merchant_id
		WHERE d.merchant_id = $1
		ORDER BY d.created_at DESC
	`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeposits(rows)
}

func scanDeposits(rows *sql.Rows) ([]*Deposit, error) {
	var out []*Deposit
	for rows.Next() {
		d := &Deposit{}
		if err := rows.Scan(
			&d.ID, &d.CreatedAt, &d.UpdatedAt,
			&d.MerchantID, &d.BusinessID,
			&d.TxHash, &d.AmountRaw, &d.AmountUSDC,
			&d.SuiAddress, &d.Status,
			&d.MerchantName,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
