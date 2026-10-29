package mailer

import (
	"context"
	"fmt"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// sanitizerPolicy allows safe HTML tags for email content.
var sanitizerPolicy = bluemonday.UGCPolicy()

// sanitizeHTML strips dangerous HTML elements/attrs from email body.
func sanitizeHTML(s string) string {
	return sanitizerPolicy.Sanitize(s)
}

// sanitizeAddr strips CRLF from email address to prevent SMTP header injection.
func sanitizeAddr(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, s)
}

// Message is a single outbound email.
type Message struct {
	To       []string
	Subject  string
	HTMLBody string
	TextBody string // plain-text fallback; used when HTMLBody is empty
}

// Sender sends email messages.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Config holds the mailer configuration. Fields are populated from settings
// via pkg.BuildMailerConfig; the zero value produces a log-only sender.
type Config struct {
	// Provider selects the sending backend: "smtp" | "resend" | "sendgrid" | "log"
	Provider string
	From     string
	FromName string

	// SMTP
	Host     string
	Port     int
	Username string
	Password string
	UseTLS   bool // true = implicit TLS (port 465); false = STARTTLS (port 587)

	// API-based providers (Resend, SendGrid)
	APIKey string
}

// New returns a Sender for the given config.
func New(cfg Config) (Sender, error) {
	switch cfg.Provider {
	case "", "log":
		return &logSender{from: cfg.From}, nil
	case "smtp":
		return newSMTPSender(cfg)
	case "resend":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("mailer: resend provider requires api_key")
		}
		return newResendSender(cfg), nil
	case "sendgrid":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("mailer: sendgrid provider requires api_key")
		}
		return newSendGridSender(cfg), nil
	default:
		return nil, fmt.Errorf("mailer: unsupported provider %q", cfg.Provider)
	}
}
