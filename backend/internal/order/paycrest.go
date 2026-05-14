package order

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// FetchExchangeRate returns the current USDC/NGN rate from the Paycrest API.
// It retries up to 3 times with exponential backoff on failure.
func FetchExchangeRate() (float64, error) {
	var lastErr error
	delays := []time.Duration{0, time.Second, 2 * time.Second}

	for i, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}
		rate, err := fetchPaycrestRate()
		if err == nil {
			log.Printf("[PAYCREST_RATE] USDC/NGN rate: %.4f", rate)
			return rate, nil
		}
		lastErr = err
		log.Printf("[PAYCREST_RATE] attempt %d failed: %v", i+1, err)
	}
	return 0, fmt.Errorf("all 3 exchange rate attempts failed: %w", lastErr)
}

type paycrestRateResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    string `json:"data"` // rate as a string e.g. "1354.86"
}

func fetchPaycrestRate() (float64, error) {
	const url = "https://api.paycrest.io/v1/rates/USDC/1/NGN"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("paycrest GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("paycrest returned status %d", resp.StatusCode)
	}

	var body paycrestRateResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("paycrest decode: %w", err)
	}
	if body.Status != "success" || body.Data == "" {
		return 0, fmt.Errorf("paycrest unexpected response: status=%s data=%s", body.Status, body.Data)
	}

	rate, err := strconv.ParseFloat(body.Data, 64)
	if err != nil {
		return 0, fmt.Errorf("paycrest parse rate %q: %w", body.Data, err)
	}
	if rate <= 0 {
		return 0, fmt.Errorf("paycrest returned non-positive rate: %f", rate)
	}
	return rate, nil
}
