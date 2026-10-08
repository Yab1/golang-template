package config

import (
	"time"

	"github.com/Yab1/golang-template/internal/platform/env"
)

// Notification holds global channel switches and the Postgres delivery poller.
type Notification struct {
	DispatchEnabled bool
	PollInterval    time.Duration
	BatchSize       int
	MaxAttempts     int
	Email           bool
	SMS             bool
	Push            bool
	InApp           bool
	Telegram        bool
	TelegramToken   string
	SMSURL          string
	SMSToken        string
	PushURL         string
	PushToken       string
}

func loadNotification() Notification {
	poll, err := time.ParseDuration(env.GetString("NOTIFICATION_POLL_INTERVAL", "2s"))
	if err != nil || poll <= 0 {
		poll = 2 * time.Second
	}
	batch := env.GetInt("NOTIFICATION_BATCH_SIZE", 25)
	if batch < 1 {
		batch = 25
	}
	attempts := env.GetInt("NOTIFICATION_MAX_ATTEMPTS", 5)
	if attempts < 1 {
		attempts = 5
	}
	return Notification{
		DispatchEnabled: env.GetBool("NOTIFICATION_DISPATCH_ENABLED", true),
		PollInterval:    poll,
		BatchSize:       batch,
		MaxAttempts:     attempts,
		Email:           env.GetBool("NOTIFICATION_EMAIL_ENABLED", false),
		SMS:             env.GetBool("NOTIFICATION_SMS_ENABLED", false),
		Push:            env.GetBool("NOTIFICATION_PUSH_ENABLED", false),
		InApp:           env.GetBool("NOTIFICATION_IN_APP_ENABLED", true),
		Telegram:        env.GetBool("NOTIFICATION_TELEGRAM_ENABLED", false),
		TelegramToken:   env.GetString("TELEGRAM_BOT_TOKEN", ""),
		SMSURL:          env.GetString("SMS_PROVIDER_URL", ""),
		SMSToken:        env.GetString("SMS_PROVIDER_TOKEN", ""),
		PushURL:         env.GetString("PUSH_PROVIDER_URL", ""),
		PushToken:       env.GetString("PUSH_PROVIDER_TOKEN", ""),
	}
}
