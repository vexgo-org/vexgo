// Package cache provides the cache backends shared by the application: an
// in-process memory implementation and a Valkey (Redis-compatible)
// implementation backed by github.com/valkey-io/valkey-go.
//
// The package is a leaf: it imports no other backend internal package, so the
// third-party valkey-go dependency stays confined here. Consumers declare
// their own narrow interfaces over the primitives they use and the
// composition root (internal/app) injects the concrete backend.
//
// Every key is namespaced with a fixed prefix so the server can share a
// Valkey instance with unrelated applications.
//
// Security note: the cache backend is a trusted component. Everything written
// to it is decoded and served to users, so callers must only store data users
// are allowed to see (the post decorator filters author data to guest
// visibility for exactly this reason). Run the server on a private network
// with authentication, and TLS (rediss:// / valkeys://) where appropriate.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

// keyPrefix namespaces every key written by this application.
const keyPrefix = "vexgo:"

// Cache is the storage seam for cacheable application state. Implementations
// must be safe for concurrent use.
type Cache interface {
	// Get returns the value stored under key, or ok=false when the key is
	// absent or expired.
	Get(ctx context.Context, key string) (value string, ok bool, err error)

	// Set stores value under key. A positive ttl expires the entry after the
	// given duration; a non-positive ttl stores it without expiration.
	Set(ctx context.Context, key, value string, ttl time.Duration) error

	// Delete removes key if present.
	Delete(ctx context.Context, key string) error

	// GetDel atomically returns and removes the value stored under key. It
	// reports ok=false when the key is absent or expired.
	GetDel(ctx context.Context, key string) (value string, ok bool, err error)

	// Incr atomically increments the counter stored at key and returns the
	// new value. When the call creates the counter, ttl is applied as its
	// expiration; increments of an existing counter keep that expiration.
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)

	// Close releases the underlying resources. It must be safe to call on
	// backends that hold none.
	Close()
}

// NewValkey opens a connection to the Valkey / Redis-compatible server at
// url (e.g. "valkey://127.0.0.1:6379/0", "redis://:password@host:6379",
// "rediss://host:6379" for TLS) and verifies it with a PING before returning,
// so misconfiguration fails at startup instead of on first use.
func NewValkey(ctx context.Context, url string) (*Valkey, error) {
	opt, err := valkey.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse valkey URL: %w", err)
	}
	// Client-side caching (the DoCache API) is never used, so the CLIENT
	// TRACKING opt-in handshake is disabled. This also keeps the client
	// compatible with servers and proxies that lack RESP3 tracking.
	opt.DisableCache = true
	// Command retries (20 attempts with backoff by default) are disabled:
	// callers run on the request path and own their failure semantics (rate
	// limiting fails open, SSO state fails closed), which needs fast errors,
	// not a hidden ~20s stall per command during an outage.
	opt.DisableRetry = true
	client, err := valkey.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("connect valkey: %w", err)
	}
	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping valkey: %w", err)
	}
	return &Valkey{client: client}, nil
}
