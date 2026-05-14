package queue

import (
	"strings"
	"time"
)

// ChainType identifies the blockchain network for a queued order.
type ChainType string

const (
	ChainSui    ChainType = "sui"
	ChainSolana ChainType = "solana"
	ChainBase   ChainType = "base"
)

// Status documents the RabbitMQ-managed lifecycle of an order message.
// These states are tracked by the broker; they are not stored in the message payload.
type Status string

const (
	StatusPending    Status = "pending"    // waiting in the queue
	StatusProcessing Status = "processing" // dequeued, not yet acked
	StatusRouted     Status = "routed"     // acked (successfully processed)
	StatusDeadLetter Status = "dead_letter" // nacked or TTL-expired → DLQ
)

// DefaultTTL is the message time-to-live.
// RabbitMQ enforces this via x-message-ttl on the main queue; expired messages
// are automatically routed to the dead-letter queue without any sweep job.
const DefaultTTL = time.Hour

const (
	MainQueueName       = "b2b-order-queue"
	QueueSuiTreasury    = "b2b-sui-treasury-queue"
	QueueSolanaTreasury = "b2b-solana-treasury-queue"
	QueueBaseTreasury   = "b2b-base-treasury-queue"

	QueueSuiDLQ    = "b2b-sui-dlq"
	QueueSolanaDLQ = "b2b-solana-dlq"
	QueueBaseDLQ   = "b2b-base-dlq"
)

// TreasurySweepMessage is published to a chain treasury queue to instruct
// the treasury worker to sweep USDC from the merchant deposit wallet.
type TreasurySweepMessage struct {
	OrderID            string    `json:"order_id"`
	Chain              ChainType `json:"chain"`
	AmountUSDC         float64   `json:"amount_usdc"`
	WalletAddress      string    `json:"wallet_address"`
	EncryptedWalletKey string    `json:"encrypted_wallet_key"`
}

// DLQMessage is published to a chain dead-letter queue when a sweep cannot
// be completed after retries.
type DLQMessage struct {
	OrderID string    `json:"order_id"`
	Chain   ChainType `json:"chain"`
	Reason  string    `json:"reason"`
}

// OrderMessage is a deposit event published to the order queue.
// DeliveryTag is populated by Dequeue and must be passed back to Ack or Nack;
// it is not serialised as part of the JSON payload.
type OrderMessage struct {
	OrderID        string    `json:"order_id"`
	MerchantID     string    `json:"merchant_id"`
	Chain          ChainType `json:"chain"`
	AmountUSDC     float64   `json:"amount_usdc"`
	TxHash         string    `json:"tx_hash"`
	Timestamp      time.Time `json:"timestamp"`
	PendingOrderID string    `json:"pending_order_id"`

	DeliveryTag uint64 `json:"-"` // AMQP delivery tag; set by Dequeue, not serialised
}

// ParseChain maps a network identifier string (case-insensitive) to a
// ChainType. Returns ("", false) for unrecognised identifiers.
func ParseChain(networkID string) (ChainType, bool) {
	switch strings.ToLower(strings.TrimSpace(networkID)) {
	case "sui", "sui:mainnet", "sui:testnet", "sui:devnet":
		return ChainSui, true
	case "solana", "sol",
		"solana:mainnet", "solana:mainnet-beta",
		"solana:devnet", "solana:testnet":
		return ChainSolana, true
	case "base", "base:mainnet", "base:mainnet-beta", "base:sepolia":
		return ChainBase, true
	default:
		return "", false
	}
}
