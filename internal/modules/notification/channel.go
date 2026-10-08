package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Yab1/golang-template/internal/platform/mailer"
)

type outbound struct {
	To      string
	Subject string
	Body    string
	Format  string
}

type sender interface {
	Send(ctx context.Context, msg outbound) error
}

type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

func permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

func isPermanent(err error) bool {
	var p *permanentError
	return errors.As(err, &p)
}

type inAppSender struct{}

func (inAppSender) Send(context.Context, outbound) error { return nil }

type emailSender struct{ mail mailer.Mailer }

func (s emailSender) Send(ctx context.Context, msg outbound) error {
	if s.mail == nil || s.mail.Driver() == "nop" {
		return permanent(errors.New("mail is disabled"))
	}
	out := mailer.Message{To: msg.To, Subject: msg.Subject, Text: msg.Body}
	if msg.Format == FormatHTML {
		out.HTML = msg.Body
		out.Text = plainFromHTML(msg.Body)
	}
	if err := s.mail.Send(ctx, out); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	return nil
}

type webhookSender struct {
	url   string
	token string
	kind  string
	http  *http.Client
}

func (s webhookSender) Send(ctx context.Context, msg outbound) error {
	payload := map[string]string{
		"to":      msg.To,
		"subject": msg.Subject,
		"body":    msg.Body,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return permanent(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(raw))
	if err != nil {
		return permanent(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	res, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed", s.kind)
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	return classifyStatus(s.kind, res.StatusCode)
}

type telegramSender struct {
	token string
	http  *http.Client
}

func (s telegramSender) Send(ctx context.Context, msg outbound) error {
	text := msg.Body
	if msg.Subject != "" {
		text = msg.Subject + "\n" + msg.Body
	}
	payload := map[string]string{"chat_id": msg.To, "text": text}
	raw, err := json.Marshal(payload)
	if err != nil {
		return permanent(err)
	}
	url := "https://api.telegram.org/bot" + s.token + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return permanent(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := s.http.Do(req)
	if err != nil {
		return errors.New("telegram request failed")
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	return classifyStatus("telegram", res.StatusCode)
}

func classifyStatus(kind string, code int) error {
	if code >= 200 && code < 300 {
		return nil
	}
	err := fmt.Errorf("%s status %d", kind, code)
	if code == http.StatusTooManyRequests || code == http.StatusRequestTimeout || code >= 500 {
		return err
	}
	return permanent(err)
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

func trimToken(s string) string { return strings.TrimSpace(s) }
