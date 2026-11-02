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

// GoogleEndpoint is the OAuth2 endpoint for Google.
var GoogleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// fetchGoogleUser fetches the authenticated user's profile from Google.
func fetchGoogleUser(ctx context.Context, token *oauth2.Token) (*model.OAuthUser, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google user request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("google user read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google user info API returned status %d: %s", resp.StatusCode, string(body))
	}

	var u struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("google user parse failed: %w", err)
	}

	return &model.OAuthUser{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		Avatar:   u.Picture,
		Provider: string(model.ProviderGoogle),
	}, nil
}
