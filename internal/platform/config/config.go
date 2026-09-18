package config

import (
	"strings"
	"time"

	"github.com/Yab1/golang-template/internal/platform/env"
	"github.com/google/uuid"
)

type Config struct {
	Addr      string
	Env       string
	AppName   string
	RefPrefix string
	Log       Log
	DB        DB
	Auth      Auth
	Redis     Redis
	RateLimit RateLimit
	HTTP      HTTP
	Storage   Storage
	Files     Files
	Mail      Mail
	Seed      Seed
	Metrics   Metrics
}

type DB struct {
	Addr            string
	MaxConns        int
	MinConns        int
	MaxConnIdleTime string
}

type Auth struct {
	Required    bool
	RBACEnabled bool
	DevToken    string
	DevUserID   uuid.UUID
	Token       Token
}

type Token struct {
	Secret     string
	Exp        time.Duration
	RefreshExp time.Duration
	Iss        string
	Aud        string
}

type Redis struct {
	Addr     string
	Password string
	DB       int
	Enabled  bool
}

type RateLimit struct {
	Enabled bool
	Auth    Rule
	Write   Rule
	Read    Rule
}

type Rule struct {
	Requests int
	Window   time.Duration
}

type Log struct {
	Level  string
	Format string
}

type HTTP struct {
	SwaggerEnabled  bool
	CORSEnabled     bool
	CORSOrigins     []string
	ShutdownTimeout time.Duration
	TrustedProxies  []string
	PublicBaseURL   string
	Swagger         Swagger
}

type Swagger struct {
	Title            string
	Description      string
	Version          string
	TermsOfService   string
	ContactName      string
	ContactURL       string
	ContactEmail     string
	LicenseName      string
	LicenseURL       string
	ExternalDocsURL  string
	ExternalDocsDesc string
}

type Mail struct {
	Enabled  bool
	Driver   string
	From     string
	FromName string
	SMTP     SMTP
	SES      SES
}

type SMTP struct {
	Host     string
	Port     int
	User     string
	Pass     string
	TLS      bool
	StartTLS bool
}

type SES struct {
	Region    string
	AccessKey string
	SecretKey string
}

type Seed struct {
	Enabled  bool
	Email    string
	Username string
	Password string
	Role     string
}

type Metrics struct {
	Enabled bool
	Token   string
}

type Storage struct {
	Driver    string
	LocalDir  string
	PublicURL string
	S3        S3
}

type S3 struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
	PathStyle bool
}

type Files struct {
	Enabled      bool
	MaxBytes     int64
	AllowedTypes []string
}

func Load() Config {
	driver := strings.ToLower(env.GetString("STORAGE_DRIVER", "local"))
	pathStyle := env.GetBool("S3_PATH_STYLE", driver == "minio")

	var devUserID uuid.UUID
	if raw := env.GetString("AUTH_DEV_USER_ID", ""); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			devUserID = id
		}
	}

	envName := env.GetString("ENV", "development")
	logFormat := "json"
	logLevel := "info"
	if envName == "development" {
		logFormat = "console"
		logLevel = "debug"
	}

	devToken := env.GetString("AUTH_DEV_TOKEN", "")
	if envName != "development" {
		devToken = ""
	}

	appName := env.GetString("APP_NAME", "golang-template")
	return Config{
		Addr:      env.GetString("ADDR", ":8080"),
		Env:       envName,
		AppName:   appName,
		RefPrefix: env.GetString("REF_PREFIX", appName),
		Log: Log{
			Level:  env.GetString("LOG_LEVEL", logLevel),
			Format: env.GetString("LOG_FORMAT", logFormat),
		},
		DB: DB{
			Addr:            env.GetString("DB_ADDR", "postgres://admin:password@localhost:5432/golang_template?sslmode=disable"),
			MaxConns:        env.GetInt("DB_MAX_CONNS", 30),
			MinConns:        env.GetInt("DB_MIN_CONNS", 2),
			MaxConnIdleTime: env.GetString("DB_MAX_CONN_IDLE_TIME", "15m"),
		},
		Auth: Auth{
			Required:    env.GetBool("AUTH_REQUIRED", true),
			RBACEnabled: env.GetBool("RBAC_ENABLED", true),
			DevToken:    devToken,
			DevUserID:   devUserID,
			Token: Token{
				Secret:     env.GetString("AUTH_TOKEN_SECRET", "dev-secret-change-me"),
				Exp:        env.GetDuration("AUTH_TOKEN_EXP", "15m"),
				RefreshExp: env.GetDuration("AUTH_REFRESH_EXP", "168h"),
				Iss:        env.GetString("AUTH_TOKEN_ISS", "golang-template"),
				Aud:        env.GetString("AUTH_TOKEN_AUD", "golang-template"),
			},
		},
		Redis: Redis{
			Addr:     env.GetString("REDIS_ADDR", "localhost:6379"),
			Password: env.GetString("REDIS_PW", ""),
			DB:       env.GetInt("REDIS_DB", 0),
			Enabled:  env.GetBool("REDIS_ENABLED", true),
		},
		RateLimit: RateLimit{
			Enabled: env.GetBool("RATE_LIMIT_ENABLED", true),
			Auth: Rule{
				Requests: env.GetInt("RATE_LIMIT_AUTH", 10),
				Window:   time.Minute,
			},
			Write: Rule{
				Requests: env.GetInt("RATE_LIMIT_WRITE", 120),
				Window:   time.Minute,
			},
			Read: Rule{
				Requests: env.GetInt("RATE_LIMIT_READ", 600),
				Window:   time.Minute,
			},
		},
		HTTP: HTTP{
			SwaggerEnabled:  env.GetBool("SWAGGER_ENABLED", true),
			CORSEnabled:     env.GetBool("CORS_ENABLED", true),
			CORSOrigins:     env.GetCSV("CORS_ALLOWED_ORIGINS", []string{"*"}),
			ShutdownTimeout: env.GetDuration("SHUTDOWN_TIMEOUT", "10s"),
			TrustedProxies:  env.GetCSV("TRUSTED_PROXIES", nil),
			PublicBaseURL:   env.GetString("PUBLIC_BASE_URL", "http://localhost:8080"),
			Swagger: Swagger{
				Title:            env.GetString("SWAGGER_TITLE", "Golang Template API"),
				Description:      env.GetString("SWAGGER_DESCRIPTION", "REST API template: auth (access+refresh), RBAC, posts, files, mail, rate limiting, reference ids."),
				Version:          env.GetString("SWAGGER_VERSION", "0.0.1"),
				TermsOfService:   env.GetString("SWAGGER_TOS", "https://www.google.com/policies/terms/"),
				ContactName:      env.GetString("SWAGGER_CONTACT_NAME", "Yeabsera"),
				ContactURL:       env.GetString("SWAGGER_CONTACT_URL", "https://yeabsera-dev.vercel.app/"),
				ContactEmail:     env.GetString("SWAGGER_CONTACT_EMAIL", "yeabsera.dev@gmail.com"),
				LicenseName:      env.GetString("SWAGGER_LICENSE_NAME", "Apache 2.0"),
				LicenseURL:       env.GetString("SWAGGER_LICENSE_URL", "https://www.apache.org/licenses/LICENSE-2.0.html"),
				ExternalDocsURL:  env.GetString("SWAGGER_EXTERNAL_DOCS_URL", "https://yeabsera-dev.vercel.app/"),
				ExternalDocsDesc: env.GetString("SWAGGER_EXTERNAL_DOCS_DESC", "External documentation"),
			},
		},
		Storage: Storage{
			Driver:    driver,
			LocalDir:  env.GetString("STORAGE_LOCAL_DIR", "./tmp/uploads"),
			PublicURL: env.GetString("STORAGE_PUBLIC_URL", "http://localhost:8080/api/v1/files"),
			S3: S3{
				Endpoint:  env.GetString("S3_ENDPOINT", "localhost:9000"),
				Region:    env.GetString("S3_REGION", "us-east-1"),
				Bucket:    env.GetString("S3_BUCKET", "uploads"),
				AccessKey: env.GetString("S3_ACCESS_KEY", "minioadmin"),
				SecretKey: env.GetString("S3_SECRET_KEY", "minioadmin"),
				UseSSL:    env.GetBool("S3_USE_SSL", false),
				PathStyle: pathStyle,
			},
		},
		Files: Files{
			Enabled:      env.GetBool("FILES_ENABLED", true),
			MaxBytes:     int64(env.GetInt("FILE_MAX_BYTES", 10<<20)),
			AllowedTypes: env.GetCSV("FILE_ALLOWED_TYPES", []string{"image/jpeg", "image/png", "image/webp", "application/pdf"}),
		},
		Mail: Mail{
			Enabled:  env.GetBool("MAIL_ENABLED", true),
			Driver:   strings.ToLower(env.GetString("MAIL_DRIVER", "log")),
			From:     env.GetString("MAIL_FROM", "noreply@localhost"),
			FromName: env.GetString("MAIL_FROM_NAME", env.GetString("APP_NAME", "golang-template")),
			SMTP: SMTP{
				Host:     env.GetString("SMTP_HOST", "localhost"),
				Port:     env.GetInt("SMTP_PORT", 1025),
				User:     env.GetString("SMTP_USER", ""),
				Pass:     env.GetString("SMTP_PASS", ""),
				TLS:      env.GetBool("SMTP_TLS", false),
				StartTLS: env.GetBool("SMTP_STARTTLS", false),
			},
			SES: SES{
				Region:    env.GetString("SES_REGION", "us-east-1"),
				AccessKey: env.GetString("SES_ACCESS_KEY", ""),
				SecretKey: env.GetString("SES_SECRET_KEY", ""),
			},
		},
		Seed: Seed{
			Enabled:  env.GetBool("SEED_ENABLED", false),
			Email:    env.GetString("SEED_ADMIN_EMAIL", "admin@localhost"),
			Username: env.GetString("SEED_ADMIN_USERNAME", "admin"),
			Password: env.GetString("SEED_ADMIN_PASSWORD", ""),
			Role:     env.GetString("SEED_ADMIN_ROLE", "admin"),
		},
		Metrics: Metrics{
			Enabled: env.GetBool("METRICS_ENABLED", false),
			Token:   env.GetString("METRICS_TOKEN", ""),
		},
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
