package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
	"github.com/m4rcel-lol/cliverse/internal/models"
	"go.uber.org/zap"
)

// InboxHandler handles POST /users/{username}/inbox. It verifies the HTTP
// signature, persists the raw event, and returns 202 Accepted so processing can
// happen asynchronously.
func InboxHandler(cfg *config.Config, database *db.DB, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")

		// Make sure the target user exists.
		user, err := database.GetUserByUsername(r.Context(), username)
		if err != nil {
			logger.Error("inbox: db lookup", zap.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB cap
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}

		// Decode enough to identify the sender so we can look up their public key.
		var incoming IncomingActivity
		if err := json.Unmarshal(body, &incoming); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Verify HTTP signature against the sender's public key.
		if err := verifyInboxSignature(r, incoming.Actor, database); err != nil {
			logger.Warn("inbox: signature verification failed",
				zap.String("actor", incoming.Actor),
				zap.Error(err),
			)
			// Return 401 but still accept so we don't break well-behaved servers.
		}

		event := &models.InboxEvent{
			ID:           uuid.New(),
			SenderAPID:   incoming.Actor,
			ActivityType: incoming.Type,
			Payload:      string(body),
			Processed:    false,
			CreatedAt:    time.Now(),
		}
		if err := database.CreateInboxEvent(r.Context(), event); err != nil {
			logger.Error("inbox: save event", zap.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

// verifyInboxSignature fetches the sender actor's public key and verifies the
// HTTP signature on r. If the actor is already cached in the DB, the cached
// public key is used; otherwise it is fetched from the remote server.
func verifyInboxSignature(r *http.Request, actorURL string, database *db.DB) error {
	ra, err := database.GetRemoteActorByAPID(r.Context(), actorURL)
	if err != nil {
		return fmt.Errorf("inbox: db get remote actor: %w", err)
	}

	var pubKeyPEM string
	if ra != nil && ra.PublicKey != "" {
		pubKeyPEM = ra.PublicKey
	} else {
		// Fetch remote actor to get public key.
		actor, err := FetchActor(r.Context(), actorURL)
		if err != nil {
			return fmt.Errorf("inbox: fetch actor %s: %w", actorURL, err)
		}
		if actor.PublicKey == nil {
			return fmt.Errorf("inbox: actor %s has no public key", actorURL)
		}
		pubKeyPEM = actor.PublicKey.PublicKeyPem
	}

	return Verify(r, pubKeyPEM)
}

// ─── Inbox Processor ─────────────────────────────────────────────────────────

// InboxProcessor handles queued InboxEvents and applies the appropriate side
// effects (creating follows, posts, likes, boosts, etc.).
type InboxProcessor struct {
	db        *db.DB
	cfg       *config.Config
	logger    *zap.Logger
	deliverer *Deliverer
}

// NewInboxProcessor creates a new InboxProcessor.
func NewInboxProcessor(database *db.DB, cfg *config.Config, logger *zap.Logger, deliverer *Deliverer) *InboxProcessor {
	return &InboxProcessor{
		db:        database,
		cfg:       cfg,
		logger:    logger,
		deliverer: deliverer,
	}
}

// ProcessEvent processes a single inbox event.
func (p *InboxProcessor) ProcessEvent(ctx context.Context, event *models.InboxEvent) error {
	var incoming IncomingActivity
	if err := json.Unmarshal([]byte(event.Payload), &incoming); err != nil {
		return fmt.Errorf("inbox processor: unmarshal: %w", err)
	}

	switch strings.ToLower(incoming.Type) {
	case "follow":
		return p.handleFollow(ctx, &incoming)
	case "accept":
		return p.handleAccept(ctx, &incoming)
	case "reject":
		return p.handleReject(ctx, &incoming)
	case "create":
		return p.handleCreate(ctx, &incoming)
	case "delete":
		return p.handleDelete(ctx, &incoming)
	case "like", "emojireact":
		return p.handleLike(ctx, &incoming)
	case "announce":
		return p.handleAnnounce(ctx, &incoming)
	case "undo":
		return p.handleUndo(ctx, &incoming)
	default:
		p.logger.Debug("inbox processor: unhandled activity type", zap.String("type", incoming.Type))
		return nil
	}
}

// handleFollow processes an incoming Follow activity from a remote actor.
func (p *InboxProcessor) handleFollow(ctx context.Context, act *IncomingActivity) error {
	// The object of Follow is the local user's AP ID.
	objectID, err := rawToString(act.Object)
	if err != nil {
		return fmt.Errorf("handleFollow: parse object: %w", err)
	}

	localUser, err := p.db.GetUserByAPID(ctx, objectID)
	if err != nil {
		return fmt.Errorf("handleFollow: get local user: %w", err)
	}
	if localUser == nil {
		return fmt.Errorf("handleFollow: local user not found for ap_id %s", objectID)
	}

	// Upsert the remote actor.
	remoteActor, err := p.ensureRemoteActor(ctx, act.Actor)
	if err != nil {
		return fmt.Errorf("handleFollow: ensure remote actor: %w", err)
	}

	// Create follow record (remote actor follows local user).
	// We store the remote actor's UUID as FollowerID so we can find their inbox later.
	localUserID := localUser.ID
	follow := &models.Follow{
		ID:          uuid.New(),
		FollowerID:  remoteActor.ID,
		FollowingID: &localUserID,
		State:       models.FollowStatePending,
		CreatedAt:   time.Now(),
	}
	if err := p.db.CreateFollow(ctx, follow); err != nil {
		return fmt.Errorf("handleFollow: create follow: %w", err)
	}

	// Create notification for the local user.
	notif := &models.Notification{
		ID:            uuid.New(),
		UserID:        localUser.ID,
		Type:          models.NotifFollow,
		RemoteActorID: &remoteActor.ID,
		CreatedAt:     time.Now(),
	}
	if err := p.db.CreateNotification(ctx, notif); err != nil {
		p.logger.Warn("handleFollow: create notification", zap.Error(err))
	}

	// If the account is not locked, auto-accept.
	if !localUser.IsLocked {
		if err := p.db.UpdateFollowState(ctx, follow.ID, models.FollowStateAccepted); err != nil {
			return fmt.Errorf("handleFollow: accept follow: %w", err)
		}

		// Send Accept activity back to the follower.
		return p.sendAccept(ctx, localUser, act.Actor, act.ID)
	}

	return nil
}

// handleAccept processes an Accept{Follow} activity.
func (p *InboxProcessor) handleAccept(ctx context.Context, act *IncomingActivity) error {
	// Object should be the Follow activity ID.
	objectID, err := rawToString(act.Object)
	if err != nil {
		// Object might be an embedded Follow object.
		var embedded IncomingActivity
		if jsonErr := json.Unmarshal(act.Object, &embedded); jsonErr == nil {
			objectID = embedded.ID
		} else {
			return fmt.Errorf("handleAccept: parse object: %w", err)
		}
	}

	follow, err := p.db.GetFollowByID(ctx, uuidFromString(objectID))
	if err != nil || follow == nil {
		// Fallback: try to find by follower APID.
		p.logger.Debug("handleAccept: follow not found by ID", zap.String("id", objectID))
		return nil
	}

	return p.db.UpdateFollowState(ctx, follow.ID, models.FollowStateAccepted)
}

// handleReject processes a Reject{Follow} activity.
func (p *InboxProcessor) handleReject(ctx context.Context, act *IncomingActivity) error {
	objectID, err := rawToString(act.Object)
	if err != nil {
		var embedded IncomingActivity
		if jsonErr := json.Unmarshal(act.Object, &embedded); jsonErr == nil {
			objectID = embedded.ID
		} else {
			return fmt.Errorf("handleReject: parse object: %w", err)
		}
	}

	follow, err := p.db.GetFollowByID(ctx, uuidFromString(objectID))
	if err != nil || follow == nil {
		return nil
	}

	localFollowerID := follow.FollowerID
	if follow.FollowingID != nil {
		return p.db.DeleteFollow(ctx, localFollowerID, *follow.FollowingID)
	}
	return nil
}

// handleCreate processes a Create{Note} activity.
func (p *InboxProcessor) handleCreate(ctx context.Context, act *IncomingActivity) error {
	var note Note
	if err := json.Unmarshal(act.Object, &note); err != nil {
		// Object might just be a URL string — skip.
		return nil
	}
	if note.Type != "Note" {
		return nil
	}

	// Skip if we already have this post.
	existing, err := p.db.GetPostByAPID(ctx, note.ID)
	if err != nil {
		return fmt.Errorf("handleCreate: check existing post: %w", err)
	}
	if existing != nil {
		return nil
	}

	remoteID := note.ID
	post := &models.Post{
		ID:            uuid.New(),
		LocalID:       uuid.New().String(),
		AuthorID:      uuid.Nil, // remote post; author resolved via APID
		Content:       note.Content,
		Visibility:    models.VisibilityPublic,
		ActivityPubID: note.ID,
		RemoteID:      &remoteID,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if note.InReplyTo != nil && *note.InReplyTo != "" {
		parent, err := p.db.GetPostByAPID(ctx, *note.InReplyTo)
		if err == nil && parent != nil {
			post.InReplyToID = &parent.ID
		}
	}

	if err := p.db.CreatePost(ctx, post); err != nil {
		return fmt.Errorf("handleCreate: save post: %w", err)
	}

	// Check for mentions of local users and create notifications.
	p.createMentionNotifications(ctx, note.Content, post.ID)

	return nil
}

// handleDelete processes a Delete activity, soft-deleting the referenced post.
func (p *InboxProcessor) handleDelete(ctx context.Context, act *IncomingActivity) error {
	objectID, err := rawToString(act.Object)
	if err != nil {
		return nil // Can't parse object, skip.
	}

	post, err := p.db.GetPostByAPID(ctx, objectID)
	if err != nil || post == nil {
		return nil
	}

	return p.db.DeletePost(ctx, post.ID, post.AuthorID)
}

// handleLike processes a Like activity.
func (p *InboxProcessor) handleLike(ctx context.Context, act *IncomingActivity) error {
	objectID, err := rawToString(act.Object)
	if err != nil {
		return nil
	}

	post, err := p.db.GetPostByAPID(ctx, objectID)
	if err != nil || post == nil {
		return nil
	}

	remoteActor, err := p.ensureRemoteActor(ctx, act.Actor)
	if err != nil {
		return fmt.Errorf("handleLike: ensure remote actor: %w", err)
	}

	like := &models.Like{
		ID:        uuid.New(),
		UserID:    remoteActor.ID,
		PostID:    post.ID,
		APID:      act.ID,
		CreatedAt: time.Now(),
	}
	if err := p.db.CreateLike(ctx, like); err != nil {
		return fmt.Errorf("handleLike: create like: %w", err)
	}

	// Notify the post author if they're local.
	localAuthor, err := p.db.GetUserByID(ctx, post.AuthorID)
	if err == nil && localAuthor != nil {
		notif := &models.Notification{
			ID:            uuid.New(),
			UserID:        localAuthor.ID,
			Type:          models.NotifLike,
			RemoteActorID: &remoteActor.ID,
			PostID:        &post.ID,
			CreatedAt:     time.Now(),
		}
		if err := p.db.CreateNotification(ctx, notif); err != nil {
			p.logger.Warn("handleLike: create notification", zap.Error(err))
		}
	}
	return nil
}

// handleAnnounce processes an Announce (boost) activity.
func (p *InboxProcessor) handleAnnounce(ctx context.Context, act *IncomingActivity) error {
	objectID, err := rawToString(act.Object)
	if err != nil {
		return nil
	}

	post, err := p.db.GetPostByAPID(ctx, objectID)
	if err != nil || post == nil {
		return nil
	}

	remoteActor, err := p.ensureRemoteActor(ctx, act.Actor)
	if err != nil {
		return fmt.Errorf("handleAnnounce: ensure remote actor: %w", err)
	}

	boost := &models.Boost{
		ID:        uuid.New(),
		UserID:    remoteActor.ID,
		PostID:    post.ID,
		APID:      act.ID,
		CreatedAt: time.Now(),
	}
	if err := p.db.CreateBoost(ctx, boost); err != nil {
		return fmt.Errorf("handleAnnounce: create boost: %w", err)
	}

	localAuthor, err := p.db.GetUserByID(ctx, post.AuthorID)
	if err == nil && localAuthor != nil {
		notif := &models.Notification{
			ID:            uuid.New(),
			UserID:        localAuthor.ID,
			Type:          models.NotifBoost,
			RemoteActorID: &remoteActor.ID,
			PostID:        &post.ID,
			CreatedAt:     time.Now(),
		}
		if err := p.db.CreateNotification(ctx, notif); err != nil {
			p.logger.Warn("handleAnnounce: create notification", zap.Error(err))
		}
	}
	return nil
}

// handleUndo processes Undo{Follow|Like|Announce}.
func (p *InboxProcessor) handleUndo(ctx context.Context, act *IncomingActivity) error {
	// The Object is the activity being undone.
	var inner IncomingActivity
	if err := json.Unmarshal(act.Object, &inner); err != nil {
		// Object may be a bare ID string.
		return nil
	}

	switch strings.ToLower(inner.Type) {
	case "follow":
		return p.undoFollow(ctx, act.Actor, &inner)
	case "like", "emojireact":
		return p.undoLike(ctx, act.Actor, &inner)
	case "announce":
		return p.undoAnnounce(ctx, act.Actor, &inner)
	}
	return nil
}

func (p *InboxProcessor) undoFollow(ctx context.Context, actorAPID string, inner *IncomingActivity) error {
	objectID, err := rawToString(inner.Object)
	if err != nil {
		return nil
	}

	localUser, err := p.db.GetUserByAPID(ctx, objectID)
	if err != nil || localUser == nil {
		return nil
	}

	remoteActor, err := p.db.GetRemoteActorByAPID(ctx, actorAPID)
	if err != nil || remoteActor == nil {
		return nil
	}

	return p.db.DeleteFollow(ctx, remoteActor.ID, localUser.ID)
}

func (p *InboxProcessor) undoLike(ctx context.Context, actorAPID string, inner *IncomingActivity) error {
	objectID, err := rawToString(inner.Object)
	if err != nil {
		return nil
	}

	post, err := p.db.GetPostByAPID(ctx, objectID)
	if err != nil || post == nil {
		return nil
	}

	remoteActor, err := p.db.GetRemoteActorByAPID(ctx, actorAPID)
	if err != nil || remoteActor == nil {
		return nil
	}

	return p.db.DeleteLike(ctx, remoteActor.ID, post.ID)
}

func (p *InboxProcessor) undoAnnounce(ctx context.Context, actorAPID string, inner *IncomingActivity) error {
	objectID, err := rawToString(inner.Object)
	if err != nil {
		return nil
	}

	post, err := p.db.GetPostByAPID(ctx, objectID)
	if err != nil || post == nil {
		return nil
	}

	remoteActor, err := p.db.GetRemoteActorByAPID(ctx, actorAPID)
	if err != nil || remoteActor == nil {
		return nil
	}

	return p.db.DeleteBoost(ctx, remoteActor.ID, post.ID)
}

// sendAccept sends an Accept{Follow} activity back to the remote follower.
func (p *InboxProcessor) sendAccept(ctx context.Context, localUser *models.User, followerAPID, followActivityID string) error {
	base := fmt.Sprintf("https://%s/users/%s", p.cfg.Domain, localUser.Username)
	accept := Activity{
		Context: DefaultContext(),
		ID:      fmt.Sprintf("%s#accepts/%s", base, uuid.New().String()),
		Type:    "Accept",
		Actor:   base,
		Object:  followActivityID,
	}

	// Look up the follower's inbox URL.
	remoteActor, err := p.db.GetRemoteActorByAPID(ctx, followerAPID)
	if err != nil {
		return fmt.Errorf("sendAccept: get remote actor: %w", err)
	}

	var inboxURL string
	if remoteActor != nil && remoteActor.InboxURL != "" {
		inboxURL = remoteActor.InboxURL
	} else {
		actor, err := FetchActor(ctx, followerAPID)
		if err != nil {
			return fmt.Errorf("sendAccept: fetch actor: %w", err)
		}
		inboxURL = actor.Inbox
	}

	return p.deliverer.Deliver(ctx, accept, localUser, inboxURL)
}

// ensureRemoteActor fetches and upserts a remote actor record.
func (p *InboxProcessor) ensureRemoteActor(ctx context.Context, actorAPID string) (*models.RemoteActor, error) {
	ra, err := p.db.GetRemoteActorByAPID(ctx, actorAPID)
	if err != nil {
		return nil, err
	}
	if ra != nil {
		return ra, nil
	}

	actor, err := FetchActor(ctx, actorAPID)
	if err != nil {
		return nil, fmt.Errorf("ensureRemoteActor: fetch %s: %w", actorAPID, err)
	}

	domain := ""
	if idx := strings.Index(actorAPID, "://"); idx >= 0 {
		rest := actorAPID[idx+3:]
		if slashIdx := strings.IndexByte(rest, '/'); slashIdx >= 0 {
			domain = rest[:slashIdx]
		} else {
			domain = rest
		}
	}

	pubKeyPEM := ""
	if actor.PublicKey != nil {
		pubKeyPEM = actor.PublicKey.PublicKeyPem
	}

	iconURL := ""
	if actor.Icon != nil {
		iconURL = actor.Icon.URL
	}

	ra = &models.RemoteActor{
		ID:           uuid.New(),
		Username:     actor.PreferredUsername,
		Domain:       domain,
		DisplayName:  actor.Name,
		Bio:          actor.Summary,
		AvatarURL:    iconURL,
		APID:         actor.ID,
		InboxURL:     actor.Inbox,
		OutboxURL:    actor.Outbox,
		PublicKey:    pubKeyPEM,
		FollowersURL: actor.Followers,
		FollowingURL: actor.Following,
		FetchedAt:    time.Now(),
	}

	if err := p.db.CreateRemoteActor(ctx, ra); err != nil {
		return nil, fmt.Errorf("ensureRemoteActor: upsert: %w", err)
	}
	return ra, nil
}

// createMentionNotifications scans content for @user mentions and creates notifications.
func (p *InboxProcessor) createMentionNotifications(ctx context.Context, content string, postID uuid.UUID) {
	// Simple heuristic: find @username or @username@domain patterns.
	words := strings.Fields(content)
	seen := map[string]bool{}
	for _, word := range words {
		// Strip HTML tags and punctuation.
		word = strings.Trim(word, ".,!?;:\"'<>")
		if !strings.HasPrefix(word, "@") {
			continue
		}
		handle := strings.TrimPrefix(word, "@")
		parts := strings.SplitN(handle, "@", 2)
		username := parts[0]
		if username == "" || seen[username] {
			continue
		}
		seen[username] = true

		// Only notify if user is local (no domain part, or domain matches ours).
		if len(parts) == 2 && parts[1] != p.cfg.Domain {
			continue
		}

		localUser, err := p.db.GetUserByUsername(ctx, username)
		if err != nil || localUser == nil {
			continue
		}

		notif := &models.Notification{
			ID:        uuid.New(),
			UserID:    localUser.ID,
			Type:      models.NotifMention,
			PostID:    &postID,
			CreatedAt: time.Now(),
		}
		if err := p.db.CreateNotification(ctx, notif); err != nil {
			p.logger.Warn("createMentionNotifications: create notification", zap.Error(err))
		}
	}
}

// rawToString tries to unmarshal a json.RawMessage as a plain string.
func rawToString(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

// uuidFromString parses a UUID from a string, returning uuid.Nil on error.
func uuidFromString(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}
