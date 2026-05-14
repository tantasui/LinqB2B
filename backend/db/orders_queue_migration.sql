-- ─────────────────────────────────────────────────────────────────────────────
-- Run AFTER migration.sql (which creates merchants and deposits tables).
--
-- orders_queue: persistent FIFO queue for incoming deposit events.
-- The webhook handler enqueues every verified deposit here; a routing worker
-- dequeues and forwards each message to the appropriate chain-specific queue.
--
-- Status lifecycle: pending → processing → routed | dead_letter
-- Messages not processed within 1 hour (expires_at) are moved to dead_letter
-- by the expiry cleaner background job.
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS orders_queue (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Deposit identity
    order_id         VARCHAR(255)  NOT NULL,           -- deposit UUID from deposits table
    merchant_id      UUID          NOT NULL,           -- FK to merchants.id
    chain            VARCHAR(16)   NOT NULL
                         CHECK (chain IN ('sui', 'solana', 'base')),
    amount_usdc      NUMERIC(18,6) NOT NULL,
    tx_hash          VARCHAR(255)  NOT NULL UNIQUE,    -- idempotency key (same as deposits.tx_hash)
    timestamp        TIMESTAMPTZ   NOT NULL,           -- blockchain event time

    -- Flexible order identifier from the incoming webhook payload.
    -- Stores eventId today; field name intentionally generic because the
    -- upstream payload structure may evolve.
    pending_order_id VARCHAR(255),

    -- Lifecycle
    status           VARCHAR(32)   NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'processing', 'routed', 'dead_letter')),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ   NOT NULL,           -- created_at + 1-hour TTL
    error_message    TEXT                              -- reason, populated on dead_letter
);

-- Dequeue path: find the oldest pending, non-expired message.
CREATE INDEX IF NOT EXISTS idx_orders_queue_dequeue
    ON orders_queue (created_at)
    WHERE status = 'pending';

-- Expiry sweep: find non-terminal messages past their TTL.
CREATE INDEX IF NOT EXISTS idx_orders_queue_expires
    ON orders_queue (expires_at)
    WHERE status IN ('pending', 'processing');

-- Chain routing: look up pending messages by chain.
CREATE INDEX IF NOT EXISTS idx_orders_queue_chain
    ON orders_queue (chain, created_at)
    WHERE status = 'pending';

COMMENT ON TABLE orders_queue IS
    'Persistent FIFO queue for incoming deposit events. 1-hour TTL; expired messages are moved to dead_letter.';

COMMENT ON COLUMN orders_queue.order_id IS
    'deposit UUID — links this queue entry to the deposits table.';

COMMENT ON COLUMN orders_queue.pending_order_id IS
    'Event identifier from the webhook payload (eventId). Name kept generic as the payload structure may change.';

COMMENT ON COLUMN orders_queue.status IS
    'pending → processing → routed | dead_letter';

COMMENT ON COLUMN orders_queue.expires_at IS
    'created_at + 1 hour. Messages still in pending/processing after this timestamp are dead-lettered by the background expiry cleaner.';
