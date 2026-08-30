package cache

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// memoryEntry pairs a cached value with its optional expiration.
type memoryEntry struct {
	value   string
	expires time.Time // zero means no expiration
}

// expired reports whether the entry has a deadline that has passed.
func (e memoryEntry) expired(now time.Time) bool {
	return !e.expires.IsZero() && now.After(e.expires)
}

// Memory is the in-process Cache implementation. It keeps every entry in the
// current process, which is the correct behavior for single-instance
// deployments and the fallback when Valkey is not configured.
type Memory struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
}

// NewMemory returns an empty in-process cache backend.
func NewMemory() *Memory {
	return &Memory{entries: make(map[string]memoryEntry)}
}

// Get returns the value stored under key, deleting it lazily when expired.
func (m *Memory) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[keyPrefix+key]
	if !ok {
		return "", false, nil
	}
	if entry.expired(time.Now()) {
		delete(m.entries, keyPrefix+key)
		return "", false, nil
	}
	return entry.value, true, nil
}

// Set stores value under key. A non-positive ttl means no expiration.
func (m *Memory) Set(_ context.Context, key, value string, ttl time.Duration) error {
	entry := memoryEntry{value: value}
	if ttl > 0 {
		entry.expires = time.Now().Add(ttl)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[keyPrefix+key] = entry
	return nil
}

// Delete removes key if present.
func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, keyPrefix+key)
	return nil
}

// GetDel atomically returns and removes the value stored under key.
func (m *Memory) GetDel(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lookupKey := keyPrefix + key
	entry, ok := m.entries[lookupKey]
	delete(m.entries, lookupKey)
	if !ok || entry.expired(time.Now()) {
		return "", false, nil
	}
	return entry.value, true, nil
}

// Incr atomically increments the counter stored at key. The ttl is applied
// only when the call creates the counter, so an existing counter keeps its
// original deadline.
func (m *Memory) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lookupKey := keyPrefix + key
	entry, ok := m.entries[lookupKey]
	if ok && entry.expired(time.Now()) {
		ok = false
	}

	n := int64(0)
	if ok {
		var err error
		n, err = strconv.ParseInt(entry.value, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	n++

	next := memoryEntry{value: strconv.FormatInt(n, 10)}
	if !ok && ttl > 0 {
		next.expires = time.Now().Add(ttl)
	} else if ok {
		next.expires = entry.expires
	}
	m.entries[lookupKey] = next
	return n, nil
}

// Close drops every cached entry.
func (m *Memory) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]memoryEntry)
}
