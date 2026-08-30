// Package middleware provides shared HTTP middlewares.
package middleware

import (
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

// rateLimitEntry pairs a client's token bucket with its last-seen time.
type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter enforces a per-client-IP request budget. Client IPs come from
// c.ClientIP(), which honors gin's trusted-proxy configuration, so the key is
// only as trustworthy as that setup.
type ipRateLimiter struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	entries map[string]*rateLimitEntry
}

// NewRateLimiter returns a gin middleware that allows each client IP
// requestsPerMinute requests per minute (bursting up to requestsPerMinute).
// It is meant for expensive unauthenticated endpoints such as captcha
// generation. requestsPerMinute <= 0 disables the limiter (nil returned).
func NewRateLimiter(requestsPerMinute int) gin.HandlerFunc {
	if requestsPerMinute <= 0 {
		return nil
	}

	rl := &ipRateLimiter{
		limit:   rate.Every(time.Minute / time.Duration(requestsPerMinute)),
		burst:   requestsPerMinute,
		entries: make(map[string]*rateLimitEntry),
	}
	return rl.handle
}

func (rl *ipRateLimiter) handle(c *gin.Context) {
	ip := c.ClientIP()

	rl.mu.Lock()
	// Sweep idle entries first so a client whose entry was just evicted gets
	// a fresh budget instead of being judged on the stale limiter.
	if len(rl.entries) >= rateSweepThreshold {
		rl.sweepLocked()
	}
	entry, ok := rl.entries[ip]
	if !ok {
		if len(rl.entries) >= rateMaxEntries {
			// Fail closed: refuse to track more clients than the cap.
			rl.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests, please try again later",
			})
			return
		}
		entry = &rateLimitEntry{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.entries[ip] = entry
	}
	allowed := entry.limiter.Allow()
	entry.lastSeen = time.Now()
	rl.mu.Unlock()

	if !allowed {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error": "Too many requests, please try again later",
		})
		return
	}

	c.Next()
}

// sweepLocked drops entries idle beyond rateEntryIdleTTL. Callers must hold
// rl.mu.
func (rl *ipRateLimiter) sweepLocked() {
	cutoff := time.Now().Add(-rateEntryIdleTTL)
	for ip, entry := range rl.entries {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.entries, ip)
		}
	}
}
