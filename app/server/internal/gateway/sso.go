package gateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"sync"
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"
)

// ssoSigningKeySetting is the settings key holding the ID-token signing key in
// PEM form. Persisting it is required, not optional: a key regenerated on every
// boot rotates the JWKS, silently invalidating previously issued id_tokens and
// leaving every replica signing with a different key.
const ssoSigningKeySetting = "sso_oidc_signing_key"

// settingsWriter is the optional write side of the settings service. The
// gateway only needs Get for normal operation; storing the generated signing
// key uses Set when the concrete service offers it (tests may pass a read-only
// fake, in which case the key stays process-local).
type settingsWriter interface {
	Set(ctx context.Context, key, value string) error
}

// SSOConfig holds OIDC SSO provider settings loaded from SettingService.
type SSOConfig struct {
	Enabled      bool
	Issuer       string
	ClientID     string
	ClientSecret string
	IDTokenTTL   time.Duration
}

// SSOProvider handles OIDC SSO (single sign-on) operations.
// It acts as an OIDC identity provider, supporting the Authorization Code Flow.
//
// ID tokens are the only JWTs left in the system: OIDC Core requires them to be
// signed JWTs. Everything else (OAuth2 access/refresh tokens, user sessions) is
// opaque (see token.go and SessionStore).
type SSOProvider struct {
	deps   Deps
	config SSOConfig

	keyMu sync.Mutex
	key   *rsa.PrivateKey
	kid   string
}

// NewSSOProvider creates a new SSOProvider with the given dependencies.
func NewSSOProvider(deps Deps) *SSOProvider {
	return &SSOProvider{
		deps:   deps,
		config: SSOConfig{IDTokenTTL: time.Hour},
	}
}

// loadSettings reads OIDC SSO configuration from the settings table.
func (p *SSOProvider) loadSettings(ctx context.Context) {
	if p.deps.SettingService == nil {
		return
	}
	enabled, _ := p.deps.SettingService.Get(ctx, "sso_oidc_enabled")
	p.config.Enabled = enabled == "true"
	p.config.Issuer, _ = p.deps.SettingService.Get(ctx, "sso_oidc_issuer")
	p.config.ClientID, _ = p.deps.SettingService.Get(ctx, "sso_oidc_client_id")
	p.config.ClientSecret, _ = p.deps.SettingService.Get(ctx, "sso_oidc_client_secret")
	if raw, err := p.deps.SettingService.Get(ctx, "sso_id_token_ttl"); err == nil && raw != "" {
		if secs, perr := time.ParseDuration(raw + "s"); perr == nil && secs > 0 {
			p.config.IDTokenTTL = secs
		}
	}
}

// signingKey returns the ID-token signing key and its key id, generating and
// persisting the key on first use.
func (p *SSOProvider) signingKey(ctx context.Context) (*rsa.PrivateKey, string) {
	p.keyMu.Lock()
	defer p.keyMu.Unlock()
	if p.key != nil {
		return p.key, p.kid
	}
	if p.deps.SettingService != nil {
		if raw, err := p.deps.SettingService.Get(ctx, ssoSigningKeySetting); err == nil && raw != "" {
			if parsed, perr := parseRSAPrivateKeyPEM(raw); perr == nil {
				p.key, p.kid = parsed, kidForPublicKey(&parsed.PublicKey)
				return p.key, p.kid
			}
		}
	}
	generated, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, ""
	}
	p.key, p.kid = generated, kidForPublicKey(&generated.PublicKey)
	if writer, ok := p.deps.SettingService.(settingsWriter); ok {
		_ = writer.Set(ctx, ssoSigningKeySetting, marshalRSAPrivateKeyPEM(generated))
	}
	return p.key, p.kid
}

func parseRSAPrivateKeyPEM(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("sso: signing key is not valid PEM")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func marshalRSAPrivateKeyPEM(key *rsa.PrivateKey) string {
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}

// kidForPublicKey derives the JWKS key id from the public key itself, so a
// rotated key is visible to relying parties as a different kid.
func kidForPublicKey(pub *rsa.PublicKey) string {
	h := sha256.New()
	_, _ = h.Write(pub.N.Bytes())
	_, _ = h.Write(big.NewInt(int64(pub.E)).Bytes())
	return "rs256-" + hex.EncodeToString(h.Sum(nil))[:16]
}

// jwks builds the JSON Web Key Set for the current signing key.
func (p *SSOProvider) jwks(ctx context.Context) map[string]any {
	key, kid := p.signingKey(ctx)
	if key == nil {
		return map[string]any{"keys": []map[string]any{}}
	}
	return map[string]any{"keys": []map[string]any{{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": kid,
		"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}}}
}

// generateIDToken creates an OIDC ID token (JWT) with standard claims.
func (p *SSOProvider) generateIDToken(ctx context.Context, issuer, subject, audience, name, username, nonce string) (string, error) {
	key, kid := p.signingKey(ctx)
	if key == nil {
		return "", golangjwt.ErrInvalidKey
	}
	now := time.Now()
	ttl := p.config.IDTokenTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	claims := golangjwt.MapClaims{
		"iss":                issuer,
		"sub":                subject,
		"aud":                audience,
		"exp":                now.Add(ttl).Unix(),
		"iat":                now.Unix(),
		"auth_time":          now.Unix(),
		"name":               name,
		"preferred_username": username,
		// azp (authorized party) claim per OIDC spec
		"azp": audience,
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	return token.SignedString(key)
}

// parseBearerToken extracts a Bearer token from the Authorization header.
func parseBearerToken(authHeader string) string {
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return ""
	}
	return authHeader[7:]
}
