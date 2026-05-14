package order

import (
	"time"
)

// PendingOrder represents a payment link order in the database.
// Created when a customer enters an NGN amount on the merchant's payment page.
type PendingOrder struct {
	ID                 string    `json:"id"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	ExpiresAt          time.Time `json:"expiresAt"`
	MerchantID         string    `json:"merchantId"`
	AmountNGN          float64   `json:"amountNgn"`
	ExpectedAmountUSDC float64   `json:"expectedAmountUsdc"`
	ExchangeRate       float64   `json:"exchangeRate"`
	MerchantAddress    string    `json:"merchantAddress"`
	CustomerEmail      string    `json:"customerEmail,omitempty"`
	Status             string    `json:"status"`
}

// CreateOrderRequest is the JSON body for POST /api/merchants/{id}/orders.
type CreateOrderRequest struct {
	AmountNGN     float64 `json:"amount_ngn"`
	CustomerEmail string  `json:"customer_email"` // optional
}

// CreateOrderResponse is returned after successfully creating a pending order.
type CreateOrderResponse struct {
	PendingOrderID  string  `json:"pending_order_id"`
	AmountUSDC      float64 `json:"amount_usdc"`
	MerchantAddress string  `json:"merchant_address"`
	ExchangeRate    float64 `json:"exchange_rate"`
	ExpiresAt       string  `json:"expires_at"`
}

// OrderStatusResponse is returned by GET /api/orders/{id}/status.
type OrderStatusResponse struct {
	PendingOrderID string  `json:"pending_order_id"`
	Status         string  `json:"status"`          // pending | received | expired
	DepositStatus  *string `json:"deposit_status"`  // null until a deposit is matched
	AmountUSDC     float64 `json:"amount_usdc"`
	ExpiresAt      string  `json:"expires_at"`
}
