**# Task 1.2 — B2B Order Queue Worker

## What it does

Consumes orders from the main order queue and routes each one to the correct
chain-specific treasury queue based on the `chain` field in the message.
That's it. No processing, no DB calls — pure routing.

```
orders.queue
    │
    ├── chain: sui    → b2b-sui-treasury-queue
    ├── chain: solana → b2b-solana-treasury-queue
    └── chain: base   → b2b-base-treasury-queue
```

## Files changed

| File | What changed |
|---|---|
| `internal/workers/b2b_order_worker.go` | **New.** The worker itself |
| `internal/queue/model.go` | Added the three treasury queue name constants |
| `internal/queue/queue.go` | Exported `MainQueueName`, declared treasury queues in topology, added `NewConsumerChannel()` |
| `cmd/main.go` | Added `go workers.StartB2BOrderWorker(orderQueue)` |

## Ack strategy

| Scenario | Action |
|---|---|
| Successfully published to chain queue | `d.Ack(false)` |
| Publish failed (broker issue, transient) | `d.Nack(false, true)` — requeue |
| Unknown chain or malformed JSON | `d.Nack(false, false)` — DLQ |

## Key design decisions

- **Single goroutine** — worker is not parallelized by design
- **autoAck: false** — every message is manually acknowledged
- **Consumer owns its channel** — two fresh channels are opened from the shared
  connection: one for consuming, one for publishing. Neither shares the main
  publish channel used by the webhook handler
- **Treasury queues declared at startup** — `declareTopology()` now declares
  all three chain queues so they exist before the worker tries to route into them
- **Raw body forwarding** — the original message bytes are forwarded as-is to
  the chain queue, no re-serialization

---

## ⚑ Flags for spec alignment

Two intentional deviations from the Task 1.2 spec. Both have good reasons but
need to be confirmed with the team before Task 1.3 depends on them.

### Flag 1 — `connections.GetNewChannel()` vs `queue.NewConsumerChannel()`

**Spec says:** use `connections.GetNewChannel()`

**What was built:** `queue.NewConsumerChannel()` — a method on the `Queue` struct
that opens a fresh channel from the shared connection.

**Why:** The Quant branch has no `connections` package and no global
`RabbitMQConn`. LinqV2 has one because it was built that way from the start.
Adding a global connections package here would mean a second RabbitMQ dial
alongside the one `queue.New()` already does — two connections for no reason.
`NewConsumerChannel()` does exactly what `GetNewChannel()` does: opens a
dedicated channel for the consumer, never sharing the publish channel.

**Action needed:** If Task 1.3 or later tasks introduce a `connections` package
with a global pool, `NewConsumerChannel()` can be swapped out for
`connections.GetNewChannel()` with zero logic change.

---

### Flag 2 — Queue name `orders.queue` vs `b2b-order-queue`

**Spec says:** consume from `b2b-order-queue`

**What was built:** consumes from `orders.queue`

**Why:** Task 1.1 (already merged in Quant) declared the main queue as
`orders.queue` and the webhook handler publishes into it via that name.
If the worker consumed from `b2b-order-queue`, it would sit on an empty queue
forever — nothing publishes there.

**Action needed:** Confirm with the team whether the queue should be renamed to
`b2b-order-queue` to match the spec. If yes, it's a one-line change in
`queue.go` (`MainQueueName = "b2b-order-queue"`) and Task 1.1 needs to be
updated to match. Do this before any other service starts depending on the
current name.**
