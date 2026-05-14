package order

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler handles HTTP requests for the order endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new order handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create handles POST /api/merchants/{merchantId}/orders.
// It creates a pending order after fetching the current USDC/NGN exchange rate.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract merchantId from path: /api/merchants/{id}/orders
	merchantID := extractMerchantID(r.URL.Path)
	if merchantID == "" {
		http.Error(w, "missing merchant id in path", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	// Create the pending order
	resp, err := h.svc.CreatePendingOrder(r.Context(), merchantID, req)
	if err != nil {
		errMsg := err.Error()

		// Validation errors → 400
		if strings.Contains(errMsg, "must be greater than") ||
			strings.Contains(errMsg, "must not exceed") ||
			strings.Contains(errMsg, "too small") {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		// Merchant not found → 404
		if strings.Contains(errMsg, "not found") {
			http.Error(w, "merchant not found", http.StatusNotFound)
			return
		}

		// Exchange rate unavailable → 503 with Retry-After
		if strings.Contains(errMsg, "exchange_rate_unavailable") {
			w.Header().Set("Retry-After", "30")
			http.Error(w, "exchange rate service temporarily unavailable, please retry", http.StatusServiceUnavailable)
			return
		}

		// Everything else → 500
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// GetStatus handles GET /api/orders/{pendingOrderId}/status.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// path: /api/orders/{id}/status
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// ["api", "orders", "{id}", "status"]
	var orderID string
	for i, p := range parts {
		if p == "orders" && i+1 < len(parts) {
			orderID = parts[i+1]
			break
		}
	}
	if orderID == "" {
		http.Error(w, "missing order id", http.StatusBadRequest)
		return
	}

	resp, err := h.svc.GetOrderStatus(r.Context(), orderID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "order not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// extractMerchantID gets the merchant UUID from a path like /api/merchants/{id}/orders.
func extractMerchantID(path string) string {
	// Split: ["", "api", "merchants", "{id}", "orders"]
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "merchants" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
