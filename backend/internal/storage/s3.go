package storage

import (
	"context"
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

// Upload uploads a file to the configured S3 bucket.
// Passing size as -1 lets minio-go handle multipart upload automatically.
// Returns the public URL of the uploaded file.
func (s *S3Storage) Upload(ctx context.Context, reader io.Reader, filename, contentType string) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("S3 storage not initialized")
	}

	// Fall back to extension-based detection if content type is not provided
	if contentType == "" {
		contentType = detectContentType(filename)
	}

	_, err := s.client.PutObject(ctx, s.cfg.Bucket, filename, reader, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	url := s.cfg.GetURL(filename)
	slog.Debug("file uploaded successfully", "url", url)
	return url, nil
}

// Delete removes an object from the configured S3 bucket by extracting the key
// from its public URL (with a filename fallback).
func (s *S3Storage) Delete(ctx context.Context, url string) error {
	if s.client == nil {
		return fmt.Errorf("S3 storage not initialized")
	}

	key := ExtractS3Key(url, s.cfg)
	if key != "" {
		return s.remove(ctx, key)
	}

	// If key extraction failed, try to delete using filename as key (fallback)
	filename := filepath.Base(url)
	if filename != "" && filename != "/" {
		return s.remove(ctx, filename)
	}
	return fmt.Errorf("invalid S3 key from URL: %s", url)
}

// remove deletes the object with the given key from the configured bucket.
func (s *S3Storage) remove(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.cfg.Bucket, key, minio.RemoveObjectOptions{})
}

// ExtractS3Key extracts the S3 object key from a URL.
// URL format examples:
//   - S3: https://bucket.s3.region.amazonaws.com/path/to/file.jpg
//   - Custom domain: https://cdn.example.com/path/to/file.jpg
//   - Path style: https://s3.amazonaws.com/bucket/path/to/file.jpg
func ExtractS3Key(url string, cfg *config.S3Config) string {
	// Remove protocol
	if after, ok := strings.CutPrefix(url, "http://"); ok {
		url = after
	} else if after, ok := strings.CutPrefix(url, "https://"); ok {
		url = after
	}

	// Split by "/"
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return ""
	}

	// If using custom domain, check if bucket is included in URL
	customDomain := cfg.CustomDomain
	if after, ok := strings.CutPrefix(customDomain, "http://"); ok {
		customDomain = after
	} else if after, ok := strings.CutPrefix(customDomain, "https://"); ok {
		customDomain = after
	}
	if customDomain != "" {
		if len(parts) > 1 {
			if !cfg.DisableBucketInCustomURL {
				// Format: customdomain/bucket/key -> skip domain and bucket
				if len(parts) >= 3 {
					return strings.Join(parts[2:], "/")
				}
				return ""
			}
			// Format: customdomain/key -> skip domain
			return strings.Join(parts[1:], "/")
		}
		return ""
	}

	// For path-style URLs (ForcePath = true)
	if cfg.ForcePath {
		// Format: endpoint/bucket/key
		if len(parts) >= 3 {
			// Skip endpoint and bucket
			return strings.Join(parts[2:], "/")
		}
		return ""
	}

	// For virtual-hosted style (default AWS S3)
	// Format: bucket.s3.region.amazonaws.com/key
	// or bucket.endpoint.com/key
	if len(parts) >= 2 {
		// Skip the first part (bucket.s3... or bucket)
		return strings.Join(parts[1:], "/")
	}

	return ""
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
