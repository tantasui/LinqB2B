-- ─────────────────────────────────────────────────────────────────────────────
-- businesses table
-- Each row represents an onboarded B2B client. The derivation_index is the
-- BIP-44 account index used to generate that business's wallet addresses from
-- the platform master mnemonic — it is unique and never reused.
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS businesses (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMP WITH TIME ZONE,

    business_id       VARCHAR(64)   NOT NULL,
    name              VARCHAR(255)  NOT NULL,
    webhook_url       VARCHAR(2048) NOT NULL,
    webhook_secret    VARCHAR(512)  NOT NULL,
    derivation_index  INTEGER       NOT NULL,
    active            BOOLEAN       NOT NULL DEFAULT TRUE
);

-- Ensure business_id and derivation_index are globally unique
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_business_id
    ON businesses (business_id)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_derivation_index
    ON businesses (derivation_index)
    WHERE deleted_at IS NULL;

-- Fast look-up for the dispatcher (find business by wallet address → business)
-- wallet_addresses already has idx_wallet_business_id; no extra index needed here.

-- ─────────────────────────────────────────────────────────────────────────────
-- wallet_addresses table  (idempotent, recreates cleanly if missing)
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS wallet_addresses (
    id           BIGSERIAL PRIMARY KEY,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMP WITH TIME ZONE,

    address      VARCHAR(255) NOT NULL,
    type         VARCHAR(64)  NOT NULL,   -- network type: sui, solana, evm …
    standard     VARCHAR(64),             -- token standard: erc20, spl …
    business_id  VARCHAR(64)  NOT NULL,   -- FK to businesses.business_id
    asset_type   VARCHAR(255)             -- e.g. "USDC,USDT"
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_address_network
    ON wallet_addresses (address, type)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_wallet_business_id  ON wallet_addresses (business_id);
CREATE INDEX IF NOT EXISTS idx_wallet_address_type ON wallet_addresses (type);

COMMENT ON TABLE businesses        IS 'Onboarded B2B clients with webhook delivery config';
COMMENT ON TABLE wallet_addresses  IS 'Per-business wallet addresses across all supported chains';
COMMENT ON COLUMN businesses.derivation_index
    IS 'BIP-44 account index — unique, monotonically increasing, never reused';