package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fystack/b2b-merchant/internal/auth"
	"github.com/fystack/b2b-merchant/internal/crypto"
	"github.com/fystack/b2b-merchant/internal/deposit"
	"github.com/fystack/b2b-merchant/internal/merchant"
	"github.com/fystack/b2b-merchant/internal/order"
	"github.com/fystack/b2b-merchant/internal/paymentlink"
	"github.com/fystack/b2b-merchant/internal/queue"
	"github.com/fystack/b2b-merchant/internal/settlement"
	"github.com/fystack/b2b-merchant/internal/webhook"
	"github.com/fystack/b2b-merchant/internal/worker"
	"github.com/fystack/b2b-merchant/internal/workers"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/infra"
	indexermodel "github.com/fystack/multichain-indexer/b2b-platform/pkg/model"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/repository"
	"github.com/redis/go-redis/v9"
)

func main() {
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/postgres")
	rabbitmqURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	mnemonic := mustEnv("MASTER_MNEMONIC")
	port := getEnv("PORT", "8081")
	env := getEnv("ENV", "development")
	webhookURL := getEnv("WEBHOOK_URL", fmt.Sprintf("http://localhost:%s/webhook", port))

	// ── Encryption ────────────────────────────────────────────────────────────
	encKeyHex := mustEnv("ENCRYPTION_KEY") // 32-byte key as 64 hex chars
	encKeyBytes, err := hex.DecodeString(encKeyHex)
	if err != nil || len(encKeyBytes) != 32 {
		log.Fatalf("ENCRYPTION_KEY must be 64 hex characters (32 bytes)")
	}
	encryptor, err := crypto.NewEncryptor(encKeyBytes)
	if err != nil {
		log.Fatalf("encryptor: %v", err)
	}

	// ── Database ──────────────────────────────────────────────────────────────
	gormDB, err := infra.NewDBConnection(dbURL, env)
	if err != nil {
		log.Fatalf("DB connect: %v", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Fatalf("get sql.DB: %v", err)
	}

	if err := runMigrations(sqlDB); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	businessRepo := repository.NewRepository[indexermodel.Business](gormDB)
	walletRepo := repository.NewRepository[indexermodel.WalletAddress](gormDB)

	// ── Order queue ───────────────────────────────────────────────────────────
	// RabbitMQ is optional: if unavailable the HTTP API still runs but
	// background sweep/treasury workers are disabled.
	var publisher queue.Publisher = queue.NoopPublisher{}
	orderQueue, err := queue.New(rabbitmqURL)
	if err != nil {
		log.Printf("WARNING: RabbitMQ unavailable (%v) — workers disabled, API running without queue", err)
	} else {
		defer orderQueue.Close()
		publisher = orderQueue
		go workers.StartB2BOrderWorker(orderQueue)
		go workers.StartB2BSuiTreasuryWorker(orderQueue, sqlDB, encryptor)
		go workers.StartB2BSolanaTreasuryWorker(orderQueue)
		go workers.StartB2BBaseTreasuryWorker(orderQueue)
	}

	// ── Redis (optional) ─────────────────────────────────────────────────────
	var redisClient *redis.Client
	if redisURL := getEnv("REDIS_URL", ""); redisURL != "" {
		redisOpts, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("WARNING: Redis URL invalid (%v) — webhook caching disabled", err)
		} else {
			c := redis.NewClient(redisOpts)
			pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := c.Ping(pingCtx).Err(); err != nil {
				log.Printf("WARNING: Redis ping failed (%v) — webhook caching disabled", err)
			} else {
				redisClient = c
				defer redisClient.Close()
			}
		}
	} else {
		log.Printf("WARNING: REDIS_URL not set — webhook caching disabled")
	}

	// ── Services ──────────────────────────────────────────────────────────────
	merchantSvc, err := merchant.NewService(sqlDB, gormDB, mnemonic, encryptor, businessRepo, walletRepo, webhookURL)
	if err != nil {
		log.Fatalf("merchant service: %v", err)
	}

	// ── Handlers ──────────────────────────────────────────────────────────────
	merchantHandler := merchant.NewHandler(merchantSvc)
	webhookHandler := webhook.NewHandler(sqlDB, publisher, redisClient)
	depositHandler := deposit.NewHandler(sqlDB)
	orderSvc := order.NewService(sqlDB, encryptor)
	orderHandler := order.NewHandler(orderSvc)
	paymentLinkHandler := paymentlink.NewHandler(sqlDB)
	settlementHandler := settlement.NewHandler(sqlDB)

	// ── Workers ───────────────────────────────────────────────────────────────
	// Note: In production we use a proper shutdown context instead of Background
	validatorWorker := worker.NewValidatorWorker(sqlDB)
	suiRefundWorker := worker.NewSuiRefundWorker(sqlDB, encryptor)
	
	go validatorWorker.Start(context.Background())
	go suiRefundWorker.Start(context.Background())

	// ── Routes ────────────────────────────────────────────────────────────────
	mux := http.NewServeMux()
	allowedOrigin := getEnv("FRONTEND_URL", "https://linq-b2b.netlify.app")

	// Merchant API
	mux.HandleFunc("/api/merchants", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			merchantHandler.Register(w, r)
		case http.MethodGet:
			merchantHandler.List(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Unprotected routes
	mux.HandleFunc("/api/merchants/login", merchantHandler.Login)
	
	mux.HandleFunc("/api/merchants/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/orders") && r.Method == http.MethodPost:
			orderHandler.Create(w, r)
		case strings.HasSuffix(path, "/me"):
			auth.Middleware(http.HandlerFunc(merchantHandler.Me)).ServeHTTP(w, r)
		case strings.HasSuffix(path, "/private-key"):
			auth.Middleware(http.HandlerFunc(merchantHandler.GetPrivateKey)).ServeHTTP(w, r)
		case strings.HasSuffix(path, "/deposits"):
			auth.Middleware(http.HandlerFunc(depositHandler.ListByMerchant)).ServeHTTP(w, r)
		case strings.Contains(path, "/payment-links"):
			auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					paymentLinkHandler.List(w, r)
				case http.MethodPost:
					paymentLinkHandler.Create(w, r)
				case http.MethodDelete:
					paymentLinkHandler.Delete(w, r)
				default:
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				}
			})).ServeHTTP(w, r)
		case strings.HasSuffix(path, "/stats"):
			auth.Middleware(http.HandlerFunc(merchantHandler.GetStats)).ServeHTTP(w, r)
		case strings.HasSuffix(path, "/password"):
			auth.Middleware(http.HandlerFunc(merchantHandler.UpdatePassword)).ServeHTTP(w, r)
		case strings.Contains(path, "/settlements"):
			auth.Middleware(http.HandlerFunc(settlementHandler.List)).ServeHTTP(w, r)
		default:
			// Public: GET /api/merchants/{id} — used by the payment page without auth
			if r.Method == http.MethodGet {
				merchantHandler.GetByID(w, r)
			} else {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})

	// Exchange rate — public
	mux.HandleFunc("/api/exchange-rates", merchantHandler.GetExchangeRate)

	// Order status polling — public, used by payment page
	mux.HandleFunc("/api/orders/", orderHandler.GetStatus)

	// Deposits API
	mux.HandleFunc("/api/deposits", depositHandler.List)

	// Webhook receiver (called by the dispatcher)
	mux.HandleFunc("/webhook", webhookHandler.Receive)

	// Frontend dashboard
	mux.HandleFunc("/", serveIndex)

	log.Printf("B2B Merchant API listening on :%s", port)
	log.Printf("Dashboard → http://localhost:%s", port)
	log.Printf("Webhook URL registered with indexer → %s", webhookURL)
	if err := http.ListenAndServe(":"+port, corsMiddleware(allowedOrigin, mux)); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func corsMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, "web/index.html")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func runMigrations(db *sql.DB) error {
	stmts := []string{
		// Drop and recreate indexer tables if they were created with wrong schema (BIGSERIAL id).
		// Safe on a fresh DB; on a populated DB these would be no-ops if id is already UUID.
		`DO $$ BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'businesses' AND column_name = 'id'
				AND data_type = 'bigint'
			) THEN
				DROP TABLE IF EXISTS wallet_addresses;
				DROP TABLE IF EXISTS businesses;
			END IF;
		END $$`,
		`CREATE TABLE IF NOT EXISTS businesses (
			id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			deleted_at       TIMESTAMPTZ,
			business_id      VARCHAR(64)  NOT NULL UNIQUE,
			name             VARCHAR(255) NOT NULL,
			webhook_url      VARCHAR(1000),
			webhook_secret   VARCHAR(255),
			derivation_index INTEGER      NOT NULL DEFAULT 0,
			active           BOOLEAN      NOT NULL DEFAULT true
		)`,
		`ALTER TABLE businesses ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`,
		`CREATE TABLE IF NOT EXISTS wallet_addresses (
			id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			deleted_at  TIMESTAMPTZ,
			address     VARCHAR(255) NOT NULL UNIQUE,
			type        VARCHAR(64)  NOT NULL,
			standard    VARCHAR(64),
			business_id VARCHAR(64)  NOT NULL REFERENCES businesses(business_id),
			asset_type  VARCHAR(64),
			active      BOOLEAN      NOT NULL DEFAULT true
		)`,
		`ALTER TABLE wallet_addresses ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`,
		`ALTER TABLE wallet_addresses ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true`,
		`CREATE TABLE IF NOT EXISTS merchants (
			id                    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			deleted_at            TIMESTAMPTZ,
			business_id           VARCHAR(64)  NOT NULL UNIQUE,
			name                  VARCHAR(255) NOT NULL,
			email                 VARCHAR(255),
			bank_name             VARCHAR(255),
			account_number        VARCHAR(64),
			sui_address           VARCHAR(255),
			encrypted_private_key TEXT,
			status                VARCHAR(32)  NOT NULL DEFAULT 'active',
			password_hash         VARCHAR(255)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_merchant_business_id ON merchants (business_id)`,
		`CREATE INDEX IF NOT EXISTS idx_merchant_sui_address  ON merchants (sui_address)`,
		`CREATE TABLE IF NOT EXISTS pending_orders (
			id                    UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
			updated_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
			expires_at            TIMESTAMPTZ   NOT NULL,
			merchant_id           UUID          NOT NULL REFERENCES merchants(id),
			amount_ngn            NUMERIC(18,2) NOT NULL,
			expected_amount_usdc  NUMERIC(18,6) NOT NULL,
			exchange_rate         NUMERIC(18,4) NOT NULL,
			merchant_address      VARCHAR(255)  NOT NULL,
			encrypted_private_key TEXT          NOT NULL,
			customer_email        VARCHAR(255),
			status                VARCHAR(32)   NOT NULL DEFAULT 'pending'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_orders_merchant_status_created
			ON pending_orders (merchant_id, status, created_at)`,
		`CREATE TABLE IF NOT EXISTS deposits (
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
		`CREATE INDEX IF NOT EXISTS idx_deposit_merchant_id    ON deposits (merchant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_deposit_business_id    ON deposits (business_id)`,
		`CREATE INDEX IF NOT EXISTS idx_deposit_status         ON deposits (status)`,
		`CREATE INDEX IF NOT EXISTS idx_deposit_network_status ON deposits (network_id, status)`,
		`CREATE TABLE IF NOT EXISTS refunds (
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
		`CREATE INDEX IF NOT EXISTS idx_refunds_status_network ON refunds (status, network_id)`,
		`CREATE TABLE IF NOT EXISTS payment_links (
			id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
			merchant_id UUID          NOT NULL REFERENCES merchants(id),
			amount_ngn  NUMERIC(18,2) NOT NULL,
			url         VARCHAR(1000) NOT NULL,
			status      VARCHAR(32)   NOT NULL DEFAULT 'active'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_links_merchant_id ON payment_links (merchant_id)`,
		`CREATE TABLE IF NOT EXISTS settlements (
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
		`CREATE INDEX IF NOT EXISTS idx_settlements_merchant_id ON settlements (merchant_id)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	log.Printf("migrations: all tables OK")
	return nil
}
