-- ─────────────────────────────────────────────────────────────────────────────
-- Run AFTER sql/business.sql from the indexer repo, which creates the shared
-- businesses and wallet_addresses tables.
-- ─────────────────────────────────────────────────────────────────────────────

-- merchants: extended B2B client profile with banking details.
-- business_id links to businesses.business_id (managed by the indexer schema).
CREATE TABLE IF NOT EXISTS merchants (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ  NOT NULL    DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL    DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,

    business_id     VARCHAR(64)  NOT NULL UNIQUE,  -- FK to businesses.business_id
    name            VARCHAR(255) NOT NULL,
    email           VARCHAR(255),
    bank_name       VARCHAR(255),
    account_number  VARCHAR(64),
    sui_address             VARCHAR(255),                  -- derived Sui wallet address
    encrypted_private_key   TEXT,                          -- AES-256-GCM encrypted Ed25519 seed
    status                  VARCHAR(32)  NOT NULL DEFAULT 'active',
    password_hash           VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_merchant_business_id ON merchants (business_id);
CREATE INDEX IF NOT EXISTS idx_merchant_sui_address  ON merchants (sui_address);

-- deposits: every confirmed USDC transfer the indexer detected for this platform.
-- status lifecycle: received → processing → swept → fiat_pending → completed
CREATE TABLE IF NOT EXISTS deposits (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at   TIMESTAMPTZ   NOT NULL    DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL    DEFAULT NOW(),

    merchant_id  UUID          REFERENCES merchants(id),
    business_id  VARCHAR(64)   NOT NULL,
    tx_hash      VARCHAR(255)  NOT NULL UNIQUE,
    amount_raw   VARCHAR(64)   NOT NULL,   -- raw chain amount (USDC has 6 decimals)
    amount_usdc  NUMERIC(18,6),            -- amount_raw / 1e6
    sui_address      VARCHAR(255)  NOT NULL,
    status           VARCHAR(32)   NOT NULL DEFAULT 'received',
    raw_payload      JSONB,                    -- full webhook payload for audit
    pending_order_id UUID          REFERENCES pending_orders(id),
    network_id       VARCHAR(64)               -- e.g. 'sui', 'solana', 'base'
);

CREATE INDEX IF NOT EXISTS idx_deposit_merchant_id    ON deposits (merchant_id);
CREATE INDEX IF NOT EXISTS idx_deposit_business_id    ON deposits (business_id);
CREATE INDEX IF NOT EXISTS idx_deposit_status         ON deposits (status);
CREATE INDEX IF NOT EXISTS idx_deposit_network_status ON deposits (network_id, status);

COMMENT ON TABLE merchants IS 'Extended B2B client profiles with banking details';
COMMENT ON TABLE deposits  IS 'Confirmed USDC deposits detected by the indexer, with mock processing state';
COMMENT ON COLUMN deposits.status IS 'received → processing → swept → fiat_pending → completed';

-- pending_orders: payment link orders created when a customer enters an NGN amount.
-- The customer is shown a USDC amount and a Sui address to send funds to.
-- status lifecycle: pending → (expires after 1 hour)
CREATE TABLE IF NOT EXISTS pending_orders (
    id                   UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at           TIMESTAMPTZ   NOT NULL    DEFAULT NOW(),
    updated_at           TIMESTAMPTZ   NOT NULL    DEFAULT NOW(),
    expires_at           TIMESTAMPTZ   NOT NULL,

    merchant_id          UUID          NOT NULL REFERENCES merchants(id),
    amount_ngn           NUMERIC(18,2) NOT NULL,
    expected_amount_usdc NUMERIC(18,6) NOT NULL,
    exchange_rate        NUMERIC(18,4) NOT NULL,    -- USDC/NGN rate at creation
    merchant_address     VARCHAR(255)  NOT NULL,    -- Unique temporary Sui address
    encrypted_private_key TEXT         NOT NULL,    -- AES-256-GCM encrypted key for this order
    customer_email       VARCHAR(255),              -- optional
    status               VARCHAR(32)   NOT NULL DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS idx_pending_orders_merchant_status_created
    ON pending_orders (merchant_id, status, created_at);

COMMENT ON TABLE pending_orders IS 'Payment link orders: customer enters NGN, receives USDC payment instructions';
COMMENT ON COLUMN pending_orders.status IS 'pending → (auto-expires after 1 hour)';

-- refunds: created when a deposit amount mismatches the expected amount from pending_orders.
-- status lifecycle: pending → completed / failed
CREATE TABLE IF NOT EXISTS refunds (
    id                   UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at           TIMESTAMPTZ   NOT NULL    DEFAULT NOW(),
    updated_at           TIMESTAMPTZ   NOT NULL    DEFAULT NOW(),

    deposit_id           UUID          NOT NULL REFERENCES deposits(id),
    network_id           VARCHAR(64)   NOT NULL,
    tx_hash              VARCHAR(255),
    refund_amount_usdc   NUMERIC(18,6) NOT NULL,
    recipient_wallet     VARCHAR(255)  NOT NULL,
    status               VARCHAR(32)   NOT NULL DEFAULT 'pending',
    failure_reason       TEXT
);

CREATE INDEX IF NOT EXISTS idx_refunds_status_network
    ON refunds (status, network_id);

COMMENT ON TABLE refunds IS 'Pending and completed refunds for overpaid or underpaid deposits';

-- payment_links: shareable URLs generated by merchants with a pre-filled NGN amount.
-- status lifecycle: active → used / expired
CREATE TABLE IF NOT EXISTS payment_links (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ   NOT NULL    DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL    DEFAULT NOW(),
    merchant_id UUID          NOT NULL REFERENCES merchants(id),
    amount_ngn  NUMERIC(18,2) NOT NULL,
    url         VARCHAR(1000) NOT NULL,
    status      VARCHAR(32)   NOT NULL DEFAULT 'active'
);

CREATE INDEX IF NOT EXISTS idx_payment_links_merchant_id ON payment_links (merchant_id);

COMMENT ON TABLE payment_links IS 'Shareable payment URLs with pre-filled NGN amounts generated by merchants';

-- settlements: bank payout records created when swept USDC is converted and sent to merchant bank.
-- status lifecycle: pending | completed | failed
CREATE TABLE IF NOT EXISTS settlements (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    merchant_id     UUID          NOT NULL REFERENCES merchants(id),
    deposit_id      UUID          REFERENCES deposits(id),
    amount_usdc     NUMERIC(18,6) NOT NULL,
    amount_ngn      NUMERIC(18,2) NOT NULL,
    exchange_rate   NUMERIC(18,4) NOT NULL,
    bank_reference  VARCHAR(255),
    nomba_reference VARCHAR(255),
    status          VARCHAR(32)   NOT NULL DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS idx_settlements_merchant_id ON settlements (merchant_id);

COMMENT ON TABLE settlements IS 'Bank payout records: USDC swept → NGN disbursed to merchant bank account';
