package mailer

import (
	"context"

	"github.com/sirupsen/logrus"
)

// logSender writes email content to the application log instead of sending it.
// Used as the default provider when email is not yet configured, and in development.
type logSender struct{ from string }

func (s *logSender) Send(_ context.Context, msg Message) error {
	logrus.WithFields(logrus.Fields{
		"from":            s.from,
		"to_count":        len(msg.To),
		"subject":         msg.Subject,
		"text_body_bytes": len(msg.TextBody),
		"html_body_bytes": len(msg.HTMLBody),
	}).Info("[mailer/log] email not sent (provider=log)")
	return nil
}
