package notification

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/platform/audit"
	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/mailer"
	"github.com/Yab1/golang-template/internal/platform/outbox"
	"github.com/Yab1/golang-template/internal/platform/refid"
)

// ErrTemplate means the stored body could not be filled.
var ErrTemplate = errors.New("notification template failed")

type Module struct {
	db          *pgxpool.Pool
	store       *Store
	accounts    Accounts
	respond     *httpx.Responder
	guard       *authz.Guard
	audit       audit.Logger
	log         *zap.SugaredLogger
	events      *outbox.Store
	topicPrefix string
	eventSource string
	writeLimit  func(http.Handler) http.Handler
	cfg         config.Notification
	mail        mailer.Mailer
	senders     map[string]sender
}

func New(
	db *pgxpool.Pool,
	respond *httpx.Responder,
	guard *authz.Guard,
	refs *refid.Generator,
	accounts Accounts,
	mail mailer.Mailer,
	cfg config.Notification,
	writeLimit func(http.Handler) http.Handler,
	auditLog audit.Logger,
	log *zap.SugaredLogger,
	events *outbox.Store,
	topicPrefix, eventSource string,
) *Module {
	if writeLimit == nil {
		writeLimit = func(next http.Handler) http.Handler { return next }
	}
	if auditLog == nil {
		auditLog = audit.NewNop()
	}
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	m := &Module{
		db: db, store: NewStore(db, refs), accounts: accounts, respond: respond, guard: guard,
		audit: auditLog, log: log, events: events, topicPrefix: topicPrefix, eventSource: eventSource,
		writeLimit: writeLimit, cfg: cfg, mail: mail, senders: map[string]sender{},
	}
	m.wireSenders()
	return m
}

func (m *Module) wireSenders() {
	httpClient := newHTTPClient()
	if m.cfg.InApp {
		m.senders[ChannelInApp] = inAppSender{}
	}
	if m.cfg.Email && m.mail != nil && m.mail.Driver() != "nop" {
		m.senders[ChannelEmail] = emailSender{mail: m.mail}
	}
	if m.cfg.SMS && trimToken(m.cfg.SMSURL) != "" {
		m.senders[ChannelSMS] = webhookSender{url: trimToken(m.cfg.SMSURL), token: m.cfg.SMSToken, kind: "sms", http: httpClient}
	}
	if m.cfg.Push && trimToken(m.cfg.PushURL) != "" {
		m.senders[ChannelPush] = webhookSender{url: trimToken(m.cfg.PushURL), token: m.cfg.PushToken, kind: "push", http: httpClient}
	}
	if m.cfg.Telegram && trimToken(m.cfg.TelegramToken) != "" {
		m.senders[ChannelTelegram] = telegramSender{token: trimToken(m.cfg.TelegramToken), http: httpClient}
	}
}

func (m *Module) channelGate(name string) (bool, string) {
	switch name {
	case ChannelEmail:
		if !m.cfg.Email {
			return false, "channel disabled"
		}
	case ChannelSMS:
		if !m.cfg.SMS {
			return false, "channel disabled"
		}
	case ChannelPush:
		if !m.cfg.Push {
			return false, "channel disabled"
		}
	case ChannelInApp:
		if !m.cfg.InApp {
			return false, "channel disabled"
		}
	case ChannelTelegram:
		if !m.cfg.Telegram {
			return false, "channel disabled"
		}
	default:
		return false, "channel disabled"
	}
	if _, ok := m.senders[name]; !ok {
		return false, "provider not configured"
	}
	return true, ""
}

func (m *Module) Routes(r chi.Router) {
	admin := m.guard.OwnershipOrRole("admin", func(*http.Request) uuid.UUID { return uuid.Nil })
	self := m.guard.AuthToken

	r.Route("/notifications", func(r chi.Router) {
		r.With(self, m.writeLimit, admin).Post("/", m.sendHandler)
		r.With(self).Get("/", m.inboxHandler)
		r.With(self, m.writeLimit).Post("/{notificationID}/read", m.readHandler)
	})
	r.Route("/notification-deliveries", func(r chi.Router) {
		r.With(self, admin).Get("/", m.listDeliveriesHandler)
		r.With(self, admin).Get("/{notificationID}", m.getDeliveryHandler)
	})
	r.Route("/notification-preferences", func(r chi.Router) {
		r.With(self).Get("/", m.listPreferencesHandler)
		r.With(self, m.writeLimit).Put("/", m.putPreferencesHandler)
	})
	r.Route("/notification-templates", func(r chi.Router) {
		r.With(self, admin).Get("/", m.listTemplatesHandler)
		r.With(self, m.writeLimit, admin).Patch("/{templateID}", m.patchTemplateHandler)
	})
}

func knownChannel(name string) bool {
	switch name {
	case ChannelEmail, ChannelSMS, ChannelPush, ChannelInApp, ChannelTelegram:
		return true
	default:
		return false
	}
}

func knownCategory(name string) bool {
	switch name {
	case CategoryWelcome, CategoryAlert, CategoryStatus, CategoryTransactional:
		return true
	default:
		return false
	}
}

func cleanDestination(channel, raw string) (string, error) {
	dest := strings.TrimSpace(raw)
	if channel == ChannelEmail && dest != "" && !strings.Contains(dest, "@") {
		return "", errors.New("email destination is invalid")
	}
	if len(dest) > 255 {
		return "", errors.New("destination is too long")
	}
	return dest, nil
}
