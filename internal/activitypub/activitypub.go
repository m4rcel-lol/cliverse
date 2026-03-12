package activitypub

import "encoding/json"

// JSON-LD context URLs.
const (
	ASContext = "https://www.w3.org/ns/activitystreams"
	W3IDSec   = "https://w3id.org/security/v1"
)

// Actor represents an ActivityPub actor (Person, Service, etc.).
type Actor struct {
	Context                   interface{} `json:"@context"`
	ID                        string      `json:"id"`
	Type                      string      `json:"type"`
	Following                 string      `json:"following"`
	Followers                 string      `json:"followers"`
	Inbox                     string      `json:"inbox"`
	Outbox                    string      `json:"outbox"`
	PreferredUsername         string      `json:"preferredUsername"`
	Name                      string      `json:"name"`
	Summary                   string      `json:"summary"`
	URL                       string      `json:"url"`
	Icon                      *Image      `json:"icon,omitempty"`
	Image                     *Image      `json:"image,omitempty"`
	PublicKey                 *PublicKey  `json:"publicKey"`
	ManuallyApprovesFollowers bool        `json:"manuallyApprovesFollowers"`
}

// PublicKey holds an actor's RSA public key for HTTP signature verification.
type PublicKey struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	PublicKeyPem string `json:"publicKeyPem"`
}

// Image represents an icon or header image attached to an actor.
type Image struct {
	Type      string `json:"type"`
	MediaType string `json:"mediaType,omitempty"`
	URL       string `json:"url"`
}

// Note represents an ActivityPub Note object (a post/status).
type Note struct {
	Context      interface{} `json:"@context"`
	ID           string      `json:"id"`
	Type         string      `json:"type"`
	AttributedTo string      `json:"attributedTo"`
	Content      string      `json:"content"`
	Published    string      `json:"published"`
	To           []string    `json:"to"`
	CC           []string    `json:"cc"`
	InReplyTo    *string     `json:"inReplyTo,omitempty"`
	URL          string      `json:"url"`
	Sensitive    bool        `json:"sensitive"`
}

// Activity represents an ActivityPub activity (Create, Follow, Like, etc.).
type Activity struct {
	Context   interface{} `json:"@context"`
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Actor     string      `json:"actor"`
	Object    interface{} `json:"object"`
	To        []string    `json:"to,omitempty"`
	CC        []string    `json:"cc,omitempty"`
	Published string      `json:"published,omitempty"`
}

// OrderedCollection is used for outbox, followers, and following endpoints.
type OrderedCollection struct {
	Context      interface{} `json:"@context"`
	ID           string      `json:"id"`
	Type         string      `json:"type"`
	TotalItems   int         `json:"totalItems"`
	OrderedItems interface{} `json:"orderedItems"`
	First        string      `json:"first,omitempty"`
}

// WebFingerResponse is the JSON response for .well-known/webfinger.
type WebFingerResponse struct {
	Subject string          `json:"subject"`
	Links   []WebFingerLink `json:"links"`
}

// WebFingerLink is a single link entry in a WebFinger response.
type WebFingerLink struct {
	Rel  string `json:"rel"`
	Type string `json:"type,omitempty"`
	Href string `json:"href,omitempty"`
}

// DefaultContext returns the standard ActivityPub JSON-LD context array.
func DefaultContext() []interface{} {
	return []interface{}{ASContext, W3IDSec}
}

// IncomingActivity is used to parse an incoming ActivityPub activity where the
// Object field may be a string ID or a full embedded object.
type IncomingActivity struct {
	Context interface{}     `json:"@context"`
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Actor   string          `json:"actor"`
	Object  json.RawMessage `json:"object"`
	To      []string        `json:"to,omitempty"`
	CC      []string        `json:"cc,omitempty"`
}
