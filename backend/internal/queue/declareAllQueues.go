package queue

import (
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// declareAllQueues declares all chain treasury queues and their dead-letter queues.
// DLQs are declared first; treasury queues reference them via x-dead-letter-routing-key
// so that Nack(requeue=false) is automatically routed by the broker — no manual
// PublishB2BDLQ call is needed inside workers.
func declareAllQueues(ch *amqp.Channel) error {
	// DLQs must exist before the treasury queues that reference them.
	for _, name := range []string{QueueSuiDLQ, QueueSolanaDLQ, QueueBaseDLQ} {
		if _, err := ch.QueueDeclare(name, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declareAllQueues: declare %s: %w", name, err)
		}
		log.Printf("[QUEUE] Declared %s", name)
	}

	// Treasury queues wired to their DLQ via the default exchange.
	treasuryQueues := []struct{ name, dlq string }{
		{QueueSuiTreasury, QueueSuiDLQ},
		{QueueSolanaTreasury, QueueSolanaDLQ},
		{QueueBaseTreasury, QueueBaseDLQ},
	}
	for _, tq := range treasuryQueues {
		args := amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": tq.dlq,
		}
		if _, err := ch.QueueDeclare(tq.name, true, false, false, false, args); err != nil {
			return fmt.Errorf("declareAllQueues: declare %s: %w", tq.name, err)
		}
		log.Printf("[QUEUE] Declared %s → DLQ: %s", tq.name, tq.dlq)
	}
	return nil
}
