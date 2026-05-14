# Test Setup — Sui USDC Flow

## Prerequisites
- Docker (for Postgres, Redis, NATS)
- Go 1.25+
- A Sui testnet wallet with testnet USDC

---

## Step 1 — Generate a test mnemonic

Run once and save the output. Use this same value everywhere as `MASTER_MNEMONIC`.

```bash
go run -mod=mod golang.org/x/tools/cmd/stringer@latest 2>/dev/null; \
python3 -c "
import secrets, hashlib
words=['abandon','ability','able','about','above','absent','absorb','abstract','absurd','abuse',
       'access','accident','account','accuse','achieve','acid','acoustic','acquire','cross','act',
       'action','actor','actress','actual','adapt','add','addict','address','adjust','admit',
       'adult','advance','advice','aerobic','afford','afraid','again','agent','agree','ahead']
print(' '.join(secrets.choice(words) for _ in range(12)))
"
```

Or use any BIP-39 generator (e.g. https://iancoleman.io/bip39/). Set `Mnemonic Length = 12`.
**This is testnet-only. Never use for real funds.**

---

## Step 2 — Start infrastructure

From the `indexer/` directory:

```bash
docker-compose up -d
```

This starts Postgres (5432), Redis (6379), NATS (4222).

---

## Step 3 — Run DB migrations

```bash
# Indexer schema (businesses + wallet_addresses)
psql postgres://postgres:postgres@localhost:5432/postgres -f indexer/sql/business.sql

# B2B merchant schema (merchants + deposits)
psql postgres://postgres:postgres@localhost:5432/postgres -f b2b-merchant/db/migration.sql
```

---

## Step 4 — Register a merchant (before starting the indexer!)

The bloom filter loads addresses at startup — register first so the indexer
picks up the address immediately.

```bash
cd b2b-merchant
cp .env.example .env
# Edit .env: set MASTER_MNEMONIC to the mnemonic you generated in Step 1
source .env
go run cmd/main.go &
```

Open **http://localhost:8081** and register a merchant:
- Name: `Test Shop`
- Bank: `Barclays`
- Account: `12345678`

Copy the **Sui address** shown — this is where you'll send testnet USDC.

---

## Step 5 — Start the indexer

From the `indexer/` directory, in a new terminal:

```bash
export MASTER_MNEMONIC="<your mnemonic>"
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres"

go run cmd/indexer/main.go index \
  --config configs/b2b/sui-testnet.yaml \
  --chains=sui_mainnet \
  --from-latest
```

Watch the logs — it will print `Onboarding endpoint registered` and start polling Sui testnet checkpoints.

---

## Step 6 — Start the dispatcher

In another terminal (from `indexer/`):

```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres"
export NATS_URL="nats://localhost:4222"

go run cmd/dispatcher/main.go
```

---

## Step 7 — Send testnet USDC

Send any amount of testnet USDC to the Sui address you copied in Step 4.

Testnet USDC contract:
```
0xa1ec7fc00a6f40db9693ad1415d0c193ad3906494428cf252621037bd7117e29::usdc::USDC
```

You can get testnet USDC from: https://faucet.circle.com (select Sui Testnet).

---

## Step 8 — Watch the flow

**Indexer terminal** — should log:
```
Processing checkpoint <N>
Matched address 0x...
Published transfer event
```

**Dispatcher terminal** — should log:
```
Processing tx: <hash> on sui
Webhook delivered businessId=test_shop status=200
```

**Dashboard** (http://localhost:8081) — deposit card appears and transitions:
```
⬇ Received  →  🔄 Sweeping  →  🏦 Swept  →  💱 Fiat Pending  →  ✅ Completed
```

---

## Reverting indexer changes

```bash
cd indexer
git checkout quant          # back to the original branch
git branch -d quant-testnet # optional: delete the testnet branch
```

## Folder structure

```
Desktop/
  go.work              ← Go workspace (gopls / IDE)
  indexer/             ← multichain indexer (branch: quant-testnet)
  b2b-merchant/        ← B2B payment platform API + dashboard
```
