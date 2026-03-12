package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
)

// nodeInfoWellKnown is the /.well-known/nodeinfo discovery document.
type nodeInfoWellKnown struct {
	Links []nodeInfoLink `json:"links"`
}

type nodeInfoLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

// nodeInfo20 is the NodeInfo 2.0 document.
type nodeInfo20 struct {
	Version           string            `json:"version"`
	Software          nodeInfoSoftware  `json:"software"`
	Protocols         []string          `json:"protocols"`
	Usage             nodeInfoUsage     `json:"usage"`
	OpenRegistrations bool              `json:"openRegistrations"`
	Services          nodeInfoServices  `json:"services"`
	Metadata          map[string]string `json:"metadata"`
}

type nodeInfoSoftware struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type nodeInfoUsage struct {
	Users         nodeInfoUsers `json:"users"`
	LocalPosts    int           `json:"localPosts"`
	LocalComments int           `json:"localComments"`
}

type nodeInfoUsers struct {
	Total          int `json:"total"`
	ActiveHalfYear int `json:"activeHalfyear"`
	ActiveMonth    int `json:"activeMonth"`
}

type nodeInfoServices struct {
	Inbound  []string `json:"inbound"`
	Outbound []string `json:"outbound"`
}

// NodeInfoWellKnownHandler handles GET /.well-known/nodeinfo and returns a
// discovery document pointing to the NodeInfo 2.0 endpoint.
func NodeInfoWellKnownHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		doc := nodeInfoWellKnown{
			Links: []nodeInfoLink{
				{
					Rel:  "http://nodeinfo.diaspora.software/ns/schema/2.0",
					Href: fmt.Sprintf("https://%s/nodeinfo/2.0", cfg.Domain),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}
}

// NodeInfoHandler handles GET /nodeinfo/2.0 and returns instance statistics.
func NodeInfoHandler(cfg *config.Config, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()

		userCount, _ := database.CountLocalUsers(ctx, cfg.Domain)
		postCount, _ := database.CountLocalPosts(ctx)

		regOpen, _ := database.GetSystemConfig(ctx, "registration_open")
		openReg := regOpen == "true"

		doc := nodeInfo20{
			Version: "2.0",
			Software: nodeInfoSoftware{
				Name:    "cliverse",
				Version: "1.0.0",
			},
			Protocols: []string{"activitypub"},
			Usage: nodeInfoUsage{
				Users: nodeInfoUsers{
					Total: userCount,
				},
				LocalPosts: postCount,
			},
			OpenRegistrations: openReg,
			Services: nodeInfoServices{
				Inbound:  []string{},
				Outbound: []string{},
			},
			Metadata: map[string]string{
				"nodeName":        cfg.InstanceName,
				"nodeDescription": cfg.InstanceDesc,
			},
		}

		w.Header().Set("Content-Type", "application/json; profile=\"http://nodeinfo.diaspora.software/ns/schema/2.0#\"")
		json.NewEncoder(w).Encode(doc)
	}
}
