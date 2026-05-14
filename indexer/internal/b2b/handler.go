package b2b

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/logger"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/types"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/model"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/repository"
	"github.com/nats-io/nats.go"
)

// BusinessLookup is a read-only view of the business repository used by the
// dispatcher to resolve a monitored wallet address → Business row.
type BusinessLookup interface {
	// FindByAddress returns the Business that owns the given wallet address on
	// the given network, or repository.ErrNotFound if no match.
	FindByAddress(ctx context.Context, address, networkType string) (*model.Business, error)
}

// dbBusinessLookup implements BusinessLookup on top of the two standard repos.
type dbBusinessLookup struct {
	walletRepo   repository.Repository[model.WalletAddress]
	businessRepo repository.Repository[model.Business]
}

func NewDBBusinessLookup(
	walletRepo repository.Repository[model.WalletAddress],
	businessRepo repository.Repository[model.Business],
) BusinessLookup {
	return &dbBusinessLookup{
		walletRepo:   walletRepo,
		businessRepo: businessRepo,
	}
}

// FindByAddress resolves: wallet_addresses(address, type) → business_id → Business.
func (d *dbBusinessLookup) FindByAddress(ctx context.Context, address, networkType string) (*model.Business, error) {
	wa, err := d.walletRepo.FindOne(ctx, repository.FindOptions{
		Where: repository.WhereType{
			"address": address,
			"type":    networkType,
		},
	})
	if err != nil {
		return nil, err // propagates ErrNotFound
	}

	biz, err := d.businessRepo.FindOne(ctx, repository.FindOptions{
		Where: repository.WhereType{"business_id": wa.BusinessID},
	})
	if err != nil {
		return nil, err
	}
	return biz, nil
}

// ─────────────────────────────────────────────────────────────────────────────

// DispatcherHandler manages the flow: NATS event → DB lookup → Webhook delivery.
type DispatcherHandler struct {
	js             nats.JetStreamContext
	businessLookup BusinessLookup
	httpClient     *http.Client
}

func NewDispatcherHandler(
	js nats.JetStreamContext,
	businessLookup BusinessLookup,
) *DispatcherHandler {
	return &DispatcherHandler{
		js:             js,
		businessLookup: businessLookup,
		httpClient:     &http.Client{Timeout: 15 * time.Second},
	}
}

// Start consuming events from NATS JetStream until ctx is cancelled.
func (h *DispatcherHandler) Start(ctx context.Context, subject, durableName string) error {
	var sub *nats.Subscription
	var err error

	// Retry loop to wait for the NATS stream to be created by the Indexer
	for {
		sub, err = h.js.PullSubscribe(
			subject,
			durableName,
			nats.PullMaxWaiting(128),
		)
		if err == nil {
			break
		}

		logger.Warn("Dispatcher: PullSubscribe failed, retrying in 2s...", "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			continue
		}
	}

	logger.Info("Dispatcher listening", "subject", subject, "durable", durableName)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msgs, err := sub.Fetch(1, nats.Context(ctx))
			if err != nil {
				if errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					continue
				}
				logger.Error("Dispatcher fetch error", "err", err)
				time.Sleep(time.Second)
				continue
			}

			for _, m := range msgs {
				if err := h.processMessage(ctx, m); err != nil {
					logger.Error("Dispatcher process error (redelivering)", "err", err)
					m.Nak()
				} else {
					m.Ack()
				}
			}
		}
	}
}

func (h *DispatcherHandler) processMessage(ctx context.Context, m *nats.Msg) error {
	var tx types.Transaction
	if err := json.Unmarshal(m.Data, &tx); err != nil {
		return fmt.Errorf("unmarshal transaction: %w", err)
	}

	logger.Info("Processing tx",
		"txHash", tx.TxHash,
		"network", tx.NetworkId,
		"to", tx.ToAddress,
		"amount", tx.Amount,
	)

	// ── Lookup the business that owns this destination address ────────────────
	biz, err := h.businessLookup.FindByAddress(ctx, tx.ToAddress, string(tx.NetworkId))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Address not associated with any business — this can happen when the
			// bloom filter has a false positive. Drop cleanly (Ack so it's not
			// redelivered forever).
			logger.Info("Dispatcher: no business found for address, skipping",
				"address", tx.ToAddress,
				"network", tx.NetworkId,
			)
			return nil
		}
		return fmt.Errorf("business lookup: %w", err)
	}

	if !biz.Active {
		logger.Info("Dispatcher: business is inactive, skipping", "businessId", biz.BusinessID)
		return nil
	}

	// ── Build the webhook payload ─────────────────────────────────────────────
	payload := WebhookPayload{
		EventID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		EventType:      "payment.confirmed",
		BusinessID:     biz.BusinessID,
		WebhookVersion: "1.0",
		SentAt:         time.Now(),
		Transaction:    tx,
		Settlement: SettlementInfo{
			Status: "pending",
		},
	}

	// ── Dispatch with per-business secret ─────────────────────────────────────
	return h.dispatchWithRetry(biz, payload)
}

func (h *DispatcherHandler) dispatchWithRetry(biz *model.Business, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	secret := []byte(biz.WebhookSecret)
	backoff := []time.Duration{2 * time.Second, 8 * time.Second, 30 * time.Second, 2 * time.Minute}

	for attempt := 0; attempt <= len(backoff); attempt++ {
		req, err := http.NewRequest(http.MethodPost, biz.WebhookURL, bytes.NewBuffer(body))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		AttachSignature(req, secret, body)

		resp, err := h.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				logger.Info("Webhook delivered",
					slog.String("businessId", biz.BusinessID),
					slog.Int("status", resp.StatusCode),
					slog.Int("attempt", attempt+1),
				)
				return nil
			}
			logger.Warn("Webhook non-2xx response",
				"businessId", biz.BusinessID,
				"status", resp.StatusCode,
				"attempt", attempt+1,
			)
		} else {
			logger.Warn("Webhook delivery error",
				"businessId", biz.BusinessID,
				"err", err,
				"attempt", attempt+1,
			)
		}

		if attempt < len(backoff) {
			time.Sleep(backoff[attempt])
		}
	}

	return fmt.Errorf("all webhook retries exhausted for business %s", biz.BusinessID)
}
