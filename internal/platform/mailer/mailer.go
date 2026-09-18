package mailer

import (
	"context"
	"fmt"
	"mime"
	"strings"

	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/platform/config"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
	Driver() string
}

func New(cfg config.Mail, log *zap.SugaredLogger) (Mailer, error) {
	if !cfg.Enabled {
		return NewNop(), nil
	}

	switch strings.ToLower(cfg.Driver) {
	case "", "log":
		return NewLog(log), nil
	case "nop":
		return NewNop(), nil
	case "smtp":
		return NewSMTP(cfg)
	case "ses":
		return NewSES(cfg)
	default:
		return nil, fmt.Errorf("unknown mail driver %q (use log, smtp, or ses)", cfg.Driver)
	}
}

func buildRFC822(fromEmail, fromName string, msg Message) []byte {
	var b strings.Builder
	from := fromEmail
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}

	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")

	if msg.HTML != "" {
		boundary := "golang-template-boundary"
		fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", boundary, msg.Text)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", boundary, msg.HTML)
		fmt.Fprintf(&b, "--%s--\r\n", boundary)
	} else {
		fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n\r\n%s", msg.Text)
	}

	return []byte(b.String())
}

func resolveFrom(cfg config.Mail) (email, name string, err error) {
	email = strings.TrimSpace(cfg.From)
	if email == "" {
		return "", "", fmt.Errorf("MAIL_FROM is required for driver %s", cfg.Driver)
	}
	return email, cfg.FromName, nil
}
