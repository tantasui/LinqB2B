package settlement

import "time"

type Settlement struct {
	ID             string    `json:"id"`
	CreatedAt      time.Time `json:"createdAt"`
	MerchantID     string    `json:"merchantId"`
	DepositID      string    `json:"depositId,omitempty"`
	AmountUSDC     float64   `json:"amountUsdc"`
	AmountNGN      float64   `json:"amountNgn"`
	ExchangeRate   float64   `json:"exchangeRate"`
	BankReference  string    `json:"bankReference"`
	NombaReference string    `json:"nombaReference"`
	Status         string    `json:"status"`
}
