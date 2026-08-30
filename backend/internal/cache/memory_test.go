package cache

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// exerciseCache runs the behavior every Cache implementation must provide, so
// the memory and valkey backends cannot drift apart.
func exerciseCache(t *testing.T, c Cache) {
	t.Helper()
	ctx := context.Background()

	// Get on a missing key is a miss, not an error.
	if value, ok, err := c.Get(ctx, "missing"); err != nil || ok || value != "" {
		t.Fatalf("Get(missing) = %q, %v, %v; want empty miss", value, ok, err)
	}

	if err := c.Set(ctx, "k", "v1", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if value, ok, err := c.Get(ctx, "k"); err != nil || !ok || value != "v1" {
		t.Fatalf("Get(k) = %q, %v, %v; want v1 hit", value, ok, err)
	}

	// Set overwrites.
	if err := c.Set(ctx, "k", "v2", time.Minute); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	if value, _, _ := c.Get(ctx, "k"); value != "v2" {
		t.Fatalf("Get(k) after overwrite = %q; want v2", value)
	}

	// Delete removes the key.
	if err := c.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("Get(k) after Delete = hit; want miss")
	}

	// GetDel returns and removes in one step.
	if err := c.Set(ctx, "once", "token", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if value, ok, err := c.GetDel(ctx, "once"); err != nil || !ok || value != "token" {
		t.Fatalf("GetDel(once) = %q, %v, %v; want token", value, ok, err)
	}
	if _, ok, _ := c.GetDel(ctx, "once"); ok {
		t.Fatal("second GetDel(once) = hit; want miss")
	}

	// Incr starts at 1 and keeps counting.
	n, err := c.Incr(ctx, "counter", time.Minute)
	if err != nil || n != 1 {
		t.Fatalf("Incr = %d, %v; want 1, nil", n, err)
	}
	if n, _ := c.Incr(ctx, "counter", time.Minute); n != 2 {
		t.Fatalf("second Incr = %d; want 2", n)
	}
}

func TestMemoryCache(t *testing.T) {
	exerciseCache(t, NewMemory())
}

func TestMemoryCacheExpiry(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	if err := c.Set(ctx, "k", "v", 20*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("Get(k) after ttl = hit; want miss")
	}
}

func TestMemoryCacheIncrAppliesTTLToNewCounterOnly(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	if n, _ := c.Incr(ctx, "c", 20*time.Millisecond); n != 1 {
		t.Fatalf("Incr = %d; want 1", n)
	}
	time.Sleep(30 * time.Millisecond)
	// The expired counter is treated as new: it restarts at 1 and the ttl is
	// applied again.
	if n, _ := c.Incr(ctx, "c", time.Minute); n != 1 {
		t.Fatalf("Incr after expiry = %d; want 1", n)
	}
}

func TestMemoryCacheKeysAreNamespaced(t *testing.T) {
	c := NewMemory()
	if err := c.Set(context.Background(), "k", "v", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, ok := c.entries[keyPrefix+"k"]; !ok {
		t.Fatalf("key stored without %q prefix", keyPrefix)
	}
}

func TestMemoryCacheNoTTLMeansNoExpiration(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	if err := c.Set(ctx, "k", "v", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// A zero ttl must not expire immediately: the entry stays retrievable
	// (past time.Now() either way).
	if value, ok, _ := c.Get(ctx, "k"); !ok || value != "v" {
		t.Fatalf("Get(k) = %q, %v; want hit", value, ok)
	}
}

// TestMemoryCache_SweepsExpiredEntries checks that expired entries are swept
// once the map passes the sweep threshold, so keys that are never read again
// cannot accumulate.
func TestMemoryCache_SweepsExpiredEntries(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	// One long-lived entry and one that is already expired.
	if err := c.Set(ctx, "live", "v", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.Set(ctx, "dead", "v", time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	// Push the map over the sweep threshold.
	for i := range memorySweepThreshold {
		if err := c.Set(ctx, strconv.Itoa(i), "v", time.Minute); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	c.mu.Lock()
	_, deadExists := c.entries[keyPrefix+"dead"]
	_, liveExists := c.entries[keyPrefix+"live"]
	c.mu.Unlock()

	if deadExists {
		t.Fatal("expired entry survived the sweep")
	}
	if !liveExists {
		t.Fatal("live entry was swept")
	}
}

// TestMemoryCache_StopsStoringAtCap checks that new keys past the hard cap
// are not stored (reads become misses) while existing keys stay overwritable.
func TestMemoryCache_StopsStoringAtCap(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	for i := range memoryMaxEntries {
		if err := c.Set(ctx, strconv.Itoa(i), "v", time.Minute); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if err := c.Set(ctx, "overflow", "v", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, ok, _ := c.Get(ctx, "overflow"); ok {
		t.Fatal("entry stored past the cap; want miss")
	}
	// Overwriting an existing key still works at the cap.
	if err := c.Set(ctx, "0", "v2", time.Minute); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	if value, ok, _ := c.Get(ctx, "0"); !ok || value != "v2" {
		t.Fatalf("Get(0) = %q, %v; want v2 hit", value, ok)
	}
}
