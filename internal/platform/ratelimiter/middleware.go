package ratelimiter

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/Yab1/golang-template/internal/platform/authz"
	"github.com/Yab1/golang-template/internal/platform/httpx"
)

type Middleware struct {
	Limiter  Limiter
	Fallback Limiter
	Respond  *httpx.Responder
	Logger   *zap.SugaredLogger
	Enabled  bool
}

func (m *Middleware) ByIP(rule Rule) func(http.Handler) http.Handler {
	return m.wrap(rule, clientIP, false)
}

func (m *Middleware) ByIPAuth(rule Rule) func(http.Handler) http.Handler {
	return m.wrap(rule, clientIP, true)
}

func (m *Middleware) ByUser(rule Rule) func(http.Handler) http.Handler {
	return m.wrap(rule, userOrIP, false)
}

func (m *Middleware) wrap(rule Rule, keyFn func(*http.Request) string, memoryFallback bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !m.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			key := keyFn(r)
			res, err := m.allow(r, key, rule, memoryFallback)
			if err != nil {
				m.Logger.Warnw("rate limiter error", "error", err, "key", key)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
			reset := time.Now().Add(res.RetryAfter).Unix()
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

			if !res.Allowed {
				retry := formatRetryAfter(res.RetryAfter)
				m.Respond.RateLimitExceeded(w, r, retry)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) allow(r *http.Request, key string, rule Rule, memoryFallback bool) (Result, error) {
	if m.Limiter != nil {
		res, err := m.Limiter.Allow(r.Context(), key, rule)
		if err == nil {
			return res, nil
		}
		m.Logger.Warnw("rate limiter redis error", "error", err, "key", key)
		if memoryFallback && m.Fallback != nil {
			return m.Fallback.Allow(r.Context(), key, rule)
		}
		return Result{Allowed: true, Limit: rule.Requests, Remaining: rule.Requests}, nil
	}

	if memoryFallback && m.Fallback != nil {
		return m.Fallback.Allow(r.Context(), key, rule)
	}

	return Result{Allowed: true, Limit: rule.Requests, Remaining: rule.Requests}, nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func userOrIP(r *http.Request) string {
	if p := authz.PrincipalFrom(r); p != nil {
		return "user:" + p.ID.String()
	}
	return "ip:" + clientIP(r)
}

func formatRetryAfter(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 1 {
		secs = 1
	}
	return fmt.Sprintf("%d", secs)
}
