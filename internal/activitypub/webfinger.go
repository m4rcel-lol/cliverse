package activitypub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
)

// WebFingerHandler handles GET /.well-known/webfinger requests.
func WebFingerHandler(cfg *config.Config, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resource := r.URL.Query().Get("resource")
		if resource == "" {
			http.Error(w, "missing resource parameter", http.StatusBadRequest)
			return
		}

		// resource is expected to be "acct:username@domain"
		username, err := parseAcctResource(resource, cfg.Domain)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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

		resp := WebFingerResponse{
			Subject: fmt.Sprintf("acct:%s@%s", user.Username, cfg.Domain),
			Links: []WebFingerLink{
				{
					Rel:  "self",
					Type: "application/activity+json",
					Href: fmt.Sprintf("https://%s/users/%s", cfg.Domain, user.Username),
				},
			},
		}

		w.Header().Set("Content-Type", "application/jrd+json")
		json.NewEncoder(w).Encode(resp)
	}
}

// parseAcctResource extracts the username from an "acct:user@domain" resource string,
// validating that the domain matches the local instance.
func parseAcctResource(resource, localDomain string) (string, error) {
	resource = strings.TrimPrefix(resource, "acct:")
	parts := strings.SplitN(resource, "@", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid acct resource: %s", resource)
	}
	username := parts[0]
	domain := parts[1]
	if domain != localDomain {
		return "", fmt.Errorf("resource domain %q does not match local domain %q", domain, localDomain)
	}
	return username, nil
}
