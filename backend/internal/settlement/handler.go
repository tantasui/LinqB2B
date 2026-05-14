package settlement

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// List handles GET /api/merchants/{id}/settlements
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	merchantID := extractMerchantID(r.URL.Path)
	if merchantID == "" {
		http.Error(w, "missing merchant id", http.StatusBadRequest)
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, created_at, merchant_id,
		       COALESCE(deposit_id::text, ''),
		       amount_usdc, amount_ngn, exchange_rate,
		       COALESCE(bank_reference, ''), COALESCE(nomba_reference, ''),
		       status
		FROM settlements
		WHERE merchant_id = $1
		ORDER BY created_at DESC
	`, merchantID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	settlements := []*Settlement{}
	for rows.Next() {
		s := &Settlement{}
		if err := rows.Scan(
			&s.ID, &s.CreatedAt, &s.MerchantID, &s.DepositID,
			&s.AmountUSDC, &s.AmountNGN, &s.ExchangeRate,
			&s.BankReference, &s.NombaReference, &s.Status,
		); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		settlements = append(settlements, s)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settlements)
}

func extractMerchantID(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "merchants" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
