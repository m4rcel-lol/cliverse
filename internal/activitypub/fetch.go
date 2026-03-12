package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// FetchActor fetches and parses a remote ActivityPub actor.
func FetchActor(ctx context.Context, actorURL string) (*Actor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorURL, nil)
	if err != nil {
		return nil, fmt.Errorf("activitypub/fetch: create request: %w", err)
	}
	req.Header.Set("Accept", "application/activity+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("activitypub/fetch: GET %s: %w", actorURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("activitypub/fetch: actor %s returned %d", actorURL, resp.StatusCode)
	}

	var actor Actor
	if err := json.NewDecoder(resp.Body).Decode(&actor); err != nil {
		return nil, fmt.Errorf("activitypub/fetch: decode actor: %w", err)
	}
	return &actor, nil
}

// FetchNote fetches and parses a remote ActivityPub Note.
func FetchNote(ctx context.Context, noteURL string) (*Note, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, noteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("activitypub/fetch: create request: %w", err)
	}
	req.Header.Set("Accept", "application/activity+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("activitypub/fetch: GET %s: %w", noteURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("activitypub/fetch: note %s returned %d", noteURL, resp.StatusCode)
	}

	var note Note
	if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
		return nil, fmt.Errorf("activitypub/fetch: decode note: %w", err)
	}
	return &note, nil
}

// WebFinger performs a WebFinger lookup for username@domain and returns the
// discovered actor.
func WebFinger(ctx context.Context, username, domain string) (*Actor, error) {
	wfURL := fmt.Sprintf("https://%s/.well-known/webfinger?resource=%s",
		domain, url.QueryEscape("acct:"+username+"@"+domain))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wfURL, nil)
	if err != nil {
		return nil, fmt.Errorf("activitypub/webfinger: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("activitypub/webfinger: GET %s: %w", wfURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("activitypub/webfinger: %s returned %d", wfURL, resp.StatusCode)
	}

	var wf WebFingerResponse
	if err := json.NewDecoder(resp.Body).Decode(&wf); err != nil {
		return nil, fmt.Errorf("activitypub/webfinger: decode response: %w", err)
	}

	// Find the self link with type application/activity+json.
	var actorURL string
	for _, link := range wf.Links {
		if link.Rel == "self" && link.Type == "application/activity+json" {
			actorURL = link.Href
			break
		}
	}
	if actorURL == "" {
		return nil, fmt.Errorf("activitypub/webfinger: no activity+json self link for %s@%s", username, domain)
	}

	return FetchActor(ctx, actorURL)
}
