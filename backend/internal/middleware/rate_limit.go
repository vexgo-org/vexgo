// Package middleware provides shared HTTP middlewares.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"golang.org/x/time/rate"
)

const (
	// rateEntryIdleTTL is how long a client IP entry is kept without any
	// request before it becomes eligible for eviction.
	rateEntryIdleTTL = 10 * time.Minute
	// rateSweepThreshold is the map size that triggers a sweep of idle
	// entries.
	rateSweepThreshold = 4096
	// rateMaxEntries is the hard cap on tracked client IPs. Once the cap is
	// reached (e.g. an attacker rotating across many IPv6 addresses), IPs
	// without an existing entry are rejected with 429 so the map cannot grow
	// without bound.
	rateMaxEntries = 65536
)

// RateLimitStore is the consumer-declared seam for per-key request budgeting.
// NewRateLimiter keeps the budget in-process when no store is given; a
// distributed store lets multiple instances share one budget.
type RateLimitStore interface {
	// Allow consumes one request for key against a budget of limit requests
	// per window. A spent budget is (false, nil); an error reports a backend
	// failure, which callers may treat differently from a denial.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// CounterStore is the consumer-declared seam for the atomic counters the
// fixed-window store builds on. It is satisfied structurally by the cache
// backends (internal/cache) without middleware depending on them.
type CounterStore interface {
	// Incr atomically increments the counter stored at key, applying ttl as
	// its expiration when the call creates the counter.
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// NewFixedWindowRateLimitStore returns a distributed RateLimitStore backed by
// atomic counters: each key's counter expires one window after its first
// increment, so every window allows limit requests. Window borders allow up
// to twice the budget across two adjacent windows, which is acceptable for
// abuse protection.
func NewFixedWindowRateLimitStore(counters CounterStore) RateLimitStore {
	return &fixedWindowRateLimitStore{counters: counters}
}

type fixedWindowRateLimitStore struct {
	counters CounterStore
}

// Allow increments the key's counter and allows the request while the window
// budget is not spent.
func (s *fixedWindowRateLimitStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		// Defensive: a non-positive budget denies instead of dividing by it.
		return false, nil
	}
	n, err := s.counters.Incr(ctx, "rl:"+key, window)
	if err != nil {
		return false, err
	}
	return n <= int64(limit), nil
}

// rateLimitEntry pairs a client's token bucket with its last-seen time.
type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// memoryRateLimitStore keeps per-key token buckets in the process.
type memoryRateLimitStore struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
}

func newMemoryRateLimitStore() *memoryRateLimitStore {
	return &memoryRateLimitStore{entries: make(map[string]*rateLimitEntry)}
}

// Allow consumes one request from the key's token bucket. Buckets start full
// (burst = limit) and refill to limit per window, matching the fixed-window
// budget. Sweep and the entry cap keep the map bounded.
func (s *memoryRateLimitStore) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		// Defensive: a non-positive budget denies instead of dividing by it.
		return false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Sweep idle entries first so a client whose entry was just evicted gets
	// a fresh budget instead of being judged on the stale limiter.
	if len(s.entries) >= rateSweepThreshold {
		s.sweepLocked()
	}
	entry, ok := s.entries[key]
	if !ok {
		if len(s.entries) >= rateMaxEntries {
			// Fail closed: refuse to track more clients than the cap.
			return false, nil
		}
		entry = &rateLimitEntry{limiter: rate.NewLimiter(rate.Every(window/time.Duration(limit)), limit)}
		s.entries[key] = entry
	}
	allowed := entry.limiter.Allow()
	entry.lastSeen = time.Now()
	return allowed, nil
}

// sweepLocked drops entries idle beyond rateEntryIdleTTL. Callers must hold
// s.mu.
func (s *memoryRateLimitStore) sweepLocked() {
	cutoff := time.Now().Add(-rateEntryIdleTTL)
	for key, entry := range s.entries {
		if entry.lastSeen.Before(cutoff) {
			delete(s.entries, key)
		}
	}
}

// NewRateLimiter returns a gin middleware that allows each client IP
// requestsPerMinute requests per minute (bursting up to requestsPerMinute).
// It is meant for expensive unauthenticated endpoints such as captcha
// generation. requestsPerMinute <= 0 disables the limiter (nil returned).
// scope namespaces the budget key, so endpoint groups sharing one store do
// not share a budget. A nil store keeps the budget in-process.
func NewRateLimiter(scope string, requestsPerMinute int, store RateLimitStore) gin.HandlerFunc {
	if requestsPerMinute <= 0 {
		return nil
	}
	if store == nil {
		store = newMemoryRateLimitStore()
	}

	return func(c *gin.Context) {
		allowed, err := store.Allow(c.Request.Context(), scope+":"+c.ClientIP(), requestsPerMinute, time.Minute)
		if err != nil {
			// Fail open: availability wins over abuse protection, which is a
			// soft target. The error is logged so an ailing store is visible
			// in production.
			slog.Error("rate limit store unavailable, allowing request", "scope", scope, "err", err)
			c.Next()
			return
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests, please try again later",
			})
			return
		}

		c.Next()
	}
}
