package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// fall back to reading .env in the project root
		dbURL = readEnvFile(".env")
	}
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set and not found in .env")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	fmt.Println("Connected.")

	for _, stmt := range migrations {
		label := strings.SplitN(strings.TrimSpace(stmt), "\n", 2)[0]
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("migration failed [%s]: %v", label, err)
		}
		fmt.Printf("OK  %s\n", label)
	}

	fmt.Println("\nAll migrations applied successfully.")
}

// readEnvFile reads DATABASE_URL from a KEY=VALUE .env file.
func readEnvFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "DATABASE_URL=") {
			return strings.TrimPrefix(line, "DATABASE_URL=")
		}
	}
	return ""
}

// migrations are executed in order. Tables are ordered to satisfy FK constraints.
var migrations = []string{
	// ── Indexer tables (needed by the merchant service) ────────────────────────
	`-- businesses
CREATE TABLE IF NOT EXISTS businesses (
    id               BIGSERIAL    PRIMARY KEY,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    business_id      VARCHAR(64)  NOT NULL UNIQUE,
    name             VARCHAR(255) NOT NULL,
    webhook_url      VARCHAR(1000),
    webhook_secret   VARCHAR(255),
    derivation_index INTEGER      NOT NULL DEFAULT 0,
    active           BOOLEAN      NOT NULL DEFAULT true
)`,

	`-- wallet_addresses
CREATE TABLE IF NOT EXISTS wallet_addresses (
    id          BIGSERIAL    PRIMARY KEY,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    address     VARCHAR(255) NOT NULL UNIQUE,
    type        VARCHAR(64)  NOT NULL,
    standard    VARCHAR(64),
    business_id VARCHAR(64)  NOT NULL REFERENCES businesses(business_id),
    asset_type  VARCHAR(64)
)`,

	// ── App tables ─────────────────────────────────────────────────────────────
	`-- merchants
CREATE TABLE IF NOT EXISTS merchants (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ,
    business_id          VARCHAR(64)  NOT NULL UNIQUE,
    name                 VARCHAR(255) NOT NULL,
    email                VARCHAR(255),
    bank_name            VARCHAR(255),
    account_number       VARCHAR(64),
    sui_address          VARCHAR(255),
    encrypted_private_key TEXT,
    status               VARCHAR(32)  NOT NULL DEFAULT 'active',
    password_hash        VARCHAR(255)
)`,

	`-- merchants indexes
CREATE INDEX IF NOT EXISTS idx_merchant_business_id ON merchants (business_id);
CREATE INDEX IF NOT EXISTS idx_merchant_sui_address  ON merchants (sui_address)`,

	`-- pending_orders
CREATE TABLE IF NOT EXISTS pending_orders (
    id                   UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at           TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    expires_at           TIMESTAMPTZ   NOT NULL,
    merchant_id          UUID          NOT NULL REFERENCES merchants(id),
    amount_ngn           NUMERIC(18,2) NOT NULL,
    expected_amount_usdc NUMERIC(18,6) NOT NULL,
    exchange_rate        NUMERIC(18,4) NOT NULL,
    merchant_address     VARCHAR(255)  NOT NULL,
    encrypted_private_key TEXT         NOT NULL,
    customer_email       VARCHAR(255),
    status               VARCHAR(32)   NOT NULL DEFAULT 'pending'
)`,

	`-- pending_orders index
CREATE INDEX IF NOT EXISTS idx_pending_orders_merchant_status_created
    ON pending_orders (merchant_id, status, created_at)`,

	`-- deposits
CREATE TABLE IF NOT EXISTS deposits (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    merchant_id      UUID          REFERENCES merchants(id),
    business_id      VARCHAR(64)   NOT NULL,
    tx_hash          VARCHAR(255)  NOT NULL UNIQUE,
    amount_raw       VARCHAR(64)   NOT NULL,
    amount_usdc      NUMERIC(18,6),
    sui_address      VARCHAR(255)  NOT NULL,
    status           VARCHAR(32)   NOT NULL DEFAULT 'received',
    raw_payload      JSONB,
    pending_order_id UUID          REFERENCES pending_orders(id),
    network_id       VARCHAR(64)
)`,

	`-- deposits indexes
CREATE INDEX IF NOT EXISTS idx_deposit_merchant_id    ON deposits (merchant_id);
CREATE INDEX IF NOT EXISTS idx_deposit_business_id    ON deposits (business_id);
CREATE INDEX IF NOT EXISTS idx_deposit_status         ON deposits (status);
CREATE INDEX IF NOT EXISTS idx_deposit_network_status ON deposits (network_id, status)`,

	`-- refunds
CREATE TABLE IF NOT EXISTS refunds (
    id                 UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deposit_id         UUID          NOT NULL REFERENCES deposits(id),
    network_id         VARCHAR(64)   NOT NULL,
    tx_hash            VARCHAR(255),
    refund_amount_usdc NUMERIC(18,6) NOT NULL,
    recipient_wallet   VARCHAR(255)  NOT NULL,
    status             VARCHAR(32)   NOT NULL DEFAULT 'pending',
    failure_reason     TEXT
)`,

	`-- refunds index
CREATE INDEX IF NOT EXISTS idx_refunds_status_network ON refunds (status, network_id)`,

	`-- payment_links
CREATE TABLE IF NOT EXISTS payment_links (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    merchant_id UUID          NOT NULL REFERENCES merchants(id),
    amount_ngn  NUMERIC(18,2) NOT NULL,
    url         VARCHAR(1000) NOT NULL,
    status      VARCHAR(32)   NOT NULL DEFAULT 'active'
)`,

	`-- payment_links index
CREATE INDEX IF NOT EXISTS idx_payment_links_merchant_id ON payment_links (merchant_id)`,

	`-- settlements
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
)`,

	`-- settlements index
CREATE INDEX IF NOT EXISTS idx_settlements_merchant_id ON settlements (merchant_id)`,
}
