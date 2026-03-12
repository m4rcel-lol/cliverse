package activitypub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
)

// ActorHandler handles GET /users/{username} and returns the ActivityPub Actor JSON.
func ActorHandler(cfg *config.Config, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		if username == "" {
			http.Error(w, "missing username", http.StatusBadRequest)
			return
		}

		user, err := database.GetUserByUsername(r.Context(), username)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		base := fmt.Sprintf("https://%s/users/%s", cfg.Domain, user.Username)

		actor := Actor{
			Context:           DefaultContext(),
			ID:                base,
			Type:              "Person",
			Following:         base + "/following",
			Followers:         base + "/followers",
			Inbox:             base + "/inbox",
			Outbox:            base + "/outbox",
			PreferredUsername: user.Username,
			Name:              user.DisplayName,
			Summary:           user.Bio,
			URL:               base,
			PublicKey: &PublicKey{
				ID:           base + "#main-key",
				Owner:        base,
				PublicKeyPem: user.PublicKey,
			},
			ManuallyApprovesFollowers: user.IsLocked,
		}

		if user.AvatarURL != "" {
			actor.Icon = &Image{
				Type:      "Image",
				MediaType: mediaTypeFromURL(user.AvatarURL),
				URL:       user.AvatarURL,
			}
		}
		if user.BannerURL != "" {
			actor.Image = &Image{
				Type:      "Image",
				MediaType: mediaTypeFromURL(user.BannerURL),
				URL:       user.BannerURL,
			}
		}

		w.Header().Set("Content-Type", "application/activity+json")
		json.NewEncoder(w).Encode(actor)
	}
}

// OutboxHandler handles GET /users/{username}/outbox and returns an OrderedCollection.
func OutboxHandler(cfg *config.Config, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		user, err := database.GetUserByUsername(r.Context(), username)
		if err != nil || user == nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		base := fmt.Sprintf("https://%s/users/%s", cfg.Domain, user.Username)
		posts, err := database.ListPostsByUser(r.Context(), user.ID, 20, 0)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Build activity wrappers for each post.
		items := make([]interface{}, 0, len(posts))
		for _, p := range posts {
			noteID := fmt.Sprintf("https://%s/posts/%s", cfg.Domain, p.LocalID)
			note := Note{
				Context:      DefaultContext(),
				ID:           noteID,
				Type:         "Note",
				AttributedTo: base,
				Content:      p.Content,
				Published:    p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
				To:           []string{"https://www.w3.org/ns/activitystreams#Public"},
				CC:           []string{base + "/followers"},
				URL:          noteID,
			}
			activity := Activity{
				Context:   DefaultContext(),
				ID:        noteID + "/activity",
				Type:      "Create",
				Actor:     base,
				Object:    note,
				To:        note.To,
				CC:        note.CC,
				Published: note.Published,
			}
			items = append(items, activity)
		}

		coll := OrderedCollection{
			Context:      DefaultContext(),
			ID:           base + "/outbox",
			Type:         "OrderedCollection",
			TotalItems:   len(items),
			OrderedItems: items,
		}

		w.Header().Set("Content-Type", "application/activity+json")
		json.NewEncoder(w).Encode(coll)
	}
}

// FollowersHandler handles GET /users/{username}/followers.
func FollowersHandler(cfg *config.Config, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		user, err := database.GetUserByUsername(r.Context(), username)
		if err != nil || user == nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		base := fmt.Sprintf("https://%s/users/%s", cfg.Domain, user.Username)
		followers, err := database.ListFollowers(r.Context(), user.ID)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		items := make([]interface{}, 0, len(followers))
		for _, f := range followers {
			items = append(items, fmt.Sprintf("https://%s/users/%s", cfg.Domain, f.Username))
		}

		coll := OrderedCollection{
			Context:      DefaultContext(),
			ID:           base + "/followers",
			Type:         "OrderedCollection",
			TotalItems:   len(items),
			OrderedItems: items,
		}

		w.Header().Set("Content-Type", "application/activity+json")
		json.NewEncoder(w).Encode(coll)
	}
}

// FollowingHandler handles GET /users/{username}/following.
func FollowingHandler(cfg *config.Config, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		user, err := database.GetUserByUsername(r.Context(), username)
		if err != nil || user == nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		base := fmt.Sprintf("https://%s/users/%s", cfg.Domain, user.Username)
		following, err := database.ListFollowing(r.Context(), user.ID)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		items := make([]interface{}, 0, len(following))
		for _, f := range following {
			items = append(items, fmt.Sprintf("https://%s/users/%s", cfg.Domain, f.Username))
		}

		coll := OrderedCollection{
			Context:      DefaultContext(),
			ID:           base + "/following",
			Type:         "OrderedCollection",
			TotalItems:   len(items),
			OrderedItems: items,
		}

		w.Header().Set("Content-Type", "application/activity+json")
		json.NewEncoder(w).Encode(coll)
	}
}

// mediaTypeFromURL guesses the MIME type from a URL's file extension.
func mediaTypeFromURL(u string) string {
	lower := strings.ToLower(u)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
