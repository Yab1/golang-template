package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"

	"github.com/Yab1/golang-template/docs"
	"github.com/Yab1/golang-template/internal/modules/file"
	"github.com/Yab1/golang-template/internal/modules/post"
	"github.com/Yab1/golang-template/internal/modules/user"
	"github.com/Yab1/golang-template/internal/platform/config"
	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/Yab1/golang-template/internal/platform/metrics"
	"github.com/Yab1/golang-template/internal/platform/ratelimiter"
)

type application struct {
	config  config.Config
	logger  *zap.SugaredLogger
	respond *httpx.Responder
	limiter *ratelimiter.Middleware
	users   *user.Module
	posts   *post.Module
	files   *file.Module
	pool    *pgxpool.Pool
	rdb     *redis.Client
}

func (app *application) mount() http.Handler {
	r := chi.NewRouter()

	// --- middleware ---
	r.Use(middleware.RequestID)
	r.Use(echoRequestID)
	r.Use(httpx.RealIP(app.config.HTTP.TrustedProxies))
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	if app.config.MetricsExposed() {
		r.Use(metrics.Middleware)
	}
	if app.config.HTTP.CORSEnabled {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   app.config.HTTP.CORSOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
			ExposedHeaders:   []string{"Link", "X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}

	// --- ops ---
	if app.config.Swagger.Enabled {
		applySwaggerInfo(app.config, version)
		r.Get("/docs/*", httpSwagger.Handler(
			httpSwagger.URL("/docs/doc.json"),
			httpSwagger.DeepLinking(true),
			httpSwagger.DocExpansion("none"),
			httpSwagger.PersistAuthorization(true),
			httpSwagger.DefaultModelsExpandDepth(httpSwagger.ShowModel),
			httpSwagger.UIConfig(map[string]string{
				"filter":             "true",
				"displayOperationId": "false",
				"tagsSorter":         `"alpha"`,
			}),
		))
	}
	if app.config.MetricsExposed() {
		r.With(metrics.Protect(app.config.Metrics.Token)).Handle("/metrics", metrics.Handler())
	}
	r.Get("/live", app.handleLive)
	r.Get("/ready", app.handleReady)
	r.Get("/health", app.handleHealthCheck)

	// --- api v1 modules ---
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(app.limiter.ByIP(toRule(app.config.RateLimit.Read)))
		app.users.Routes(r)
		app.posts.Routes(r)
		if app.files != nil {
			app.files.Routes(r)
		}
	})

	return r
}

func echoRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set(middleware.RequestIDHeader, id)
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) run(mux http.Handler) error {
	timeout := app.config.HTTP.ShutdownTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	srv := &http.Server{
		Addr:         app.config.Addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		app.logger.Infow("server has started",
			"addr", app.config.Addr,
			"env", app.config.Env,
			"auth_required", app.config.Auth.Required,
			"rbac", app.config.Auth.RBACEnabled,
			"storage", app.config.Storage.Driver,
			"mail", app.config.Mail.Driver,
			"metrics", app.config.Metrics.Enabled,
		)
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		app.logger.Infow("shutdown signal", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return err
		}
		return <-errCh
	}
}

func toRule(r config.Rule) ratelimiter.Rule {
	return ratelimiter.Rule{
		Requests: r.Requests,
		Window:   r.Window,
	}
}

func applySwaggerInfo(cfg config.Config, fallbackVersion string) {
	sw := cfg.Swagger
	docs.SwaggerInfo.Title = sw.Title
	docs.SwaggerInfo.Description = sw.Description
	if sw.Version != "" {
		docs.SwaggerInfo.Version = sw.Version
	} else {
		docs.SwaggerInfo.Version = fallbackVersion
	}
	docs.SwaggerInfo.BasePath = "/api/v1"

	if host, schemes := parsePublicBase(cfg.HTTP.PublicBaseURL); host != "" {
		docs.SwaggerInfo.Host = host
		docs.SwaggerInfo.Schemes = schemes
	}
}

func parsePublicBase(raw string) (host string, schemes []string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", nil
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}
	return u.Host, []string{scheme}
}
