package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/vexgo-org/vexgo/backend/internal/config"
)

// S3Storage stores files in an S3-compatible bucket.
type S3Storage struct {
	// client is nil until NewS3Storage verified the bucket, so every method
	// checks it rather than trusting the wiring.
	client *minio.Client
	cfg    *config.S3Config
}

// NewS3Storage initializes the MinIO client and verifies the bucket,
// mirroring the previous handler.InitS3 behavior.
func NewS3Storage(ctx context.Context, cfg *config.S3Config) (*S3Storage, error) {
	if !cfg.IsEnabled() {
		return nil, nil
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("S3 configuration error: %w", err)
	}

	// Strip protocol prefix from endpoint, minio-go manages SSL separately
	endpoint := cfg.Endpoint
	useSSL := true
	if after, ok := strings.CutPrefix(endpoint, "http://"); ok {
		endpoint = after
		useSSL = false
	} else if after, ok := strings.CutPrefix(endpoint, "https://"); ok {
		endpoint = after
		useSSL = true
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: useSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// Verify connectivity by checking if the target bucket exists
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to S3: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bucket %s does not exist", cfg.Bucket)
	}

	slog.Info("s3 storage initialized successfully")
	return &S3Storage{client: client, cfg: cfg}, nil
}

// Put uploads the object under key. An empty contentType falls back to
// extension-based detection. size is handed to minio-go as the object size; -1
// leaves the multipart decision to the client.
func (s *S3Storage) Put(
	ctx context.Context,
	key string,
	reader io.Reader,
	size int64,
	contentType string,
) error {
	if err := s.checkClientInitialized(); err != nil {
		return err
	}

	cleanedKey, err := unwrapCleanKey(key)
	if err != nil {
		return err
	}

	if contentType == "" {
		contentType = detectContentType(cleanedKey)
	}

	_, err = s.client.PutObject(
		ctx,
		s.cfg.Bucket,
		cleanedKey,
		reader,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		return fmt.Errorf("put S3 object failed: %w", err)
	}

	return nil
}

// Open returns a reader over the stored object. GetObject defers the request
// until the first read, so a missing object only surfaces here — hence the
// Stat, which turns that into ErrNotFound instead of a read-time error.
func (s *S3Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := s.checkClientInitialized(); err != nil {
		return nil, err
	}

	cleanedKey, err := unwrapCleanKey(key)
	if err != nil {
		return nil, err
	}

	object, err := s.client.GetObject(
		ctx,
		s.cfg.Bucket,
		cleanedKey,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("open S3 object failed: %w", err)
	}

	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		resp := minio.ToErrorResponse(err)
		if resp.Code == minio.NoSuchKey ||
			resp.Code == "NoSuchObject" ||
			resp.StatusCode == 404 {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("stat S3 object failed: %w", err)
	}

	return object, nil
}

// Delete removes the object. S3 treats removing a missing key as success, so
// this stays idempotent like the local backend.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	if err := s.checkClientInitialized(); err != nil {
		return err
	}

	cleanedKey, err := unwrapCleanKey(key)
	if err != nil {
		return err
	}

	if err = s.client.RemoveObject(
		ctx,
		s.cfg.Bucket,
		cleanedKey,
		minio.RemoveObjectOptions{},
	); err != nil {
		return fmt.Errorf("delete S3 object failed: %w", err)
	}

	return nil
}

// URL returns the bucket's public URL for the key, honoring the custom domain
// and path-style settings from the S3 config.
func (s *S3Storage) URL(_ context.Context, key string) (string, error) {
	cleanedKey, err := unwrapCleanKey(key)
	if err != nil {
		return "", err
	}
	return s.cfg.GetURL(cleanedKey), nil
}

// checkClientInitialized guards the zero value: NewS3Storage returns a nil
// *S3Storage when S3 is disabled, and app only keeps it when it is non-nil.
func (s *S3Storage) checkClientInitialized() error {
	if s.client == nil {
		return errors.New("S3 storage not initialized")
	}
	return nil
}

// detectContentType returns the MIME type based on the file extension.
// Defaults to application/octet-stream for unknown types.
func detectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}
