package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strings"
)

// smtpSender sends mail via SMTP using the standard library.
// Supports both STARTTLS (port 587, UseTLS=false) and implicit TLS (port 465, UseTLS=true).
// For production use with Gmail, QQ, 163, Outlook etc., set Host/Port/Username/Password
// in settings. All modern SMTP servers require AUTH PLAIN over TLS.
type smtpSender struct {
	cfg Config
}

func newSMTPSender(cfg Config) (Sender, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("mailer: smtp provider requires host")
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("mailer: smtp provider requires from address")
	}
	return &smtpSender{cfg: cfg}, nil
}

func (s *smtpSender) Send(_ context.Context, msg Message) error {
	return s.send(msg)
}

func (s *smtpSender) send(msg Message) error {
	// Sanitize: prevent SMTP header injection in To field
	sanitizedTo := make([]string, len(msg.To))
	for i, addr := range msg.To {
		sanitizedTo[i] = sanitizeAddr(addr)
	}
	msg.To = sanitizedTo
	msg.HTMLBody = sanitizeHTML(msg.HTMLBody)

	port := s.cfg.Port
	if port == 0 {
		if s.cfg.UseTLS {
			port = 465
		} else {
			port = 587
		}
	}
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)

	from := s.cfg.From
	if s.cfg.FromName != "" {
		from = mime.QEncoding.Encode("utf-8", s.cfg.FromName) + " <" + s.cfg.From + ">"
	}

	body, err := buildMIMEBody(from, msg)
	if err != nil {
		return err
	}

	if s.cfg.UseTLS {
		// Implicit TLS (port 465)
		tlsCfg := &tls.Config{ServerName: s.cfg.Host}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("mailer: smtp tls dial: %w", err)
		}
		c, err := smtp.NewClient(conn, s.cfg.Host)
		if err != nil {
			return fmt.Errorf("mailer: smtp new client: %w", err)
		}
		defer c.Quit()
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("mailer: smtp auth: %w", err)
		}
		return sendViaClient(c, s.cfg.From, msg.To, body)
	}

	// STARTTLS (port 587)
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("mailer: smtp dial: %w", err)
	}
	defer c.Quit()

	host, _, _ := net.SplitHostPort(addr)
	if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
		return fmt.Errorf("mailer: smtp starttls: %w", err)
	}
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("mailer: smtp auth: %w", err)
	}
	return sendViaClient(c, s.cfg.From, msg.To, body)
}

func sendViaClient(c *smtp.Client, from string, to []string, body []byte) error {
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("mailer: smtp MAIL FROM: %w", err)
	}
	for _, t := range to {
		if err := c.Rcpt(t); err != nil {
			return fmt.Errorf("mailer: smtp RCPT TO: %w", err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mailer: smtp DATA: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("mailer: smtp write body: %w", err)
	}
	return w.Close()
}

func buildMIMEBody(from string, msg Message) ([]byte, error) {
	var sb strings.Builder

	// Headers
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + strings.Join(msg.To, ", ") + "\r\n")
	sb.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject) + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")

	if msg.HTMLBody != "" && msg.TextBody != "" {
		// multipart/alternative
		mw := multipart.NewWriter(&sb)
		sb.WriteString("Content-Type: multipart/alternative; boundary=\"" + mw.Boundary() + "\"\r\n\r\n")

		pw, err := mw.CreatePart(map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}, "Content-Transfer-Encoding": {"quoted-printable"}})
		if err != nil {
			return nil, fmt.Errorf("mailer: build text MIME part: %w", err)
		}
		qw := quotedprintable.NewWriter(pw)
		if _, err := qw.Write([]byte(msg.TextBody)); err != nil {
			return nil, fmt.Errorf("mailer: write text MIME part: %w", err)
		}
		if err := qw.Close(); err != nil {
			return nil, fmt.Errorf("mailer: close text MIME part: %w", err)
		}

		pw, err = mw.CreatePart(map[string][]string{"Content-Type": {"text/html; charset=utf-8"}, "Content-Transfer-Encoding": {"quoted-printable"}})
		if err != nil {
			return nil, fmt.Errorf("mailer: build HTML MIME part: %w", err)
		}
		qw = quotedprintable.NewWriter(pw)
		if _, err := qw.Write([]byte(msg.HTMLBody)); err != nil {
			return nil, fmt.Errorf("mailer: write HTML MIME part: %w", err)
		}
		if err := qw.Close(); err != nil {
			return nil, fmt.Errorf("mailer: close HTML MIME part: %w", err)
		}

		if err := mw.Close(); err != nil {
			return nil, fmt.Errorf("mailer: close MIME writer: %w", err)
		}
	} else if msg.HTMLBody != "" {
		sb.WriteString("Content-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n")
		qw := quotedprintable.NewWriter(&sb)
		if _, err := qw.Write([]byte(msg.HTMLBody)); err != nil {
			return nil, fmt.Errorf("mailer: write HTML body: %w", err)
		}
		if err := qw.Close(); err != nil {
			return nil, fmt.Errorf("mailer: close HTML body: %w", err)
		}
	} else {
		sb.WriteString("Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n")
		qw := quotedprintable.NewWriter(&sb)
		if _, err := qw.Write([]byte(msg.TextBody)); err != nil {
			return nil, fmt.Errorf("mailer: write text body: %w", err)
		}
		if err := qw.Close(); err != nil {
			return nil, fmt.Errorf("mailer: close text body: %w", err)
		}
	}

	return []byte(sb.String()), nil
}
