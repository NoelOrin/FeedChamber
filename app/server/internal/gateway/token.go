package gateway

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
)

// OAuth2/OIDC access tokens are opaque bearer strings: the server keeps only a
// SHA-256 digest, so a database leak does not hand out live credentials and
// revocation is a row update rather than a JWT blacklist.
//
// JWT survives in exactly one place — the OIDC id_token, which the spec
// requires to be a signed JWT (see sso.go).
const oauthAccessTokenPrefix = "of_at_"

var (
	errOAuthTokenNotFound = errors.New("oauth token not found")
	errOAuthTokenRevoked  = errors.New("oauth token revoked")
	errOAuthTokenExpired  = errors.New("oauth token expired")
)

// newOAuthAccessToken mints an opaque access token. The plaintext goes to the
// client; only the digest is stored.
func newOAuthAccessToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = oauthAccessTokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return raw, hashOAuthToken(raw), nil
}

// hashOAuthToken is the single place that maps a presented token to its stored
// digest; changing it invalidates every issued token, so treat it as contract.
func hashOAuthToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// lookupOAuthAccessToken resolves a presented token to a usable row. Expiry and
// revocation are filtered here so callers never have to re-check them.
func lookupOAuthAccessToken(ctx context.Context, db *gorm.DB, raw string) (*model.OAuthToken, error) {
	if db == nil || raw == "" {
		return nil, errOAuthTokenNotFound
	}
	var row model.OAuthToken
	err := db.WithContext(ctx).
		Where("access_token = ?", hashOAuthToken(raw)).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errOAuthTokenNotFound
		}
		return nil, err
	}
	if row.Revoked {
		return nil, errOAuthTokenRevoked
	}
	if !row.ExpiresAt.IsZero() && time.Now().After(row.ExpiresAt) {
		return nil, errOAuthTokenExpired
	}
	return &row, nil
}

// storeOAuthAccessToken mints a token and records its digest, returning the
// plaintext for the token response. Shared by the OAuth2 provider and OIDC SSO.
func storeOAuthAccessToken(ctx context.Context, db *gorm.DB, clientID, userID, scope string) (string, error) {
	raw, hash, err := newOAuthAccessToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	row := &model.OAuthToken{
		ClientID:  clientID,
		UserID:    userID,
		TokenHash: hash,
		Scope:     scope,
		ExpiresAt: now.Add(pkg.OAuthAccessTokenTTL),
		CreatedAt: now,
	}
	if err := db.WithContext(ctx).Create(row).Error; err != nil {
		return "", err
	}
	return raw, nil
}

// issueOAuthAccessToken is the OAuthProvider-bound convenience wrapper.
func (p *OAuthProvider) issueOAuthAccessToken(ctx context.Context, clientID, userID, scope string) (string, error) {
	return storeOAuthAccessToken(ctx, p.db, clientID, userID, scope)
}

// oauthClientRole is the pseudo-role projected onto request context for
// client_credentials tokens (they have no user behind them).
const oauthClientRole = "oauth_client"
