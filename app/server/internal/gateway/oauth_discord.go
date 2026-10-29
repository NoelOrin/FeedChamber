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

// DiscordEndpoint is the OAuth2 endpoint for Discord.
var DiscordEndpoint = oauth2.Endpoint{
	AuthURL:  "https://discord.com/api/oauth2/authorize",
	TokenURL: "https://discord.com/api/oauth2/token",
}

// fetchDiscordUser fetches the authenticated user's profile from Discord.
func fetchDiscordUser(ctx context.Context, token *oauth2.Token) (*model.OAuthUser, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	resp, err := client.Get("https://discord.com/api/users/@me")
	if err != nil {
		return nil, fmt.Errorf("discord user request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("discord user read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord user API returned status %d: %s", resp.StatusCode, string(body))
	}

	var u struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Avatar   string `json:"avatar"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("discord user parse failed: %w", err)
	}

	// Construct avatar URL from hash.
	avatarURL := ""
	if u.Avatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", u.ID, u.Avatar)
	}

	return &model.OAuthUser{
		ID:       u.ID,
		Name:     u.Username,
		Email:    u.Email,
		Avatar:   avatarURL,
		Provider: string(model.ProviderDiscord),
	}, nil
}
