package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const sendgridAPI = "https://api.sendgrid.com/v3/mail/send"

// sendgridHTTPClient has a 15s timeout to avoid blocking on unresponsive SendGrid API.
var sendgridHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       60 * time.Second,
	},
}

type sendgridSender struct{ cfg Config }

func newSendGridSender(cfg Config) Sender { return &sendgridSender{cfg: cfg} }

func (s *sendgridSender) Send(ctx context.Context, msg Message) error {
	msg.HTMLBody = sanitizeHTML(msg.HTMLBody)

	type addr struct {
		Email string `json:"email"`
		Name  string `json:"name,omitempty"`
	}
	type content struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	type personalization struct {
		To []addr `json:"to"`
	}

	tos := make([]addr, len(msg.To))
	for i, t := range msg.To {
		tos[i] = addr{Email: t}
	}

	var contents []content
	if msg.HTMLBody != "" {
		contents = append(contents, content{Type: "text/html", Value: msg.HTMLBody})
	}
	if msg.TextBody != "" {
		contents = append(contents, content{Type: "text/plain", Value: msg.TextBody})
	}
	if len(contents) == 0 {
		contents = []content{{Type: "text/plain", Value: ""}}
	}

	payload := map[string]any{
		"personalizations": []personalization{{To: tos}},
		"from":             addr{Email: s.cfg.From, Name: s.cfg.FromName},
		"subject":          msg.Subject,
		"content":          contents,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mailer: sendgrid marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendgridAPI, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("mailer: sendgrid request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sendgridHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("mailer: sendgrid send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mailer: sendgrid HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}
