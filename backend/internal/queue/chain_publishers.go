package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PublishB2BOrder publishes a deposit event to the main b2b-order-queue.
// This is the triple-write RabbitMQ step; called after Postgres + Redis succeed.
func (q *Queue) PublishB2BOrder(ctx context.Context, msg OrderMessage) error {
	return q.Enqueue(ctx, msg)
}

// PublishB2BSuiTreasury enqueues a sweep instruction to the Sui treasury queue.
func PublishB2BSuiTreasury(q *Queue, msg TreasurySweepMessage) error {
	return publishToChainTreasuryQueue(q, QueueSuiTreasury, msg)
}

// PublishB2BSolanaTreasury enqueues a sweep instruction to the Solana treasury queue.
func PublishB2BSolanaTreasury(q *Queue, msg TreasurySweepMessage) error {
	return publishToChainTreasuryQueue(q, QueueSolanaTreasury, msg)
}

// PublishB2BBaseTreasury enqueues a sweep instruction to the Base treasury queue.
func PublishB2BBaseTreasury(q *Queue, msg TreasurySweepMessage) error {
	return publishToChainTreasuryQueue(q, QueueBaseTreasury, msg)
}

// PublishB2BDLQ routes a failed sweep to the chain-specific dead-letter queue.
// Prefer letting the broker route via x-dead-letter-exchange (automatic on Nack);
// use this only when you need to publish to the DLQ directly without a delivery.
func PublishB2BDLQ(q *Queue, chain ChainType, orderID string, reason string) error {
	dlqName, err := chainDLQName(chain)
	if err != nil {
		return err
	}

	msg := DLQMessage{
		OrderID: orderID,
		Chain:   chain,
		Reason:  reason,
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("PublishB2BDLQ: marshal: %w", err)
	}

	if err := q.publish("", dlqName, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	}); err != nil {
		log.Printf("[B2B_DLQ] Failed to publish order_id=%s chain=%s: %v", orderID, chain, err)
		return fmt.Errorf("PublishB2BDLQ: publish: %w", err)
	}

	log.Printf("[B2B_DLQ] Sent order_id=%s chain=%s reason=%q → %s", orderID, chain, reason, dlqName)
	return nil
}

func publishToChainTreasuryQueue(q *Queue, queueName string, msg TreasurySweepMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("publish to %s: marshal: %w", queueName, err)
	}

	if err := q.publish("", queueName, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	}); err != nil {
		log.Printf("[B2B_PUBLISHER] Failed to publish to %s order_id=%s: %v", queueName, msg.OrderID, err)
		return fmt.Errorf("publish to %s: %w", queueName, err)
	}

	log.Printf("[B2B_PUBLISHER] Published order_id=%s chain=%s amount=%.6f → %s",
		msg.OrderID, msg.Chain, msg.AmountUSDC, queueName)
	return nil
}

func chainDLQName(chain ChainType) (string, error) {
	switch chain {
	case ChainSui:
		return QueueSuiDLQ, nil
	case ChainSolana:
		return QueueSolanaDLQ, nil
	case ChainBase:
		return QueueBaseDLQ, nil
	default:
		return "", fmt.Errorf("chainDLQName: unknown chain %q", chain)
	}
}
