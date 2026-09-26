package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

var (
	ErrInvalidKey = errors.New("invalid storage key")
	ErrNotFound   = errors.New("storage object not found")
)

// Storage abstracts the file backend (S3-compatible or local disk). It is an
// external-dependency seam so handlers can be tested without a real bucket.
type Storage interface {
	Put(
		ctx context.Context,
		key string,
		reader io.Reader,
		size int64,
		contentType string,
	) error

	Open(ctx context.Context, key string) (io.ReadCloser, error)

	Delete(ctx context.Context, key string) error

	URL(ctx context.Context, key string) (string, error)
}

func unwrapCleanKey(key string) (string, error) {
	cleanedKey, err := cleanKey(key)
	if err != nil {
		return "", fmt.Errorf("%w: %q", err, key)
	}
	return cleanedKey, nil
}

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
