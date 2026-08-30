package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"golang.org/x/time/rate"
)

// fakeRateLimitStore records Allow calls per key.
type fakeRateLimitStore struct {
	counts map[string]int
	limit  int
	err    error
}

func (f *fakeRateLimitStore) Allow(_ context.Context, key string, limit int, _ time.Duration) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.counts == nil {
		f.counts = make(map[string]int)
	}
	f.limit = limit
	f.counts[key]++
	return f.counts[key] <= limit, nil
}

func newRateLimitRouter(requestsPerMinute int, store RateLimitStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/limited", NewRateLimiter("test", requestsPerMinute, store), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func postFrom(router *gin.Engine, remoteAddr string) int {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/limited", nil)
	req.RemoteAddr = remoteAddr
	router.ServeHTTP(w, req)
	return w.Code
}

func TestRateLimiter_Disabled(t *testing.T) {
	if NewRateLimiter("auth", 0, nil) != nil {
		t.Errorf("expected nil middleware for requestsPerMinute <= 0")
	}
	if NewRateLimiter("auth", -1, nil) != nil {
		t.Errorf("expected nil middleware for negative requestsPerMinute")
	}
}

func TestRateLimiter_AllowsWithinBudget(t *testing.T) {
	router := newRateLimitRouter(5, nil)
	for i := range 5 {
		if code := postFrom(router, "10.0.0.1:1234"); code != http.StatusOK {
			t.Fatalf("request %d: expected 200 within budget, got %d", i, code)
		}
	}
}

func TestRateLimiter_RejectsOverBudget(t *testing.T) {
	router := newRateLimitRouter(3, nil)
	for range 3 {
		if code := postFrom(router, "10.0.0.1:1234"); code != http.StatusOK {
			t.Fatalf("expected 200 within budget, got %d", code)
		}
	}
	if code := postFrom(router, "10.0.0.1:1234"); code != http.StatusTooManyRequests {
		t.Errorf("expected 429 over budget, got %d", code)
	}
}

func TestRateLimiter_PerClientIndependent(t *testing.T) {
	router := newRateLimitRouter(1, nil)

	if code := postFrom(router, "10.0.0.1:1234"); code != http.StatusOK {
		t.Fatalf("expected 200 for first request, got %d", code)
	}
	// A different client IP has its own budget.
	if code := postFrom(router, "10.0.0.2:1234"); code != http.StatusOK {
		t.Errorf("expected 200 for a different client IP, got %d", code)
	}
	// The exhausted client is rejected again.
	if code := postFrom(router, "10.0.0.1:1234"); code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for exhausted client, got %d", code)
	}
}

// TestRateLimiter_KeysIncludeScope checks that the store key carries the
// scope, so endpoint groups sharing one store do not share a budget.
func TestRateLimiter_KeysIncludeScope(t *testing.T) {
	store := &fakeRateLimitStore{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/limited", NewRateLimiter("auth", 1, store), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	if code := postFrom(router, "10.0.0.1:1234"); code != http.StatusOK {
		t.Fatalf("expected 200 for first request, got %d", code)
	}
	if _, ok := store.counts["auth:10.0.0.1"]; !ok {
		t.Fatalf("expected store key %q, got keys %v", "auth:10.0.0.1", store.counts)
	}
}

// TestRateLimiter_FailsOpenOnStoreError checks that a store failure allows
// the request (fail-open) instead of blocking traffic during a backend outage.
func TestRateLimiter_FailsOpenOnStoreError(t *testing.T) {
	router := newRateLimitRouter(1, &fakeRateLimitStore{err: context.DeadlineExceeded})
	for range 3 {
		if code := postFrom(router, "10.0.0.1:1234"); code != http.StatusOK {
			t.Fatalf("expected 200 on store error, got %d", code)
		}
	}
}

func TestMemoryRateLimitStore_BudgetPerKey(t *testing.T) {
	s := newMemoryRateLimitStore()
	ctx := context.Background()

	for i := range 3 {
		allowed, err := s.Allow(ctx, "auth:10.0.0.1", 3, time.Minute)
		if err != nil || !allowed {
			t.Fatalf("request %d: expected allowed, got %v, %v", i, allowed, err)
		}
	}
	if allowed, err := s.Allow(ctx, "auth:10.0.0.1", 3, time.Minute); allowed || err != nil {
		t.Fatalf("expected spent budget (false, nil), got %v, %v", allowed, err)
	}
	// A different key has its own budget.
	if allowed, _ := s.Allow(ctx, "auth:10.0.0.2", 3, time.Minute); !allowed {
		t.Fatal("expected allowed for a different key")
	}
}

// TestMemoryRateLimitStore_FailsClosedAtCap drives the in-memory store to the
// hard cap on tracked keys and checks that unknown keys are rejected (instead
// of the map growing without bound), while an already-tracked key still works.
func TestMemoryRateLimitStore_FailsClosedAtCap(t *testing.T) {
	s := &memoryRateLimitStore{entries: make(map[string]*rateLimitEntry, rateMaxEntries)}
	for i := range rateMaxEntries {
		key := "auth:10.1." + strconv.Itoa(i/256) + "." + strconv.Itoa(i%256)
		s.entries[key] = &rateLimitEntry{limiter: rate.NewLimiter(rate.Every(time.Minute), 1), lastSeen: time.Now()}
	}
	ctx := context.Background()

	// A tracked key still passes.
	if allowed, _ := s.Allow(ctx, "auth:10.1.0.0", 1, time.Minute); !allowed {
		t.Error("expected allowed for tracked key at cap")
	}
	// An unknown key is rejected instead of adding a new entry.
	if allowed, err := s.Allow(ctx, "auth:10.200.0.1", 1, time.Minute); allowed || err != nil {
		t.Errorf("expected spent budget for unknown key at cap, got %v, %v", allowed, err)
	}
	if len(s.entries) != rateMaxEntries {
		t.Errorf("expected entry count to stay at the cap, got %d", len(s.entries))
	}
}

// TestMemoryRateLimitStore_SweepsIdleEntries checks that once the sweep
// threshold is reached, entries idle beyond the TTL are evicted and their keys
// get a fresh budget.
func TestMemoryRateLimitStore_SweepsIdleEntries(t *testing.T) {
	s := newMemoryRateLimitStore()
	// An exhausted entry that went idle long ago.
	s.entries["auth:10.0.0.1"] = &rateLimitEntry{
		limiter:  rate.NewLimiter(rate.Every(time.Minute), 1),
		lastSeen: time.Now().Add(-2 * rateEntryIdleTTL),
	}
	s.entries["auth:10.0.0.1"].limiter.Allow()
	// Push the map over the sweep threshold.
	for i := range rateSweepThreshold {
		key := "auth:10.2." + strconv.Itoa(i/256) + "." + strconv.Itoa(i%256)
		s.entries[key] = &rateLimitEntry{limiter: rate.NewLimiter(rate.Every(time.Minute), 1), lastSeen: time.Now()}
	}

	if allowed, _ := s.Allow(context.Background(), "auth:10.0.0.1", 1, time.Minute); !allowed {
		t.Error("expected allowed after idle entry was swept")
	}
}

// fakeCounterStore is an in-memory CounterStore fake.
type fakeCounterStore struct {
	counts map[string]int64
	ttls   map[string]time.Duration
	err    error
}

func (f *fakeCounterStore) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	if f.counts == nil {
		f.counts = make(map[string]int64)
		f.ttls = make(map[string]time.Duration)
	}
	f.counts[key]++
	if f.counts[key] == 1 {
		f.ttls[key] = ttl
	}
	return f.counts[key], nil
}

func TestFixedWindowRateLimitStore(t *testing.T) {
	counters := &fakeCounterStore{}
	s := NewFixedWindowRateLimitStore(counters)
	ctx := context.Background()

	for i := range 3 {
		allowed, err := s.Allow(ctx, "auth:10.0.0.1", 3, time.Minute)
		if err != nil || !allowed {
			t.Fatalf("request %d: expected allowed, got %v, %v", i, allowed, err)
		}
	}
	if allowed, err := s.Allow(ctx, "auth:10.0.0.1", 3, time.Minute); allowed || err != nil {
		t.Fatalf("expected spent budget, got %v, %v", allowed, err)
	}
	// Counters are namespaced under "rl:" and carry the window as ttl.
	if _, ok := counters.counts["rl:auth:10.0.0.1"]; !ok {
		t.Fatalf("expected counter key %q, got %v", "rl:auth:10.0.0.1", counters.counts)
	}
	if ttl := counters.ttls["rl:auth:10.0.0.1"]; ttl != time.Minute {
		t.Errorf("expected counter ttl of one minute, got %v", ttl)
	}
}

func TestFixedWindowRateLimitStore_PropagatesErrors(t *testing.T) {
	s := NewFixedWindowRateLimitStore(&fakeCounterStore{err: context.DeadlineExceeded})
	if _, err := s.Allow(context.Background(), "auth:10.0.0.1", 3, time.Minute); err == nil {
		t.Fatal("expected error from store backend, got nil")
	}
}
