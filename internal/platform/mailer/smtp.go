package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"

	"github.com/Yab1/golang-template/internal/platform/config"
)

type SMTP struct {
	host     string
	port     int
	user     string
	pass     string
	from     string
	fromName string
	tls      bool
	startTLS bool
}

func NewSMTP(cfg config.Mail) (*SMTP, error) {
	from, name, err := resolveFrom(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.SMTP.Host == "" {
		return nil, fmt.Errorf("SMTP_HOST is required")
	}
	return &SMTP{
		host:     cfg.SMTP.Host,
		port:     cfg.SMTP.Port,
		user:     cfg.SMTP.User,
		pass:     cfg.SMTP.Pass,
		from:     from,
		fromName: name,
		tls:      cfg.SMTP.TLS,
		startTLS: cfg.SMTP.StartTLS,
	}, nil
}

func (s *SMTP) Driver() string {
	return "smtp"
}

func (s *SMTP) Send(ctx context.Context, msg Message) error {
	raw := buildRFC822(s.from, s.fromName, msg)
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))

	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}

	errCh := make(chan error, 1)
	go func() {
		switch {
		case s.tls:
			errCh <- sendImplicitTLS(addr, s.host, auth, s.from, msg.To, raw)
		case s.startTLS:
			errCh <- sendStartTLS(addr, s.host, auth, s.from, msg.To, raw)
		default:
			errCh <- smtp.SendMail(addr, auth, s.from, []string{msg.To}, raw)
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, raw []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	return smtpTransmit(c, auth, from, to, raw)
}

func sendStartTLS(addr, host string, auth smtp.Auth, from, to string, raw []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
		return err
	}
	return smtpTransmit(c, auth, from, to, raw)
}

func smtpTransmit(c *smtp.Client, auth smtp.Auth, from, to string, raw []byte) error {
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
