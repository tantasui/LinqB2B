package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fystack/multichain-indexer/b2b-platform/internal/b2b"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/infra"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/model"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/repository"
	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")
	dbURL := getEnv("DATABASE_URL", "postgres://indexer:password@localhost:5432/multichain")
	subject := getEnv("NATS_SUBJECT", "transfer.event.dispatch")
	durable := getEnv("NATS_DURABLE", "b2b-webhook-dispatcher")
	env := getEnv("ENV", "development")

	log.Println("Starting B2B Webhook Dispatcher...")

	// ── 1. Database ───────────────────────────────────────────────────────────
	// Both the indexer and the dispatcher read from the same shared DB.
	// The indexer populates wallet_addresses; the dispatcher reads businesses
	// and wallet_addresses to resolve incoming payments to a business + webhook.
	db, err := infra.NewDBConnection(dbURL, env)
	if err != nil {
		log.Fatalf("DB connect error: %v", err)
	}

	walletRepo := repository.NewRepository[model.WalletAddress](db)
	businessRepo := repository.NewRepository[model.Business](db)
	businessLookup := b2b.NewDBBusinessLookup(walletRepo, businessRepo)

	// ── 2. NATS ───────────────────────────────────────────────────────────────
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("NATS connect error: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("JetStream error: %v", err)
	}

	// ── 3. Handler ────────────────────────────────────────────────────────────
	handler := b2b.NewDispatcherHandler(js, businessLookup)

	// ── 4. Run ────────────────────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down dispatcher...")
		cancel()
	}()

	log.Printf("Dispatcher running — subject=%s durable=%s", subject, durable)
	if err := handler.Start(ctx, subject, durable); err != nil && err != context.Canceled {
		log.Fatalf("Dispatcher error: %v", err)
	}
	log.Println("Dispatcher stopped.")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
