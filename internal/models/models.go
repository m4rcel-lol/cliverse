package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID
	Username        string
	Domain          string
	DisplayName     string
	Bio             string
	AvatarURL       string
	BannerURL       string
	PasswordHash    string
	PrivateKey      string // RSA private key PEM for ActivityPub HTTP signatures
	PublicKey       string // RSA public key PEM
	APID            string // ActivityPub ID URL
	InboxURL        string
	OutboxURL       string
	IsAdmin         bool
	IsLocked        bool
	IsSilenced      bool
	ForcePassChange bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SSHKey struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	PublicKey   string
	Fingerprint string
	CreatedAt   time.Time
}

type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	RemoteAddr string
	CreatedAt  time.Time
	LastSeenAt time.Time
	Ended      bool
}

type Post struct {
	ID            uuid.UUID
	LocalID       string
	AuthorID      uuid.UUID
	Content       string
	Visibility    string // public, unlisted, followers, direct
	InReplyToID   *uuid.UUID
	ActivityPubID string
	RemoteID      *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Deleted       bool
}

type Follow struct {
	ID                uuid.UUID
	FollowerID        uuid.UUID
	FollowingID       *uuid.UUID
	FollowingRemoteID *uuid.UUID
	State             string // pending, accepted
	CreatedAt         time.Time
}

type Like struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	PostID    uuid.UUID
	APID      string
	CreatedAt time.Time
}

type Boost struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	PostID    uuid.UUID
	APID      string
	CreatedAt time.Time
}

type Bookmark struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	PostID    uuid.UUID
	CreatedAt time.Time
}

type Notification struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Type          string // follow, mention, like, boost, reply
	ActorID       *uuid.UUID
	RemoteActorID *uuid.UUID
	PostID        *uuid.UUID
	Read          bool
	CreatedAt     time.Time
}

type RemoteActor struct {
	ID           uuid.UUID
	Username     string
	Domain       string
	DisplayName  string
	Bio          string
	AvatarURL    string
	APID         string
	InboxURL     string
	OutboxURL    string
	PublicKey    string
	FollowersURL string
	FollowingURL string
	FetchedAt    time.Time
}

type FederationDelivery struct {
	ID           uuid.UUID
	RecipientURL string
	Payload      string
	Attempts     int
	LastAttempt  *time.Time
	Status       string // pending, delivered, failed
	CreatedAt    time.Time
}

type InboxEvent struct {
	ID           uuid.UUID
	SenderAPID   string
	ActivityType string
	Payload      string
	Processed    bool
	Error        string
	CreatedAt    time.Time
}

type DomainPolicy struct {
	ID        uuid.UUID
	Domain    string
	Action    string // block, allow, silence
	Reason    string
	CreatedAt time.Time
}

type Report struct {
	ID           uuid.UUID
	ReporterID   uuid.UUID
	TargetUserID *uuid.UUID
	TargetPostID *uuid.UUID
	Reason       string
	Status       string // open, resolved, dismissed
	Notes        string
	CreatedAt    time.Time
	ResolvedAt   *time.Time
}

type AuditLog struct {
	ID        uuid.UUID
	ActorID   *uuid.UUID
	Action    string
	Target    string
	Details   string // JSON
	IPAddr    string
	CreatedAt time.Time
}

type Draft struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Content    string
	Visibility string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type SystemConfig struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}

// Visibility constants
const (
	VisibilityPublic    = "public"
	VisibilityUnlisted  = "unlisted"
	VisibilityFollowers = "followers"
	VisibilityDirect    = "direct"
)

// Follow state constants
const (
	FollowStatePending  = "pending"
	FollowStateAccepted = "accepted"
)

// Notification type constants
const (
	NotifFollow  = "follow"
	NotifMention = "mention"
	NotifLike    = "like"
	NotifBoost   = "boost"
	NotifReply   = "reply"
)

// Delivery status constants
const (
	DeliveryPending   = "pending"
	DeliveryDelivered = "delivered"
	DeliveryFailed    = "failed"
)

// Report status constants
const (
	ReportOpen      = "open"
	ReportResolved  = "resolved"
	ReportDismissed = "dismissed"
)

// Domain policy actions
const (
	PolicyBlock   = "block"
	PolicyAllow   = "allow"
	PolicySilence = "silence"
)
