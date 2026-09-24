package storage

import (
	"context"
	"io"
)

// Storage abstracts the file backend (S3-compatible or local disk). It is an
// external-dependency seam so handlers can be tested without a real bucket.
type Storage interface {
	// Upload stores the file content under a generated key and returns its
	// public URL.
	Upload(ctx context.Context, reader io.Reader, filename, contentType string) (string, error)
	// Delete removes the file identified by its public URL. Missing files are
	// not an error.
	Delete(ctx context.Context, url string) error
}
