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

const resendAPI = "https://api.resend.com/emails"

// resendHTTPClient has a 15s timeout to avoid blocking on unresponsive Resend API.
var resendHTTPClient = &http.Client{
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

type resendSender struct{ cfg Config }

func newResendSender(cfg Config) Sender { return &resendSender{cfg: cfg} }

func (s *resendSender) Send(ctx context.Context, msg Message) error {
	msg.HTMLBody = sanitizeHTML(msg.HTMLBody)

	from := s.cfg.From
	if s.cfg.FromName != "" {
		from = s.cfg.FromName + " <" + s.cfg.From + ">"
	}

	payload := map[string]any{
		"from":    from,
		"to":      msg.To,
		"subject": msg.Subject,
	}
	if msg.HTMLBody != "" {
		payload["html"] = msg.HTMLBody
	}
	if msg.TextBody != "" {
		payload["text"] = msg.TextBody
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mailer: resend marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPI, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("mailer: resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := resendHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("mailer: resend send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mailer: resend HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}
