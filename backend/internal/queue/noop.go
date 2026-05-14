package queue

import (
	"context"
	"log"
)

// NoopPublisher satisfies the Publisher interface but discards all messages.
// Used when RabbitMQ is unavailable so the HTTP API can still serve requests.
type NoopPublisher struct{}

func (NoopPublisher) Enqueue(ctx context.Context, msg OrderMessage) error {
	log.Printf("queue [noop]: dropped order_id=%s tx=%s (no broker)", msg.OrderID, msg.TxHash)
	return nil
}

func (NoopPublisher) PublishB2BOrder(ctx context.Context, msg OrderMessage) error {
	log.Printf("queue [noop]: dropped b2b order_id=%s tx=%s (no broker)", msg.OrderID, msg.TxHash)
	return nil
}
