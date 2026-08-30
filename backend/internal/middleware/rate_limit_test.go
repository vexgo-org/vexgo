package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"golang.org/x/time/rate"
)

func rateEveryMinute(n int) rate.Limit {
	return rate.Every(time.Minute / time.Duration(n))
}

func newRateLimiter(limit rate.Limit, burst int) *rate.Limiter {
	return rate.NewLimiter(limit, burst)
}

func newRateLimitRouter(t *testing.T, requestsPerMinute int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/limited", NewRateLimiter(requestsPerMinute), func(c *gin.Context) {
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
	if NewRateLimiter(0) != nil {
		t.Errorf("expected nil middleware for requestsPerMinute <= 0")
	}
	if NewRateLimiter(-1) != nil {
		t.Errorf("expected nil middleware for negative requestsPerMinute")
	}
}

func TestRateLimiter_AllowsWithinBudget(t *testing.T) {
	router := newRateLimitRouter(t, 5)
	for i := range 5 {
		if code := postFrom(router, "10.0.0.1:1234"); code != http.StatusOK {
			t.Fatalf("request %d: expected 200 within budget, got %d", i, code)
		}
	}
}

func TestRateLimiter_RejectsOverBudget(t *testing.T) {
	router := newRateLimitRouter(t, 3)
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
	router := newRateLimitRouter(t, 1)

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

// TestRateLimiter_FailsClosedAtCap drives the limiter to the hard cap on
// tracked client IPs and checks that unknown clients are rejected (instead of
// the map growing without bound), while an already-tracked client still works.
func TestRateLimiter_FailsClosedAtCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := &ipRateLimiter{
		limit:   rateEveryMinute(1000),
		burst:   1000,
		entries: make(map[string]*rateLimitEntry, rateMaxEntries),
	}
	for i := range rateMaxEntries {
		rl.entries["10.1."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256)] = &rateLimitEntry{limiter: newRateLimiter(rl.limit, rl.burst), lastSeen: time.Now()}
	}

	router := gin.New()
	router.POST("/limited", rl.handle, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// A tracked client still passes.
	if code := postFrom(router, "10.1.0.0:1234"); code != http.StatusOK {
		t.Errorf("expected 200 for tracked client at cap, got %d", code)
	}
	// An unknown client is rejected instead of adding a new entry.
	if code := postFrom(router, "10.200.0.1:1234"); code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for unknown client at cap, got %d", code)
	}
	if len(rl.entries) != rateMaxEntries {
		t.Errorf("expected entry count to stay at the cap, got %d", len(rl.entries))
	}
}

// TestRateLimiter_SweepsIdleEntries checks that once the sweep threshold is
// reached, entries idle beyond the TTL are evicted and their clients get a
// fresh budget.
func TestRateLimiter_SweepsIdleEntries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := &ipRateLimiter{
		limit:   rateEveryMinute(1),
		burst:   1,
		entries: make(map[string]*rateLimitEntry),
	}
	// An exhausted entry that went idle long ago.
	rl.entries["10.0.0.1"] = &rateLimitEntry{
		limiter:  newRateLimiter(rl.limit, rl.burst),
		lastSeen: time.Now().Add(-2 * rateEntryIdleTTL),
	}
	rl.entries["10.0.0.1"].limiter.Allow()
	// Push the map over the sweep threshold.
	for i := range rateSweepThreshold {
		rl.entries["10.2."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256)] = &rateLimitEntry{limiter: newRateLimiter(rl.limit, rl.burst), lastSeen: time.Now()}
	}

	router := gin.New()
	router.POST("/limited", rl.handle, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	if code := postFrom(router, "10.0.0.1:1234"); code != http.StatusOK {
		t.Errorf("expected 200 after idle entry was swept, got %d", code)
	}
}
