package storage

import (
	"context"
	"io"
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
