package federation

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/m4rcel-lol/cliverse/internal/activitypub"
	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
	"github.com/m4rcel-lol/cliverse/internal/models"
	"go.uber.org/zap"
)

// workerHTTPClient is a shared HTTP client for worker delivery attempts.
var workerHTTPClient = &http.Client{Timeout: 30 * time.Second}

// Worker polls the DB for pending federation deliveries and inbox events.
type Worker struct {
	db        *db.DB
	cfg       *config.Config
	logger    *zap.Logger
	processor *activitypub.InboxProcessor
}

// NewWorker creates a new Worker.
func NewWorker(database *db.DB, cfg *config.Config, logger *zap.Logger, processor *activitypub.InboxProcessor) *Worker {
	return &Worker{db: database, cfg: cfg, logger: logger, processor: processor}
}

// Start runs the worker loops until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	deliveryTicker := time.NewTicker(10 * time.Second)
	inboxTicker := time.NewTicker(5 * time.Second)
	defer deliveryTicker.Stop()
	defer inboxTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-deliveryTicker.C:
			w.processDeliveries(ctx)
		case <-inboxTicker.C:
			w.processInboxEvents(ctx)
		}
	}
}

// processDeliveries handles pending outbound federation deliveries.
func (w *Worker) processDeliveries(ctx context.Context) {
	deliveries, err := w.db.GetPendingDeliveries(ctx, 50)
	if err != nil {
		w.logger.Error("worker: get pending deliveries", zap.Error(err))
		return
	}

	for _, fd := range deliveries {
		w.attemptDelivery(ctx, fd)
	}
}

func (w *Worker) attemptDelivery(ctx context.Context, fd *models.FederationDelivery) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fd.RecipientURL,
		bytes.NewBufferString(fd.Payload))
	if err != nil {
		w.logger.Error("worker: build request", zap.String("url", fd.RecipientURL), zap.Error(err))
		w.markFailed(ctx, fd)
		return
	}
	req.Header.Set("Content-Type", "application/activity+json")

	resp, err := workerHTTPClient.Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			resp.Body.Close()
		}
		fd.Attempts++
		if fd.Attempts >= db.MaxDeliveryAttempts {
			w.markFailed(ctx, fd)
		} else {
			if dbErr := w.db.UpdateDeliveryStatus(ctx, fd.ID, models.DeliveryPending, fd.Attempts); dbErr != nil {
				w.logger.Error("worker: update delivery attempts", zap.Error(dbErr))
			}
		}
		return
	}
	resp.Body.Close()

	if dbErr := w.db.UpdateDeliveryStatus(ctx, fd.ID, models.DeliveryDelivered, fd.Attempts+1); dbErr != nil {
		w.logger.Error("worker: mark delivered", zap.Error(dbErr))
	}
}

func (w *Worker) markFailed(ctx context.Context, fd *models.FederationDelivery) {
	if err := w.db.UpdateDeliveryStatus(ctx, fd.ID, models.DeliveryFailed, fd.Attempts); err != nil {
		w.logger.Error("worker: mark failed", zap.Error(err))
	}
}

// processInboxEvents handles queued inbox events.
func (w *Worker) processInboxEvents(ctx context.Context) {
	events, err := w.db.GetUnprocessedInboxEvents(ctx, 50)
	if err != nil {
		w.logger.Error("worker: get inbox events", zap.Error(err))
		return
	}

	for _, event := range events {
		errMsg := ""
		if err := w.processor.ProcessEvent(ctx, event); err != nil {
			w.logger.Error("worker: process inbox event",
				zap.String("id", event.ID.String()),
				zap.String("type", event.ActivityType),
				zap.Error(err),
			)
			errMsg = err.Error()
		}
		if dbErr := w.db.MarkInboxEventProcessed(ctx, event.ID, errMsg); dbErr != nil {
			w.logger.Error("worker: mark event processed", zap.Error(dbErr))
		}
	}
}
