package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	mainExchange = "orders"
	mainQueue    = MainQueueName
	routingKey   = "order"
	dlxExchange  = "orders.dlx"
	dlQueue      = "orders.dead_letter"
)

// Publisher is the interface the webhook handler depends on to enqueue orders.
// Using the interface keeps the handler testable without a live broker.
type Publisher interface {
	Enqueue(ctx context.Context, msg OrderMessage) error
	PublishB2BOrder(ctx context.Context, msg OrderMessage) error
}

// Queue manages order messages via RabbitMQ.
//
// Topology:
//   - orders (direct exchange) → orders.queue (durable, 1-hour TTL)
//   - orders.dlx (fanout DLX)  → orders.dead_letter
//
// Messages that expire or are Nacked are automatically routed to the DLQ by the
// broker; no background sweep goroutine is needed.
//
// pubCh and consCh are intentionally separate channels: RabbitMQ channels are
// not goroutine-safe, and mixing publish/consume on one channel breaks when a
// consumer registers via basic.consume (which blocks the channel).
type Queue struct {
	conn   *amqp.Connection
	pubCh  *amqp.Channel // used by all publishers; guarded by pubMu
	pubMu  sync.Mutex
	consCh *amqp.Channel // used by Dequeue, Ack, Nack
}

// New dials the RabbitMQ server at url, opens two dedicated channels, and
// declares the full exchange/queue topology. Call Close when done.
func New(url string) (*Queue, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("queue: dial %s: %w", url, err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("queue: open publish channel: %w", err)
	}

	consCh, err := conn.Channel()
	if err != nil {
		pubCh.Close()
		conn.Close()
		return nil, fmt.Errorf("queue: open consumer channel: %w", err)
	}

	q := &Queue{conn: conn, pubCh: pubCh, consCh: consCh}
	if err := q.declareTopology(); err != nil {
		pubCh.Close()
		consCh.Close()
		conn.Close()
		return nil, err
	}
	return q, nil
}

// declareTopology idempotently creates exchanges and queues.
// Safe to call multiple times; existing resources are left unchanged.
func (q *Queue) declareTopology() error {
	// ── Dead-letter exchange & queue ─────────────────────────────────────────
	if err := q.pubCh.ExchangeDeclare(dlxExchange, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("queue: declare dlx: %w", err)
	}
	if _, err := q.pubCh.QueueDeclare(dlQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("queue: declare dead-letter queue: %w", err)
	}
	if err := q.pubCh.QueueBind(dlQueue, "", dlxExchange, false, nil); err != nil {
		return fmt.Errorf("queue: bind dead-letter queue: %w", err)
	}

	// ── Main exchange ─────────────────────────────────────────────────────────
	if err := q.pubCh.ExchangeDeclare(mainExchange, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("queue: declare exchange: %w", err)
	}

	// ── Main queue with TTL → DLX ─────────────────────────────────────────────
	args := amqp.Table{
		"x-message-ttl":          int32(DefaultTTL / time.Millisecond),
		"x-dead-letter-exchange": dlxExchange,
	}
	if _, err := q.pubCh.QueueDeclare(mainQueue, true, false, false, false, args); err != nil {
		return fmt.Errorf("queue: declare main queue: %w", err)
	}
	if err := q.pubCh.QueueBind(mainQueue, routingKey, mainExchange, false, nil); err != nil {
		return fmt.Errorf("queue: bind main queue: %w", err)
	}

	// ── Chain treasury queues + DLQs ─────────────────────────────────────────
	if err := declareAllQueues(q.pubCh); err != nil {
		return err
	}
	return nil
}

// Close releases both AMQP channels and the connection.
func (q *Queue) Close() error {
	q.pubCh.Close()
	q.consCh.Close()
	if err := q.conn.Close(); err != nil {
		return fmt.Errorf("queue close connection: %w", err)
	}
	return nil
}

// NewConsumerChannel opens a fresh AMQP channel on the existing connection.
// Consumers must own their channel — never consume on the shared pubCh.
func (q *Queue) NewConsumerChannel() (*amqp.Channel, error) {
	return q.conn.Channel()
}

// publish serialises access to pubCh. amqp.Channel is not goroutine-safe;
// all publisher paths (Enqueue and chain treasury publishers) must call this.
func (q *Queue) publish(exchange, key string, pub amqp.Publishing) error {
	q.pubMu.Lock()
	defer q.pubMu.Unlock()
	return q.pubCh.Publish(exchange, key, false, false, pub)
}

// Enqueue publishes an order message as a persistent JSON payload.
// The broker guarantees durability across restarts.
func (q *Queue) Enqueue(ctx context.Context, msg OrderMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("queue enqueue: marshal: %w", err)
	}
	err = q.publish(mainExchange, routingKey, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    msg.TxHash,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
	if err != nil {
		log.Printf("queue: enqueue error tx=%s: %v", msg.TxHash, err)
		return fmt.Errorf("queue enqueue: publish: %w", err)
	}
	log.Printf("queue: enqueued order_id=%s merchant=%s chain=%s tx=%s amount=%.6f",
		msg.OrderID, msg.MerchantID, msg.Chain, msg.TxHash, msg.AmountUSDC)
	return nil
}

// Dequeue fetches one message without auto-acking (manual acknowledgement mode).
// The caller must call Ack(msg.DeliveryTag) on success or Nack(msg.DeliveryTag)
// on failure. Returns (nil, nil) when the queue is empty.
func (q *Queue) Dequeue(ctx context.Context) (*OrderMessage, error) {
	d, ok, err := q.consCh.Get(mainQueue, false)
	if err != nil {
		return nil, fmt.Errorf("queue dequeue: %w", err)
	}
	if !ok {
		return nil, nil
	}
	var msg OrderMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		// Malformed payload — reject without requeue so it lands in the DLQ.
		_ = d.Nack(false, false)
		return nil, fmt.Errorf("queue dequeue: unmarshal: %w", err)
	}
	msg.DeliveryTag = d.DeliveryTag
	log.Printf("queue: dequeued order_id=%s chain=%s tx=%s delivery_tag=%d",
		msg.OrderID, msg.Chain, msg.TxHash, msg.DeliveryTag)
	return &msg, nil
}

// Ack acknowledges successful processing of the message with the given deliveryTag.
func (q *Queue) Ack(ctx context.Context, deliveryTag uint64) error {
	if err := q.consCh.Ack(deliveryTag, false); err != nil {
		log.Printf("queue: ack error delivery_tag=%d: %v", deliveryTag, err)
		return fmt.Errorf("queue ack: %w", err)
	}
	log.Printf("queue: acked delivery_tag=%d", deliveryTag)
	return nil
}

// Nack rejects the message (requeue=false), routing it to the dead-letter queue.
// The reason is logged locally; RabbitMQ appends x-death headers to the DLQ message.
func (q *Queue) Nack(ctx context.Context, deliveryTag uint64, reason string) error {
	log.Printf("queue: nacking delivery_tag=%d reason=%q", deliveryTag, reason)
	if err := q.consCh.Nack(deliveryTag, false, false); err != nil {
		return fmt.Errorf("queue nack: %w", err)
	}
	return nil
}
