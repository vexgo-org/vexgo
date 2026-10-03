// Package storage provides the file backends behind the Storage seam: local
// disk by default, any S3-compatible object storage when S3 is enabled. The
// upload domain stores media through it and keeps the key it was given.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// Sentinel errors shared by every backend, so a caller can match on them
// without knowing which one it got.
var (
	// ErrInvalidKey means the key is empty, absolute, or escapes the backend root.
	ErrInvalidKey = errors.New("invalid storage key")
	// ErrNotFound means no object is stored under the key.
	ErrNotFound = errors.New("storage object not found")
)

// Storage abstracts the file backend (S3-compatible or local disk). It is an
// external-dependency seam so handlers can be tested without a real bucket.
// Every method addresses an object by key, never by URL: turning a key into a
// public URL is the backend's job (URL), and the key is what the caller stores
// alongside the URL so it can reach the object again.
type Storage interface {
	// Put stores reader under key. An empty contentType lets the backend derive
	// one from the key, and a negative size means "unknown": LocalStorage then
	// skips its length check, S3 hands it to minio-go. A rejected or truncated
	// write must leave nothing readable at key: there is no media record to
	// delete it through the API, so a partial object would only accumulate as an
	// orphan.
	Put(
		ctx context.Context,
		key string,
		reader io.Reader,
		size int64,
		contentType string,
	) error

	// Open returns a reader over the stored object, or ErrNotFound when the key
	// holds nothing. The caller closes it.
	Open(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object. A key that holds nothing is not an error, so
	// deleting an already-deleted object stays idempotent.
	Delete(ctx context.Context, key string) error

	// URL returns the public URL the object is served from. It does not check
	// that the object exists.
	URL(ctx context.Context, key string) (string, error)
}

// unwrapCleanKey cleans key and names the offending value in the error, so the
// backends report a bad key identically without repeating the plumbing.
func unwrapCleanKey(key string) (string, error) {
	cleanedKey, err := cleanKey(key)
	if err != nil {
		return "", fmt.Errorf("%w: %q", err, key)
	}
	return cleanedKey, nil
}

// cleanKey normalizes an object key and refuses the shapes that could escape
// the backend root: empty keys, Windows separators, absolute paths, and ".."
// segments. LocalStorage confines itself at the OS level through os.Root; this
// keeps the rejection early and identical on S3, which has no such root.
func cleanKey(key string) (string, error) {
	if key == "" {
		return "", ErrInvalidKey
	}

	if strings.ContainsRune(key, '\\') {
		return "", ErrInvalidKey
	}

	if strings.HasPrefix(key, "/") {
		return "", ErrInvalidKey
	}

	cleaned := path.Clean(key)
	if cleaned == "." ||
		cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") {
		return "", ErrInvalidKey
	}

	return cleaned, nil
}
