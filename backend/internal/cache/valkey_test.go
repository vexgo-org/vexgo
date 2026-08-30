package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestValkey(t *testing.T) (*Valkey, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	c, err := NewValkey(context.Background(), "redis://"+mr.Addr())
	if err != nil {
		t.Fatalf("NewValkey: %v", err)
	}
	t.Cleanup(c.Close)
	return c, mr
}

func TestValkeyCache(t *testing.T) {
	c, _ := newTestValkey(t)
	exerciseCache(t, c)
}

func TestValkeyCacheExpiry(t *testing.T) {
	c, mr := newTestValkey(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	mr.FastForward(2 * time.Minute)
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("Get(k) after ttl = hit; want miss")
	}
}

func TestValkeyCacheIncrAppliesTTLToNewCounterOnly(t *testing.T) {
	c, mr := newTestValkey(t)
	ctx := context.Background()

	if n, err := c.Incr(ctx, "c", time.Minute); err != nil || n != 1 {
		t.Fatalf("Incr = %d, %v; want 1, nil", n, err)
	}
	// Existing counter keeps its original deadline across increments.
	mr.FastForward(30 * time.Second)
	if n, _ := c.Incr(ctx, "c", time.Minute); n != 2 {
		t.Fatalf("Incr = %d; want 2", n)
	}
	mr.FastForward(31 * time.Second)
	if n, _ := c.Incr(ctx, "c", time.Minute); n != 1 {
		t.Fatalf("Incr after expiry = %d; want 1", n)
	}
}

func TestValkeyCacheKeysAreNamespaced(t *testing.T) {
	c, mr := newTestValkey(t)
	if err := c.Set(context.Background(), "k", "v", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := mr.Get(keyPrefix + "k"); err != nil {
		t.Fatalf("key not stored under %q prefix: %v", keyPrefix, err)
	}
}

func TestValkeyCacheConnectionError(t *testing.T) {
	if _, err := NewValkey(context.Background(), "redis://127.0.0.1:1"); err == nil {
		t.Fatal("NewValkey against a dead port = nil error; want failure")
	}
}

func TestValkeyCacheInvalidURL(t *testing.T) {
	if _, err := NewValkey(context.Background(), "http://127.0.0.1"); err == nil {
		t.Fatal("NewValkey with invalid scheme = nil error; want failure")
	}
}

func TestValkeyCacheServerError(t *testing.T) {
	c, mr := newTestValkey(t)
	mr.Close()
	if _, ok, err := c.Get(context.Background(), "k"); err == nil || ok {
		t.Fatalf("Get against closed server = ok=%v, err=%v; want error", ok, err)
	}
}

func TestValkeyNewValkeyPingFailure(t *testing.T) {
	// miniredis that accepts connections but responds with an error to PING
	// proves the startup PING fails the constructor.
	mr := miniredis.RunT(t)
	mr.SetError("boom")
	if _, err := NewValkey(context.Background(), "redis://"+mr.Addr()); err == nil {
		t.Fatal("NewValkey against erroring server = nil error; want failure")
	}
}
