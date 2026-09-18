package main

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Yab1/golang-template/internal/platform/httpx"
)

func (app *application) handleLive(w http.ResponseWriter, r *http.Request) {
	if err := httpx.JSONResponse(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": version,
	}); err != nil {
		app.respond.InternalServerError(w, r, err)
	}
}

func (app *application) handleReady(w http.ResponseWriter, r *http.Request) {
	checks, ok := app.dependencyChecks(r.Context())
	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}
	if err := httpx.WriteJSON(w, status, map[string]any{
		"status":  statusLabel(ok),
		"checks":  checks,
		"version": version,
	}); err != nil {
		app.respond.InternalServerError(w, r, err)
	}
}

func (app *application) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	checks, ok := app.dependencyChecks(r.Context())
	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}

	data := map[string]any{
		"status":  statusLabel(ok),
		"env":     app.config.Env,
		"version": version,
		"checks":  checks,
		"flags": map[string]any{
			"auth_required":  app.config.Auth.Required,
			"rbac_enabled":   app.config.Auth.RBACEnabled,
			"dev_token":      app.config.Auth.DevToken != "",
			"rate_limit":     app.config.RateLimit.Enabled,
			"redis":          app.config.Redis.Enabled,
			"swagger":        app.config.HTTP.SwaggerEnabled,
			"cors":           app.config.HTTP.CORSEnabled,
			"files":          app.config.Files.Enabled,
			"storage_driver": app.config.Storage.Driver,
			"file_max_bytes": strconv.FormatInt(app.config.Files.MaxBytes, 10),
			"mail":           app.config.Mail.Enabled,
			"mail_driver":    app.config.Mail.Driver,
			"metrics":        app.config.MetricsExposed(),
			"seed":           app.config.Seed.Enabled,
			"log_level":      app.config.Log.Level,
			"log_format":     app.config.Log.Format,
		},
	}

	if err := httpx.WriteJSON(w, status, data); err != nil {
		app.respond.InternalServerError(w, r, err)
	}
}

func (app *application) dependencyChecks(ctx context.Context) (map[string]string, bool) {
	checks := map[string]string{}
	ok := true

	if app.pool == nil {
		checks["db"] = "unconfigured"
		ok = false
	} else if err := app.pool.Ping(ctx); err != nil {
		checks["db"] = "error"
		ok = false
	} else {
		checks["db"] = "ok"
	}

	if app.config.Redis.Enabled {
		if app.rdb == nil {
			checks["redis"] = "unavailable"
			ok = false
		} else if err := app.rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "error"
			ok = false
		} else {
			checks["redis"] = "ok"
		}
	}

	return checks, ok
}

func statusLabel(ok bool) string {
	if ok {
		return "ok"
	}
	return "unavailable"
}
