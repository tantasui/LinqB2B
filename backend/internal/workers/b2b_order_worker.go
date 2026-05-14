package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/fystack/b2b-merchant/internal/queue"
	amqp "github.com/rabbitmq/amqp091-go"
)

// StartB2BOrderWorker consumes from the main order queue and routes each
// delivery to the correct chain-specific treasury queue based on the chain field.
//
// Routing table:
//
//	sui    → b2b-sui-treasury-queue
//	solana → b2b-solana-treasury-queue
//	base   → b2b-base-treasury-queue
//
// Ack strategy:
//
//	success          → d.Ack(false)
//	transient error  → d.Nack(false, true)   requeue for retry
//	permanent error  → d.Nack(false, false)  route to DLQ
//
// This is a single goroutine — do not parallelize.
// Call as: go workers.StartB2BOrderWorker(orderQueue)
func StartB2BOrderWorker(q *queue.Queue) {
	log.Println("[B2B_ORDER_WORKER] Starting...")

	// Consumers own their channel — never share the publish channel.
	ch, err := q.NewConsumerChannel()
	if err != nil {
		log.Fatalf("[B2B_ORDER_WORKER] Failed to open consumer channel: %v", err)
	}
	defer ch.Close()

	// publishCh is a separate channel from the consumer channel: RabbitMQ channels
	// are not goroutine-safe, and publish + consume must not share one.
	publishCh, err := q.NewConsumerChannel()
	if err != nil {
		log.Fatalf("[B2B_ORDER_WORKER] Failed to open publish channel: %v", err)
	}
	defer publishCh.Close()

	deliveries, err := ch.Consume(
		queue.MainQueueName, // source: orders.queue
		"b2b-order-worker",  // consumer tag
		false,               // autoAck: false — manual ack only
		false,               // exclusive
		false,               // no-local
		false,               // no-wait
		nil,                 // args
	)
	if err != nil {
		log.Fatalf("[B2B_ORDER_WORKER] Failed to register consumer: %v", err)
	}

	log.Printf("[B2B_ORDER_WORKER] Listening on %s", queue.MainQueueName)

	for d := range deliveries {
		processOrderDelivery(d, publishCh)
	}

	log.Println("[B2B_ORDER_WORKER] Delivery channel closed — worker exiting")
}

// processOrderDelivery handles a single AMQP delivery: unmarshal → route → ack.
func processOrderDelivery(d amqp.Delivery, publishCh *amqp.Channel) {
	var msg queue.OrderMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		// Malformed JSON — permanent error, send to DLQ.
		log.Printf("[B2B_ORDER_WORKER] Permanent error: failed to unmarshal message: %v — sending to DLQ", err)
		_ = d.Nack(false, false)
		return
	}

	targetQueue, err := routeToChainQueue(msg.Chain)
	if err != nil {
		// Unrecognised chain — permanent error, no point retrying.
		log.Printf("[B2B_ORDER_WORKER] Permanent error: unknown chain %q tx_hash=%s — sending to DLQ",
			msg.Chain, msg.TxHash)
		_ = d.Nack(false, false)
		return
	}

	log.Printf("[B2B_ORDER_WORKER] Routing tx_hash=%s chain=%s → %s",
		msg.TxHash, msg.Chain, targetQueue)

	if err := publishToChainQueue(publishCh, targetQueue, d.Body); err != nil {
		// Publish failed — transient, requeue so we can retry.
		log.Printf("[B2B_ORDER_WORKER] Transient error: failed to publish to %s tx_hash=%s: %v — requeueing",
			targetQueue, msg.TxHash, err)
		_ = d.Nack(false, true)
		return
	}

	log.Printf("[B2B_ORDER_WORKER] Successfully routed tx_hash=%s chain=%s → %s",
		msg.TxHash, msg.Chain, targetQueue)
	_ = d.Ack(false)
}

// routeToChainQueue maps a ChainType to its treasury queue name.
// Returns an error for any unrecognised chain so the caller can DLQ the message.
func routeToChainQueue(chain queue.ChainType) (string, error) {
	switch chain {
	case queue.ChainSui:
		return queue.QueueSuiTreasury, nil
	case queue.ChainSolana:
		return queue.QueueSolanaTreasury, nil
	case queue.ChainBase:
		return queue.QueueBaseTreasury, nil
	default:
		return "", fmt.Errorf("unrecognised chain: %q", chain)
	}
}

// publishToChainQueue publishes the raw message body as a persistent JSON payload
// to the named chain treasury queue using the default exchange.
func publishToChainQueue(ch *amqp.Channel, queueName string, body []byte) error {
	return ch.PublishWithContext(
		context.Background(),
		"",        // default exchange — routes directly by queue name
		queueName, // routing key = queue name when using default exchange
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}
