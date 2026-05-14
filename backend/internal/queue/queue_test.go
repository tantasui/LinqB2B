package queue_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fystack/b2b-merchant/internal/queue"
)

// ── Model unit tests ─────────────────────────────────────────────────────────

func TestParseChain(t *testing.T) {
	tests := []struct {
		input string
		want  queue.ChainType
		ok    bool
	}{
		// Sui variants
		{"sui", queue.ChainSui, true},
		{"Sui", queue.ChainSui, true},
		{"SUI", queue.ChainSui, true},
		{"sui:mainnet", queue.ChainSui, true},
		{"sui:testnet", queue.ChainSui, true},
		{"sui:devnet", queue.ChainSui, true},
		// Solana variants
		{"solana", queue.ChainSolana, true},
		{"sol", queue.ChainSolana, true},
		{"SOL", queue.ChainSolana, true},
		{"Solana", queue.ChainSolana, true},
		{"solana:mainnet", queue.ChainSolana, true},
		{"solana:mainnet-beta", queue.ChainSolana, true},
		{"solana:devnet", queue.ChainSolana, true},
		{"solana:testnet", queue.ChainSolana, true},
		// Base variants
		{"base", queue.ChainBase, true},
		{"Base", queue.ChainBase, true},
		{"BASE", queue.ChainBase, true},
		{"base:mainnet", queue.ChainBase, true},
		{"base:mainnet-beta", queue.ChainBase, true},
		{"base:sepolia", queue.ChainBase, true},
		// Unknown / unsupported
		{"ethereum", "", false},
		{"eth", "", false},
		{"bitcoin", "", false},
		{"unknown", "", false},
		{"", "", false},
		{"  ", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, ok := queue.ParseChain(tc.input)
			if ok != tc.ok {
				t.Errorf("ParseChain(%q) ok=%v, want %v", tc.input, ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("ParseChain(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestChainTypeValues(t *testing.T) {
	if queue.ChainSui != "sui" {
		t.Errorf("ChainSui = %q, want %q", queue.ChainSui, "sui")
	}
	if queue.ChainSolana != "solana" {
		t.Errorf("ChainSolana = %q, want %q", queue.ChainSolana, "solana")
	}
	if queue.ChainBase != "base" {
		t.Errorf("ChainBase = %q, want %q", queue.ChainBase, "base")
	}
}

func TestStatusValues(t *testing.T) {
	if queue.StatusPending != "pending" {
		t.Errorf("StatusPending = %q, want %q", queue.StatusPending, "pending")
	}
	if queue.StatusProcessing != "processing" {
		t.Errorf("StatusProcessing = %q, want %q", queue.StatusProcessing, "processing")
	}
	if queue.StatusRouted != "routed" {
		t.Errorf("StatusRouted = %q, want %q", queue.StatusRouted, "routed")
	}
	if queue.StatusDeadLetter != "dead_letter" {
		t.Errorf("StatusDeadLetter = %q, want %q", queue.StatusDeadLetter, "dead_letter")
	}
}

func TestDefaultTTL(t *testing.T) {
	if queue.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %s, want 1h", queue.DefaultTTL)
	}
}

func TestOrderMessageFields(t *testing.T) {
	now := time.Now()
	msg := queue.OrderMessage{
		OrderID:        "order-1",
		MerchantID:     "merchant-1",
		Chain:          queue.ChainSui,
		AmountUSDC:     100.5,
		TxHash:         "0xabc",
		Timestamp:      now,
		PendingOrderID: "event-1",
	}
	if msg.Chain != queue.ChainSui {
		t.Errorf("unexpected chain: %s", msg.Chain)
	}
	if msg.AmountUSDC != 100.5 {
		t.Errorf("unexpected amount: %f", msg.AmountUSDC)
	}
	// DeliveryTag is zero until Dequeue populates it.
	if msg.DeliveryTag != 0 {
		t.Errorf("DeliveryTag should be zero before Dequeue, got %d", msg.DeliveryTag)
	}
}

// ── RabbitMQ integration tests ────────────────────────────────────────────────
// These tests require a live RabbitMQ instance.
// Set RABBITMQ_URL (e.g. amqp://guest:guest@localhost:5672/) to run them;
// they are skipped automatically when the variable is absent.

func rabbitMQURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		t.Skip("RABBITMQ_URL not set — skipping RabbitMQ integration tests")
	}
	return url
}

// newTestQueue opens a fresh Queue and registers t.Cleanup to close it.
func newTestQueue(t *testing.T) *queue.Queue {
	t.Helper()
	q, err := queue.New(rabbitMQURL(t))
	if err != nil {
		t.Fatalf("queue.New: %v", err)
	}
	t.Cleanup(func() { q.Close() })
	return q
}

// drainMain pulls and acks all messages currently in the main queue so each
// test starts from a clean state.
func drainMain(t *testing.T, q *queue.Queue) {
	t.Helper()
	ctx := context.Background()
	for {
		msg, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("drain Dequeue: %v", err)
		}
		if msg == nil {
			return
		}
		if err := q.Ack(ctx, msg.DeliveryTag); err != nil {
			t.Fatalf("drain Ack: %v", err)
		}
	}
}

func TestQueue_EnqueueDequeue(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()
	drainMain(t, q)

	msg := queue.OrderMessage{
		OrderID:        "order-enqueue-1",
		MerchantID:     "merchant-enqueue-1",
		Chain:          queue.ChainSui,
		AmountUSDC:     50.5,
		TxHash:         "0xenqueue1",
		Timestamp:      time.Now().UTC().Truncate(time.Millisecond),
		PendingOrderID: "event-enqueue-1",
	}

	if err := q.Enqueue(ctx, msg); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got == nil {
		t.Fatal("Dequeue returned nil; expected a message")
	}
	if got.OrderID != msg.OrderID {
		t.Errorf("OrderID = %q, want %q", got.OrderID, msg.OrderID)
	}
	if got.MerchantID != msg.MerchantID {
		t.Errorf("MerchantID = %q, want %q", got.MerchantID, msg.MerchantID)
	}
	if got.Chain != msg.Chain {
		t.Errorf("Chain = %q, want %q", got.Chain, msg.Chain)
	}
	if got.AmountUSDC != msg.AmountUSDC {
		t.Errorf("AmountUSDC = %f, want %f", got.AmountUSDC, msg.AmountUSDC)
	}
	if got.TxHash != msg.TxHash {
		t.Errorf("TxHash = %q, want %q", got.TxHash, msg.TxHash)
	}
	if got.PendingOrderID != msg.PendingOrderID {
		t.Errorf("PendingOrderID = %q, want %q", got.PendingOrderID, msg.PendingOrderID)
	}
	if got.DeliveryTag == 0 {
		t.Error("DeliveryTag should be non-zero after Dequeue")
	}

	if err := q.Ack(ctx, got.DeliveryTag); err != nil {
		t.Fatalf("Ack: %v", err)
	}
}

func TestQueue_DequeueEmpty(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()
	drainMain(t, q)

	msg, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue on empty queue: %v", err)
	}
	if msg != nil {
		t.Errorf("expected nil on empty queue, got %+v", msg)
		q.Ack(ctx, msg.DeliveryTag)
	}
}

func TestQueue_Ack_RemovesMessage(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()
	drainMain(t, q)

	if err := q.Enqueue(ctx, queue.OrderMessage{
		OrderID: "order-ack-1", TxHash: "0xack1", Chain: queue.ChainBase,
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	got, err := q.Dequeue(ctx)
	if err != nil || got == nil {
		t.Fatalf("Dequeue: err=%v msg=%v", err, got)
	}
	if err := q.Ack(ctx, got.DeliveryTag); err != nil {
		t.Fatalf("Ack: %v", err)
	}

	// After ack the message must not reappear.
	after, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue after ack: %v", err)
	}
	if after != nil {
		t.Errorf("message reappeared after Ack; got %+v", after)
		q.Ack(ctx, after.DeliveryTag)
	}
}

func TestQueue_Nack_RoutesToDeadLetter(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()
	drainMain(t, q)

	if err := q.Enqueue(ctx, queue.OrderMessage{
		OrderID: "order-nack-1", TxHash: "0xnack1", Chain: queue.ChainSolana,
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	got, err := q.Dequeue(ctx)
	if err != nil || got == nil {
		t.Fatalf("Dequeue: err=%v msg=%v", err, got)
	}
	if err := q.Nack(ctx, got.DeliveryTag, "test rejection"); err != nil {
		t.Fatalf("Nack: %v", err)
	}

	// Nacked with requeue=false → message goes to DLQ, not back to main queue.
	after, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue after nack: %v", err)
	}
	if after != nil && after.TxHash == got.TxHash {
		t.Error("nacked message reappeared in main queue; expected it in the dead-letter queue")
		q.Ack(ctx, after.DeliveryTag)
	} else if after != nil {
		q.Ack(ctx, after.DeliveryTag)
	}
}

func TestQueue_MultipleMessages_FIFO(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()
	drainMain(t, q)

	wantOrder := []string{"0xfifo1", "0xfifo2", "0xfifo3"}
	for i, txHash := range wantOrder {
		if err := q.Enqueue(ctx, queue.OrderMessage{
			OrderID: fmt.Sprintf("order-fifo-%d", i),
			TxHash:  txHash,
			Chain:   queue.ChainSui,
		}); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	for _, wantTx := range wantOrder {
		msg, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("Dequeue: %v", err)
		}
		if msg == nil {
			t.Fatal("Dequeue returned nil; expected a message")
		}
		if msg.TxHash != wantTx {
			t.Errorf("FIFO order broken: got TxHash=%q, want %q", msg.TxHash, wantTx)
		}
		if err := q.Ack(ctx, msg.DeliveryTag); err != nil {
			t.Fatalf("Ack: %v", err)
		}
	}
}

func TestQueue_PublisherInterface(t *testing.T) {
	q := newTestQueue(t)
	// *Queue must satisfy the Publisher interface at compile time.
	var _ queue.Publisher = q
}
