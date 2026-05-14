package b2b

import (
	"time"

	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/types"
)

// WebhookPayload represents the data sent to the B2B service
type WebhookPayload struct {
	EventID        string            `json:"eventId"`
	EventType      string            `json:"eventType"`
	BusinessID     string            `json:"businessId"`
	WebhookVersion string            `json:"webhookVersion"`
	SentAt         time.Time         `json:"sentAt"`
	Transaction    types.Transaction `json:"transaction"`
	Settlement     SettlementInfo    `json:"settlement"`
}

// SettlementInfo contains metadata for the fiat settlement
type SettlementInfo struct {
	Status          string `json:"status"`
	Currency        string `json:"currency"`
	EstimatedAmount string `json:"estimatedAmount"`
}

// ConfirmationState tracks the progress of a transaction towards finality
type ConfirmationState struct {
	TxHash        string    `json:"txHash"`
	NetworkID     string    `json:"networkId"`
	SeenAtBlock   uint64    `json:"seenAtBlock"`
	RequiredDepth uint8     `json:"requiredDepth"`
	ReceivedAt    time.Time `json:"receivedAt"`
}
