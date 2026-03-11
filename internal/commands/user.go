package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleUser handles user lookup commands.
func HandleUser(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: user <show|lookup> @user@domain")
	}

	switch ctx.Args[0] {
	case "show":
		return userShow(ctx)
	case "lookup":
		return userLookup(ctx)
	default:
		return fmt.Errorf("unknown user subcommand: %s", ctx.Args[0])
	}
}

func userShow(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: user show @user@domain")
	}
	return profileShow(ctx, ctx.Args[1])
}

func userLookup(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: user lookup @user@domain")
	}

	handle := strings.TrimPrefix(ctx.Args[1], "@")
	parts := strings.SplitN(handle, "@", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid handle: use @user@domain format")
	}
	username, domain := parts[0], parts[1]

	if domain == ctx.Config.Domain {
		return fmt.Errorf("use 'profile show @%s' for local users", username)
	}

	fmt.Fprintf(ctx.W, "Looking up @%s@%s via WebFinger...\n", username, domain)

	actor, err := fetchRemoteActor(ctx, username, domain)
	if err != nil {
		return fmt.Errorf("lookup failed: %w", err)
	}

	if err := ctx.DB.CreateRemoteActor(ctx.Ctx, actor); err != nil {
		return fmt.Errorf("store actor: %w", err)
	}

	displayName := actor.DisplayName
	if displayName == "" {
		displayName = actor.Username
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Found and cached @%s@%s\033[0m\n", actor.Username, actor.Domain)
	fmt.Fprintf(ctx.W, "\033[1m@%s@%s\033[0m\n", actor.Username, actor.Domain)
	fmt.Fprintf(ctx.W, "Display Name: %s\n", displayName)
	fmt.Fprintf(ctx.W, "Bio: %s\n", actor.Bio)
	if actor.AvatarURL != "" {
		fmt.Fprintf(ctx.W, "Avatar: %s\n", actor.AvatarURL)
	}
	fmt.Fprintf(ctx.W, "AP ID: %s\n\n", actor.APID)
	return nil
}

// fetchRemoteActor performs WebFinger + actor fetch for a remote user.
func fetchRemoteActor(ctx *Context, username, domain string) (*models.RemoteActor, error) {
	wfURL := fmt.Sprintf("https://%s/.well-known/webfinger?resource=acct:%s@%s", domain, username, domain)

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequestWithContext(ctx.Ctx, "GET", wfURL, nil)
	req.Header.Set("Accept", "application/jrd+json, application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webfinger request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webfinger returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read webfinger: %w", err)
	}

	var wf struct {
		Links []struct {
			Rel  string `json:"rel"`
			Type string `json:"type"`
			Href string `json:"href"`
		} `json:"links"`
	}
	if err := json.Unmarshal(body, &wf); err != nil {
		return nil, fmt.Errorf("parse webfinger: %w", err)
	}

	var actorURL string
	for _, link := range wf.Links {
		if link.Rel == "self" {
			actorURL = link.Href
			break
		}
	}
	if actorURL == "" {
		return nil, fmt.Errorf("no self link in WebFinger response")
	}

	req2, _ := http.NewRequestWithContext(ctx.Ctx, "GET", actorURL, nil)
	req2.Header.Set("Accept", "application/activity+json, application/ld+json")

	resp2, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("fetch actor: %w", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(io.LimitReader(resp2.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read actor: %w", err)
	}

	var actor struct {
		ID                string `json:"id"`
		PreferredUsername string `json:"preferredUsername"`
		Name              string `json:"name"`
		Summary           string `json:"summary"`
		Icon              struct {
			URL string `json:"url"`
		} `json:"icon"`
		Inbox     string `json:"inbox"`
		Outbox    string `json:"outbox"`
		Followers string `json:"followers"`
		Following string `json:"following"`
		PublicKey struct {
			PublicKeyPem string `json:"publicKeyPem"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(body2, &actor); err != nil {
		return nil, fmt.Errorf("parse actor: %w", err)
	}

	return &models.RemoteActor{
		ID:           uuid.New(),
		Username:     actor.PreferredUsername,
		Domain:       domain,
		DisplayName:  actor.Name,
		Bio:          actor.Summary,
		AvatarURL:    actor.Icon.URL,
		APID:         actor.ID,
		InboxURL:     actor.Inbox,
		OutboxURL:    actor.Outbox,
		PublicKey:    actor.PublicKey.PublicKeyPem,
		FollowersURL: actor.Followers,
		FollowingURL: actor.Following,
		FetchedAt:    time.Now(),
	}, nil
}
