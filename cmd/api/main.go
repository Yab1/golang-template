package main

import (
	"context"
	"os"
	"time"

	"github.com/Yab1/golang-template/internal/modules/file"
	"github.com/Yab1/golang-template/internal/modules/post"
	"github.com/Yab1/golang-template/internal/modules/user"
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
	"github.com/redis/go-redis/v9"
)

const version = "0.0.1"

//	@title						Golang Template API
//	@version					0.0.1
//	@description				REST API template: JWT access+refresh, RBAC, posts CRUD, files (local/minio/s3), mail, Redis rate limiting, human reference ids.
//	@termsOfService				https://www.google.com/policies/terms/

//	@contact.name				Yeabsera
//	@contact.url				https://yeabsera-dev.vercel.app/
//	@contact.email				yeabsera.dev@gmail.com

//	@license.name				Apache 2.0
//	@license.url				https://www.apache.org/licenses/LICENSE-2.0.html

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
	defer log.Sync()

	pool, err := db.New(
		cfg.DB.Addr,
		cfg.DB.MaxConns,
		cfg.DB.MinConns,
		cfg.DB.MaxConnIdleTime,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	log.Info("database connection pool established")

	respond := httpx.NewResponder(log)
	jwtAuth := authn.NewJWTAuthenticator(
		cfg.Auth.Token.Secret,
		cfg.Auth.Token.Aud,
		cfg.Auth.Token.Iss,
	)

	var (
		redisLimiter ratelimiter.Limiter
		rdb          *redis.Client
	)
	if cfg.Redis.Enabled {
		client, err := redisx.New(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		if err != nil {
			log.Warnw("redis unavailable; rate limiter will fail open / use memory on auth", "error", err)
		} else {
			rdb = client
			defer rdb.Close()
			redisLimiter = ratelimiter.NewRedisLimiter(rdb)
			log.Info("redis connection established")
		}
	}

	rl := &ratelimiter.Middleware{
		Limiter:  redisLimiter,
		Fallback: ratelimiter.NewMemoryLimiter(),
		Respond:  respond,
		Logger:   log,
		Enabled:  cfg.RateLimit.Enabled,
	}

	mail, err := mailer.New(cfg.Mail, log)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("mailer ready", "driver", mail.Driver(), "enabled", cfg.Mail.Enabled)

	refs := refid.New(cfg.RefPrefix)
	log.Infow("reference id prefix", "prefix", refs.Prefix())

	blocklist := authn.NewBlocklist(rdb)
	users := user.New(pool, respond, jwtAuth, cfg.Auth.Token, mail, cfg.AppName, refs, blocklist, rl.ByIPAuth(toRule(cfg.RateLimit.Auth)))
	guard := authz.NewGuard(jwtAuth, users, users, respond)
	guard.Required = cfg.Auth.Required
	guard.RBACEnabled = cfg.Auth.RBACEnabled
	guard.AllowDev = cfg.IsDevelopment()
	guard.DevToken = cfg.Auth.DevToken
	guard.DevUserID = cfg.Auth.DevUserID
	guard.Blocklist = blocklist
	users.SetGuard(guard)

	if !cfg.IsDevelopment() {
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

	if cfg.Seed.Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		u, created, err := users.SeedAdmin(ctx, cfg.Seed.Email, cfg.Seed.Username, cfg.Seed.Password, cfg.Seed.Role)
		cancel()
		if err != nil {
			log.Fatalw("seed admin failed", "error", err)
		}
		if created {
			log.Infow("seeded admin", "id", u.ID, "reference_id", u.ReferenceID, "email", u.Email, "role", u.Role.Name)
		} else {
			log.Infow("seed admin already exists", "id", u.ID, "reference_id", u.ReferenceID, "email", u.Email)
		}
	}

	posts := post.New(pool, respond, guard, refs, rl.ByUser(toRule(cfg.RateLimit.Write)))

	var files *file.Module
	if cfg.Files.Enabled {
		store, err := blob.New(cfg.Storage)
		if err != nil {
			log.Fatal(err)
		}
		log.Infow("blob store ready", "driver", store.Driver())
		files = file.New(store, respond, guard, rl.ByUser(toRule(cfg.RateLimit.Write)), cfg.Files)
	}

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
