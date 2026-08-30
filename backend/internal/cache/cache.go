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
	"time"
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
