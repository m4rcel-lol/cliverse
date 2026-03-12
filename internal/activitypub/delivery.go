package activitypub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/db"
	"github.com/m4rcel-lol/cliverse/internal/models"
	"go.uber.org/zap"
)

// Deliverer handles outbound ActivityPub delivery to remote inboxes.
type Deliverer struct {
	db     *db.DB
	logger *zap.Logger
}

// NewDeliverer creates a new Deliverer.
func NewDeliverer(database *db.DB, logger *zap.Logger) *Deliverer {
	return &Deliverer{db: database, logger: logger}
}

// deliveryClient is a shared HTTP client used for outbound ActivityPub
// deliveries. It has a longer timeout than fetchClient because delivery targets
// may be slow to respond.
var deliveryClient = &http.Client{Timeout: 30 * time.Second}

// Deliver sends an activity to a single remote inbox URL, signing the request
// with the actor's private key. On transient failure the delivery is queued for
// retry via the DB.
func (d *Deliverer) Deliver(ctx context.Context, activity interface{}, actorUser *models.User, inboxURL string) error {
	body, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("delivery: marshal activity: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, inboxURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("delivery: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", req.URL.Host)

	AddDigest(req, body)

	keyID := fmt.Sprintf("https://%s/users/%s#main-key", actorUser.Domain, actorUser.Username)
	if err := Sign(req, keyID, actorUser.PrivateKey); err != nil {
		return fmt.Errorf("delivery: sign request: %w", err)
	}

	resp, err := deliveryClient.Do(req)
	if err != nil {
		d.logger.Warn("delivery failed, queuing retry",
			zap.String("inbox", inboxURL),
			zap.Error(err),
		)
		return d.QueueDelivery(ctx, string(body), inboxURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		d.logger.Warn("delivery non-2xx, queuing retry",
			zap.String("inbox", inboxURL),
			zap.Int("status", resp.StatusCode),
		)
		return d.QueueDelivery(ctx, string(body), inboxURL)
	}

	d.logger.Debug("delivery success", zap.String("inbox", inboxURL))
	return nil
}

// DeliverToFollowers delivers an activity to all remote followers of actorUser.
func (d *Deliverer) DeliverToFollowers(ctx context.Context, activity interface{}, actorUser *models.User) error {
	// Query remote actors that follow this local user via the follows table.
	rows, err := d.db.Pool().Query(ctx, `
		SELECT ra.inbox_url
		FROM remote_actors ra
		JOIN follows f ON f.follower_id = ra.id
		WHERE f.following_id = $1 AND f.state = 'accepted'`,
		actorUser.ID)
	if err != nil {
		return fmt.Errorf("delivery: list remote followers: %w", err)
	}
	defer rows.Close()

	var inboxes []string
	for rows.Next() {
		var inboxURL string
		if err := rows.Scan(&inboxURL); err != nil {
			return fmt.Errorf("delivery: scan inbox url: %w", err)
		}
		inboxes = append(inboxes, inboxURL)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("delivery: rows error: %w", err)
	}

	for _, inboxURL := range inboxes {
		if err := d.Deliver(ctx, activity, actorUser, inboxURL); err != nil {
			d.logger.Error("deliver to follower failed",
				zap.String("inbox", inboxURL),
				zap.Error(err),
			)
		}
	}
	return nil
}

// QueueDelivery persists a delivery in the DB so the federation worker can
// retry it later.
func (d *Deliverer) QueueDelivery(ctx context.Context, payload string, recipientURL string) error {
	fd := &models.FederationDelivery{
		ID:           uuid.New(),
		RecipientURL: recipientURL,
		Payload:      payload,
		Attempts:     0,
		Status:       models.DeliveryPending,
		CreatedAt:    time.Now(),
	}
	return d.db.CreateDelivery(ctx, fd)
}
