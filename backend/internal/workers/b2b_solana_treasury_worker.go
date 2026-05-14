package workers

import (
	"log"

	"github.com/fystack/b2b-merchant/internal/queue"
)

// StartB2BSolanaTreasuryWorker is a placeholder — implementation deferred to a later sprint.
// The Solana treasury queue is declared and ready; the sweep logic will be added here.
func StartB2BSolanaTreasuryWorker(q *queue.Queue) {
	log.Printf("[B2B_SOLANA_TREASURY_WORKER] Queue %s declared — worker not yet implemented (planned for later sprint)",
		queue.QueueSolanaTreasury)
}
