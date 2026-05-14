package paymentlink

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// List handles GET /api/merchants/{id}/payment-links
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
		SELECT id, created_at, merchant_id, amount_ngn, url, status
		FROM payment_links
		WHERE merchant_id = $1
		ORDER BY created_at DESC
	`, merchantID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	links := []*PaymentLink{}
	for rows.Next() {
		pl := &PaymentLink{}
		if err := rows.Scan(&pl.ID, &pl.CreatedAt, &pl.MerchantID, &pl.AmountNGN, &pl.URL, &pl.Status); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		links = append(links, pl)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

// Create handles POST /api/merchants/{id}/payment-links
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	merchantID := extractMerchantID(r.URL.Path)
	if merchantID == "" {
		http.Error(w, "missing merchant id", http.StatusBadRequest)
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.AmountNGN <= 0 {
		http.Error(w, "amount_ngn must be greater than 0", http.StatusBadRequest)
		return
	}
	if req.AmountNGN > 10_000_000 {
		http.Error(w, "amount_ngn must not exceed 10,000,000", http.StatusBadRequest)
		return
	}

	// Verify merchant exists
	var exists bool
	err := h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM merchants WHERE id = $1 AND deleted_at IS NULL)`,
		merchantID,
	).Scan(&exists)
	if err != nil || !exists {
		http.Error(w, "merchant not found", http.StatusNotFound)
		return
	}

	baseURL := os.Getenv("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5174"
	}
	linkURL := fmt.Sprintf("%s/pay/%s?amount=%.0f", baseURL, merchantID, req.AmountNGN)

	pl := &PaymentLink{}
	err = h.db.QueryRowContext(r.Context(), `
		INSERT INTO payment_links (merchant_id, amount_ngn, url, status)
		VALUES ($1, $2, $3, 'active')
		RETURNING id, created_at, merchant_id, amount_ngn, url, status
	`, merchantID, req.AmountNGN, linkURL).Scan(
		&pl.ID, &pl.CreatedAt, &pl.MerchantID, &pl.AmountNGN, &pl.URL, &pl.Status,
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pl)
}

// Delete handles DELETE /api/merchants/{id}/payment-links/{linkId}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// path: /api/merchants/{merchantId}/payment-links/{linkId}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	var merchantID, linkID string
	for i, p := range parts {
		if p == "merchants" && i+1 < len(parts) {
			merchantID = parts[i+1]
		}
		if p == "payment-links" && i+1 < len(parts) {
			linkID = parts[i+1]
		}
	}
	if merchantID == "" || linkID == "" {
		http.Error(w, "missing merchant id or link id", http.StatusBadRequest)
		return
	}

	res, err := h.db.ExecContext(r.Context(),
		`DELETE FROM payment_links WHERE id = $1 AND merchant_id = $2`,
		linkID, merchantID,
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
