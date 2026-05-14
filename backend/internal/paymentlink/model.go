package paymentlink

import "time"

type PaymentLink struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	MerchantID string    `json:"merchantId"`
	AmountNGN  float64   `json:"amountNgn"`
	URL        string    `json:"url"`
	Status     string    `json:"status"` // active | used | expired
}

type CreateRequest struct {
	AmountNGN float64 `json:"amount_ngn"`
}
