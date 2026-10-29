package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"

	"feedchamber/internal/model"
)

// GitHubEndpoint is the OAuth2 endpoint for GitHub.
var GitHubEndpoint = oauth2.Endpoint{
	AuthURL:  "https://github.com/login/oauth/authorize",
	TokenURL: "https://github.com/login/oauth/access_token",
}

// fetchGitHubUser fetches the authenticated user's profile from GitHub.
func fetchGitHubUser(ctx context.Context, token *oauth2.Token) (*model.OAuthUser, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("github user request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("github user read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user API returned status %d: %s", resp.StatusCode, string(body))
	}

	var u struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("github user parse failed: %w", err)
	}

	// If email is empty, try fetching from /user/emails.
	if u.Email == "" {
		u.Email = fetchGitHubPrimaryEmail(ctx, client)
	}

	return &model.OAuthUser{
		ID:       fmt.Sprintf("%d", u.ID),
		Name:     u.Name,
		Email:    u.Email,
		Avatar:   u.AvatarURL,
		Provider: string(model.ProviderGitHub),
	}, nil
}

// fetchGitHubPrimaryEmail fetches the primary email from the GitHub emails API.
func fetchGitHubPrimaryEmail(ctx context.Context, client *http.Client) string {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return ""
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	if len(emails) > 0 {
		return emails[0].Email
	}
	return ""
}
