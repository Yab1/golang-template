package ratelimiter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Yab1/golang-template/internal/platform/httpx"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestRedisLimiter_AllowAndExhaust(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	limiter := NewRedisLimiter(client)
	rule := Rule{Requests: 2, Window: time.Second}
	ctx := context.Background()

	first, err := limiter.Allow(ctx, "k", rule)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Allowed || first.Remaining != 1 {
		t.Fatalf("first: %+v", first)
	}

	second, err := limiter.Allow(ctx, "k", rule)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Allowed {
		t.Fatalf("second should be allowed: %+v", second)
	}

	third, err := limiter.Allow(ctx, "k", rule)
	if err != nil {
		t.Fatal(err)
	}
	if third.Allowed {
		t.Fatalf("third should be blocked: %+v", third)
	}
	if third.RetryAfter <= 0 {
		t.Fatalf("expected retry-after, got %v", third.RetryAfter)
	}
}

func TestRedisLimiter_WindowRollover(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	limiter := NewRedisLimiter(client)
	rule := Rule{Requests: 1, Window: 50 * time.Millisecond}
	ctx := context.Background()

	first, err := limiter.Allow(ctx, "roll", rule)
	if err != nil || !first.Allowed {
		t.Fatalf("first: %+v err=%v", first, err)
	}

	blocked, err := limiter.Allow(ctx, "roll", rule)
	if err != nil || blocked.Allowed {
		t.Fatalf("blocked: %+v err=%v", blocked, err)
	}

	time.Sleep(120 * time.Millisecond)
	mr.FastForward(120 * time.Millisecond)

	again, err := limiter.Allow(ctx, "roll", rule)
	if err != nil || !again.Allowed {
		t.Fatalf("after window: %+v err=%v", again, err)
	}
}

func TestMemoryLimiter_Concurrency(t *testing.T) {
	limiter := NewMemoryLimiter()
	rule := Rule{Requests: 50, Window: time.Minute}

	var wg sync.WaitGroup
	var allowed int
	var mu sync.Mutex

	for i := 0; i < 80; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := limiter.Allow(context.Background(), "ip", rule)
			if err != nil {
				t.Error(err)
				return
			}
			if res.Allowed {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != 50 {
		t.Fatalf("allowed=%d want 50", allowed)
	}
}

type errLimiter struct {
	err error
}

func (e errLimiter) Allow(context.Context, string, Rule) (Result, error) {
	return Result{}, e.err
}

func TestMiddleware_HeadersAnd429(t *testing.T) {
	logger := zap.NewNop().Sugar()
	mw := &Middleware{
		Limiter: NewMemoryLimiter(),
		Respond: httpx.NewResponder(logger),
		Logger:  logger,
		Enabled: true,
	}

	handler := mw.ByIP(Rule{Requests: 1, Window: time.Minute})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("first status=%d", rr.Code)
	}
	if rr.Header().Get("X-RateLimit-Limit") != "1" {
		t.Fatalf("missing limit header: %v", rr.Header())
	}

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d", rr2.Code)
	}
	if rr2.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}

func TestMiddleware_FailOpenOnRedisError(t *testing.T) {
	logger := zap.NewNop().Sugar()
	mw := &Middleware{
		Limiter: errLimiter{err: errors.New("redis down")},
		Respond: httpx.NewResponder(logger),
		Logger:  logger,
		Enabled: true,
	}

	handler := mw.ByIP(Rule{Requests: 1, Window: time.Minute})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("fail-open status=%d", rr.Code)
	}
}

func TestMiddleware_AuthFallbackToMemory(t *testing.T) {
	logger := zap.NewNop().Sugar()
	mw := &Middleware{
		Limiter:  errLimiter{err: errors.New("redis down")},
		Fallback: NewMemoryLimiter(),
		Respond:  httpx.NewResponder(logger),
		Logger:   logger,
		Enabled:  true,
	}

	handler := mw.ByIPAuth(Rule{Requests: 1, Window: time.Minute})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/authentication/token", nil)
	req.RemoteAddr = "9.9.9.9:1"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first fallback status=%d", rr.Code)
	}

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second fallback status=%d", rr2.Code)
	}
}
