package main

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/modules/file"
	"github.com/Yab1/golang-template/internal/modules/post"
	"github.com/Yab1/golang-template/internal/modules/user"
	"github.com/Yab1/golang-template/internal/platform/audit"
	"github.com/Yab1/golang-template/internal/platform/authn"
	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/blob"
	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/Yab1/golang-template/internal/platform/db"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/logger"
	"github.com/Yab1/golang-template/internal/platform/mailer"
	"github.com/Yab1/golang-template/internal/platform/ratelimiter"
	redisx "github.com/Yab1/golang-template/internal/platform/redis"
	"github.com/Yab1/golang-template/internal/platform/refid"
)

const version = "0.0.1"

//	@title			Golang Template API
//	@version		0.0.1
//	@description	REST API template: JWT access+refresh, RBAC, posts CRUD, files (local/minio/s3), mail, Redis rate limiting, human reference ids.
//	@termsOfService	https://www.google.com/policies/terms/

//	@contact.name	Yeabsera
//	@contact.url	https://yeabsera-dev.vercel.app/
//	@contact.email	yeabsera.dev@gmail.com

//	@license.name	Apache 2.0
//	@license.url	https://www.apache.org/licenses/LICENSE-2.0.html

//	@externalDocs.description	External documentation
//	@externalDocs.url			https://yeabsera-dev.vercel.app/

//	@host		localhost:8080
//	@BasePath	/api/v1
//	@schemes	http https

// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Type "Bearer" followed by a space and the access JWT (or AUTH_DEV_TOKEN in development).
func main() {
	cfg := config.Load()

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	// --- database ---
	pool := mustDB(cfg, log)
	defer pool.Close()

	// --- http helpers ---
	respond := httpx.NewResponder(log)

	// --- redis + rate limit ---
	rdb, redisLimiter := setupRedis(cfg, log)
	if rdb != nil {
		defer func() { _ = rdb.Close() }()
	}
	rl := setupRateLimiter(cfg, respond, log, redisLimiter)

	// --- mail ---
	mail := mustMail(cfg, log)

	// --- audit ---
	auditLog := setupAudit(cfg, pool, log)

	// --- auth (jwt, blocklist, guard, user module) ---
	refs := refid.New(cfg.RefPrefix)
	log.Infow("reference id prefix", "prefix", refs.Prefix())
	users, guard := setupAuth(cfg, pool, respond, mail, refs, rdb, rl, auditLog, log)
	warnProdAuth(cfg, log)

	// --- seed ---
	maybeSeed(cfg, users, log)

	// --- domain modules ---
	posts := post.New(pool, respond, guard, refs, rl.ByUser(toRule(cfg.RateLimit.Write)), auditLog, log)
	files := setupFiles(cfg, respond, guard, rl, auditLog, log)

	app := &application{
		config:  cfg,
		logger:  log,
		respond: respond,
		limiter: rl,
		users:   users,
		posts:   posts,
		files:   files,
		pool:    pool,
		rdb:     rdb,
	}
	if err := app.run(app.mount()); err != nil {
		log.Fatal(err)
	}
}

func mustDB(cfg config.Config, log *zap.SugaredLogger) *pgxpool.Pool {
	pool, err := db.New(cfg.DB.Addr, cfg.DB.MaxConns, cfg.DB.MinConns, cfg.DB.MaxConnIdleTime)
	if err != nil {
		log.Fatal(err)
	}
	log.Info("database connection pool established")
	return pool
}

func setupRedis(cfg config.Config, log *zap.SugaredLogger) (*redis.Client, ratelimiter.Limiter) {
	if !cfg.Redis.Enabled {
		return nil, nil
	}
	client, err := redisx.New(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		log.Warnw("redis unavailable; rate limiter will fail open / use memory on auth", "error", err)
		return nil, nil
	}
	log.Info("redis connection established")
	return client, ratelimiter.NewRedisLimiter(client)
}

func setupRateLimiter(cfg config.Config, respond *httpx.Responder, log *zap.SugaredLogger, redisLimiter ratelimiter.Limiter) *ratelimiter.Middleware {
	return &ratelimiter.Middleware{
		Limiter:  redisLimiter,
		Fallback: ratelimiter.NewMemoryLimiter(),
		Respond:  respond,
		Logger:   log,
		Enabled:  cfg.RateLimit.Enabled,
	}
}

func mustMail(cfg config.Config, log *zap.SugaredLogger) mailer.Mailer {
	mail, err := mailer.New(cfg.Mail, log)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("mailer ready", "driver", mail.Driver(), "enabled", cfg.Mail.Enabled)
	return mail
}

func setupAuth(
	cfg config.Config,
	pool *pgxpool.Pool,
	respond *httpx.Responder,
	mail mailer.Mailer,
	refs *refid.Generator,
	rdb *redis.Client,
	rl *ratelimiter.Middleware,
	auditLog audit.Logger,
	log *zap.SugaredLogger,
) (*user.Module, *authz.Guard) {
	jwtAuth := authn.NewJWTAuthenticator(
		cfg.Auth.Token.Secret,
		cfg.Auth.Token.Aud,
		cfg.Auth.Token.Iss,
	)
	blocklist := authn.NewBlocklist(rdb)
	users := user.New(
		pool,
		respond,
		jwtAuth,
		cfg.Auth,
		mail,
		cfg.AppName,
		cfg.HTTP.PublicBaseURL,
		refs,
		blocklist,
		rl.ByIPAuth(toRule(cfg.RateLimit.Auth)),
		auditLog,
		log,
	)
	guard := authz.NewGuard(jwtAuth, users, users, respond)
	guard.Required = cfg.Auth.Required
	guard.RBACEnabled = cfg.Auth.RBACEnabled
	guard.AllowDev = cfg.IsDevelopment()
	guard.DevToken = cfg.Auth.DevToken
	guard.DevUserID = cfg.Auth.DevUserID
	guard.Blocklist = blocklist
	users.SetGuard(guard)
	return users, guard
}

func warnProdAuth(cfg config.Config, log *zap.SugaredLogger) {
	if cfg.IsDevelopment() {
		return
	}
	if os.Getenv("AUTH_DEV_TOKEN") != "" {
		log.Warn("AUTH_DEV_TOKEN ignored outside development")
	}
	if !cfg.Auth.Required {
		log.Warn("AUTH_REQUIRED=false outside development")
	}
	if cfg.Metrics.Enabled && cfg.Metrics.Token == "" {
		log.Warn("METRICS_ENABLED ignored outside development without METRICS_TOKEN")
	}
}

func maybeSeed(cfg config.Config, users *user.Module, log *zap.SugaredLogger) {
	if !cfg.Seed.Enabled {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	u, created, err := users.SeedAdmin(ctx, cfg.Seed.Email, cfg.Seed.Username, cfg.Seed.Password, cfg.Seed.Role)
	if err != nil {
		log.Fatalw("seed admin failed", "error", err)
	}
	if created {
		log.Infow("seeded admin", "id", u.ID, "reference_id", u.ReferenceID, "email", u.Email, "role", u.Role.Name)
		return
	}
	log.Infow("seed admin already exists", "id", u.ID, "reference_id", u.ReferenceID, "email", u.Email)
}

func setupAudit(cfg config.Config, pool *pgxpool.Pool, log *zap.SugaredLogger) audit.Logger {
	if !cfg.Audit.Enabled {
		log.Info("audit logging disabled")
		return audit.NewNop()
	}
	log.Info("audit logging enabled")
	return audit.NewStore(pool)
}

func setupFiles(
	cfg config.Config,
	respond *httpx.Responder,
	guard *authz.Guard,
	rl *ratelimiter.Middleware,
	auditLog audit.Logger,
	log *zap.SugaredLogger,
) *file.Module {
	if !cfg.Files.Enabled {
		return nil
	}
	store, err := blob.New(cfg.Storage)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("blob store ready", "driver", store.Driver())
	return file.New(store, respond, guard, rl.ByUser(toRule(cfg.RateLimit.Write)), cfg.Files, auditLog, log)
}
