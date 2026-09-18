package mailer

import (
	"context"
	"strings"
	"testing"

	"github.com/Yab1/golang-template/internal/platform/config"
	"go.uber.org/zap"
)

func TestNopSend(t *testing.T) {
	if err := NewNop().Send(context.Background(), Message{To: "a@b.c", Subject: "hi"}); err != nil {
		t.Fatal(err)
	}
	if NewNop().Driver() != "nop" {
		t.Fatal("driver")
	}
}

func TestLogSend(t *testing.T) {
	m := NewLog(zap.NewNop().Sugar())
	if err := m.Send(context.Background(), Message{To: "a@b.c", Subject: "hi", Text: "body"}); err != nil {
		t.Fatal(err)
	}
}

func TestNewDisabledIsNop(t *testing.T) {
	m, err := New(config.Mail{Enabled: false, Driver: "ses"}, zap.NewNop().Sugar())
	if err != nil {
		t.Fatal(err)
	}
	if m.Driver() != "nop" {
		t.Fatalf("got %s", m.Driver())
	}
}

func TestNewUnknownDriver(t *testing.T) {
	_, err := New(config.Mail{Enabled: true, Driver: "pigeon"}, zap.NewNop().Sugar())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewSMTPRequiresFrom(t *testing.T) {
	_, err := NewSMTP(config.Mail{Driver: "smtp", SMTP: config.SMTP{Host: "localhost", Port: 1025}})
	if err == nil {
		t.Fatal("expected MAIL_FROM error")
	}
}

func TestBuildRFC822(t *testing.T) {
	raw := string(buildRFC822("noreply@local", "App", Message{
		To:      "user@local",
		Subject: "Welcome",
		Text:    "hello",
		HTML:    "<p>hello</p>",
	}))
	for _, want := range []string{"From: App <noreply@local>", "To: user@local", "Subject: Welcome", "hello", "<p>hello</p>"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("missing %q in %q", want, raw)
		}
	}
}
