# B2B Stablecoin Payment Platform

A production-ready infrastructure layer that watches multiple blockchains for stablecoin payments and delivers real-time signed webhook notifications to registered businesses.

Phase 1 target: **Sui USDC**. Solana and EVM chains follow in Phases 2 and 3.

---

## How it works

```
Business registers via POST /admin/onboard
        │
        ▼
Platform derives dedicated wallet addresses (Sui, Solana, Ethereum)
        │
        ▼
Business shares their Sui address with their own customers
        │
        ▼
Customer sends USDC to that address on-chain
        │
        ▼
Indexer detects the USDC transfer on Sui
        │
        ▼
Indexer publishes event to NATS JetStream
        │
        ▼
Dispatcher consumes event → looks up business by address in shared DB
        │
        ▼
Dispatcher signs payload with HMAC-SHA256 and POSTs to business webhook URL
        │
        ▼
Business verifies signature and credits their user's account
```

---

## Architecture

The platform runs as **two independent services** that share a single PostgreSQL database.

### Indexer (`cmd/indexer`)

Watches the Sui blockchain via gRPC, detects USDC transfers to monitored addresses, and publishes transaction events to NATS JetStream. Also serves the admin HTTP API for business onboarding.

**Workers:**
- **Regular** — real-time checkpoint polling
- **Catchup** — backfills historical gaps on startup
- **Rescanner** — retries any failed checkpoints
- **Manual** — processes explicit block ranges from a Redis queue

### Dispatcher (`cmd/dispatcher`)

Consumes transaction events from NATS JetStream, looks up which business owns the destination address, and delivers a signed webhook to that business's registered endpoint with exponential-backoff retries.

---

## Project structure

```
cmd/
  indexer/           Entry point for the indexer + admin API
  dispatcher/        Entry point for the webhook dispatcher

internal/
  indexer/sui.go     Sui checkpoint parsing, finality logic, USDC event parsing
  b2b/
    handler.go       Dispatcher: NATS → DB lookup → webhook delivery
    security.go      HMAC-SHA256 signing and verification
    types.go         WebhookPayload, SettlementInfo types
  onboarding/
    admin.go         POST /admin/onboard handler

pkg/
  model/
    business.go      Business DB model
    wallet_address.go  WalletAddress DB model
  repository/        Generic GORM repository (Find, FindOne, Save, Count)
  wallet/wallet.go   BIP-44 multi-chain address derivation
  common/config/     YAML config loading

sql/
  business.sql       DB migration for businesses + wallet_addresses tables
  wallet_address.sql Legacy migration (superseded by business.sql)

configs/
  b2b/sui.yaml       B2B platform configuration
```

---

## Database schema

Both services connect to the same PostgreSQL instance. The indexer writes to it (via onboarding); the dispatcher reads from it.

### `businesses`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key, auto-generated |
| `business_id` | varchar(64) | Unique slug, e.g. `"acme_corp"` |
| `name` | varchar(255) | Display name |
| `webhook_url` | varchar(2048) | HTTPS endpoint for payment events |
| `webhook_secret` | varchar(512) | HMAC-SHA256 signing secret |
| `derivation_index` | integer | BIP-44 account index, unique, never reused |
| `active` | boolean | Set false to pause webhook delivery |

### `wallet_addresses`

| Column | Type | Notes |
|---|---|---|
| `id` | bigserial | Primary key |
| `address` | varchar(255) | On-chain wallet address |
| `type` | varchar(64) | Network: `sui`, `sol`, `evm` |
| `business_id` | varchar(64) | FK to `businesses.business_id` |
| `asset_type` | varchar(255) | Assets monitored, e.g. `"USDC"` |

The bloom filter in the indexer is seeded from `wallet_addresses` at startup so the indexer only emits events for addresses it actually manages.

---

## Setup

### Prerequisites

- Go 1.25+
- PostgreSQL 14+
- NATS Server with JetStream enabled
- Redis

### 1. Run the database migration

```bash
psql $DATABASE_URL -f sql/business.sql
```

### 2. Configure

Copy and edit the config:

```bash
cp configs/b2b/sui.yaml configs/b2b/local.yaml
```

Set environment variables (never put real secrets in the YAML):

```bash
export MASTER_MNEMONIC="your twelve word bip39 mnemonic here"
export DATABASE_URL="postgres://user:password@localhost:5432/multichain"
```

Key fields in `sui.yaml`:

```yaml
services:
  master_mnemonic: "${MASTER_MNEMONIC}"   # BIP-44 seed for address derivation
  database:
    url: "${DATABASE_URL}"
  nats:
    url: "nats://localhost:4222"
  redis:
    url: "localhost:6379"

chains:
  sui_mainnet:
    confirmations: 1    # checkpoints behind tip before confirming
```

### 3. Build

```bash
go build -o indexer    cmd/indexer/main.go
go build -o dispatcher cmd/dispatcher/main.go
```

### 4. Start infrastructure

```bash
docker-compose up -d   # starts PostgreSQL, NATS, Redis
```

### 5. Start the indexer

```bash
./indexer index \
  --config configs/b2b/local.yaml \
  --chains=sui_mainnet \
  --from-latest
```

### 6. Start the dispatcher

```bash
DATABASE_URL=$DATABASE_URL \
NATS_URL=nats://localhost:4222 \
./dispatcher
```

---

## API Reference

### `POST /admin/onboard`

Creates a new business, derives wallet addresses for all supported chains, and persists both to the shared database.

**Request**

```json
{
  "businessId": "acme_corp",
  "name": "Acme Corporation",
  "webhookUrl": "https://payments.acme.io/webhook",
  "webhookSecret": "optional-bring-your-own-secret"
}
```

| Field | Required | Notes |
|---|---|---|
| `businessId` | Yes | Unique slug. Immutable after creation. |
| `name` | Yes | Display name |
| `webhookUrl` | Yes | Must be HTTPS in production |
| `webhookSecret` | No | If omitted, a 32-byte random secret is generated |

**Response `201 Created`**

```json
{
  "businessId": "acme_corp",
  "derivationIndex": 0,
  "addresses": {
    "sui":      "0xabc123...",
    "solana":   "7xKXtg...",
    "ethereum": "0xdef456..."
  },
  "webhookSecret": "a1b2c3...",
  "createdAt": "2025-01-01T12:00:00Z"
}
```

> **Important:** `webhookSecret` is returned **once only** at creation time. Store it securely — it will not be returned again. Share it with your team for webhook signature verification.

**Error responses**

| Status | Reason |
|---|---|
| `400` | Missing required fields |
| `409` | `businessId` already exists |
| `500` | Internal error (check logs) |

---

### `GET /health`

```json
{
  "status": "ok",
  "timestamp": "2025-01-01T12:00:00Z",
  "version": "1.0.0"
}
```

---

## Webhook delivery

When a confirmed USDC payment arrives at a monitored address, the dispatcher POSTs to the registered `webhookUrl`.

### Payload

```json
{
  "eventId": "evt_1735689600000000000",
  "eventType": "payment.confirmed",
  "businessId": "acme_corp",
  "webhookVersion": "1.0",
  "sentAt": "2025-01-01T12:00:00Z",
  "transaction": {
    "txHash": "3Fz8wLkMnPqRs...",
    "networkId": "SUI",
    "blockNumber": 48500100,
    "blockHash": "checkpoint-digest",
    "fromAddress": "0xsender...",
    "toAddress": "0xabc123...",
    "assetAddress": "0xdba34672e30cb065b1f93e3ab55318768fd6fef66c15942c9f7cb846e2f900e7::usdc::USDC",
    "amount": "10000000",
    "type": "token_transfer",
    "txFee": "0.001",
    "timestamp": 1735689600,
    "confirmations": 2,
    "status": "confirmed"
  },
  "settlement": {
    "status": "pending",
    "currency": "",
    "estimatedAmount": ""
  }
}
```

`amount` is in the smallest unit of the asset (USDC uses 6 decimal places, so `10000000` = 10 USDC).

### Signature verification

Every webhook request carries two headers:

```
X-Webhook-Signature: sha256=<hex>
X-Webhook-Timestamp: <unix_seconds>
```

**To verify in your backend:**

1. Reject requests where `X-Webhook-Timestamp` is more than 5 minutes old (replay attack prevention)
2. Compute `HMAC-SHA256(webhookSecret, rawRequestBody)`
3. Compare with `X-Webhook-Signature` using a constant-time comparison

**Go example:**

```go
func verifyWebhook(secret []byte, body []byte, sigHeader, tsHeader string) error {
    ts, err := strconv.ParseInt(tsHeader, 10, 64)
    if err != nil || time.Now().Unix()-ts > 300 {
        return errors.New("timestamp too old or invalid")
    }
    mac := hmac.New(sha256.New, secret)
    mac.Write(body)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    if !hmac.Equal([]byte(expected), []byte(sigHeader)) {
        return errors.New("signature mismatch")
    }
    return nil
}
```

**Python example:**

```python
import hmac, hashlib, time

def verify_webhook(secret: str, body: bytes, sig_header: str, ts_header: str) -> bool:
    if abs(time.time() - int(ts_header)) > 300:
        return False
    expected = "sha256=" + hmac.new(
        secret.encode(), body, hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, sig_header)
```

### Retry schedule

If your endpoint does not return a `2xx` response, the dispatcher retries on this schedule:

| Attempt | Delay |
|---|---|
| 1 | 2 seconds |
| 2 | 8 seconds |
| 3 | 30 seconds |
| 4 | 2 minutes |

After all retries are exhausted the event is logged as failed. NATS JetStream's durable consumer ensures no events are lost across dispatcher restarts.

---

## Sui finality model

Sui has no reorgs. A checkpoint is BFT-final the moment it carries a `ValidatorAggregatedSignature` from a quorum of 2/3+ of staked validators.

The `confirmations` field in the config controls how many checkpoints behind the tip we wait before marking a transaction `confirmed`. The default is `1`. Set it to `0` on a private/dedicated node to process every checkpoint immediately.

USDC on Sui is Circle's native (non-bridged) USDC:
```
0xdba34672e30cb065b1f93e3ab55318768fd6fef66c15942c9f7cb846e2f900e7::usdc::USDC
```

---

## Known limitations (Phase 1)

- **`/admin/onboard` has no authentication.** Add an admin API key check or network-level access control before exposing this endpoint.
- **`SettlementInfo.currency` and `estimatedAmount` are not populated.** Fiat settlement estimation is a Phase 2 concern.
- **Wallet derivation uses a simplified seed.** The current `wallet.go` does not implement full SLIP-0010 per-index derivation for Sui and Solana, meaning all businesses currently get the same derived address. This must be fixed before handling real funds.
- **No management API.** There is no endpoint yet to list, update, or deactivate businesses.

---

## Environment variables

| Variable | Service | Description |
|---|---|---|
| `MASTER_MNEMONIC` | Indexer | BIP-39 mnemonic for wallet derivation |
| `DATABASE_URL` | Both | PostgreSQL connection string |
| `NATS_URL` | Both | NATS server URL |
| `NATS_SUBJECT` | Dispatcher | JetStream subject (default: `transfer.event.dispatch`) |
| `NATS_DURABLE` | Dispatcher | Consumer durable name (default: `b2b-webhook-dispatcher`) |
| `ENV` | Both | `development` or `production` |

---

## ToDo

### Phase 1 — Sui USDC ✅
- Sui checkpoint finality logic
- USDC event parsing (treasury events + BalanceChanges)
- Business onboarding with wallet address derivation
- Shared DB read by both indexer and dispatcher
- HMAC-signed webhook delivery with retry

### Phase 2 — Solana USDC/USDT
- Goroutine pool for Solana slot fetching (high TPS)
- SPL token transfer parsing
- `confirmed` commitment level mapping
