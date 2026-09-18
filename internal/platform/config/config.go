package config

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/platform/env"
)

// Config mirrors .envrc.example section groups.
type Config struct {
	// --- app ---
	Addr      string
	Env       string
	AppName   string
	RefPrefix string

	// --- log / http / swagger / database / auth / … ---
	Log       Log
	HTTP      HTTP
	Swagger   Swagger
	DB        DB
	Auth      Auth
	Seed      Seed
	Redis     Redis
	RateLimit RateLimit
	Storage   Storage
	Files     Files
	Mail      Mail
	Metrics   Metrics
	Audit     Audit
}

func Load() Config {
	envName := env.GetString("ENV", "development")
	appName := env.GetString("APP_NAME", "golang-template")

	return Config{
		Addr:      env.GetString("ADDR", ":8080"),
		Env:       envName,
		AppName:   appName,
		RefPrefix: env.GetString("REF_PREFIX", appName),
		Log:       loadLog(envName),
		HTTP:      loadHTTP(),
		Swagger:   loadSwagger(),
		DB:        loadDB(),
		Auth:      loadAuth(envName),
		Seed:      loadSeed(),
		Redis:     loadRedis(),
		RateLimit: loadRateLimit(),
		Storage:   loadStorage(),
		Files:     loadFiles(),
		Mail:      loadMail(appName),
		Metrics:   loadMetrics(),
		Audit:     loadAudit(),
	}
}

func (c Config) IsDevelopment() bool {
	return strings.EqualFold(c.Env, "development")
}

func (c Config) MetricsExposed() bool {
	if !c.Metrics.Enabled {
		return false
	}
	if c.IsDevelopment() {
		return true
	}
	return c.Metrics.Token != ""
}

func parseDevUserID() uuid.UUID {
	raw := env.GetString("AUTH_DEV_USER_ID", "")
	if raw == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func minuteRule(requests int) Rule {
	return Rule{Requests: requests, Window: time.Minute}
}
