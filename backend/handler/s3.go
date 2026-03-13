package handler

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"vexgo/backend/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	S3Client     *minio.Client    // MinIO client instance
	S3Cfg        *config.S3Config // S3 configuration reference
	UseS3Storage bool             // Flag indicating whether S3 storage is active
)

// InitS3 initializes the MinIO client and verifies the connection.
// Returns an error if configuration is invalid or the bucket is unreachable.
func InitS3(cfg *config.S3Config) error {
	if !cfg.IsEnabled() {
		UseS3Storage = false
		return nil
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("S3 configuration error: %w", err)
	}

	// Strip protocol prefix from endpoint, minio-go manages SSL separately
	endpoint := cfg.Endpoint
	useSSL := true
	if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
		useSSL = false
	} else if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		useSSL = true
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	// Create MinIO client with static credentials
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: useSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return fmt.Errorf("failed to create minio client: %w", err)
	}

	// Verify connectivity by checking if the target bucket exists
	exists, err := client.BucketExists(context.TODO(), cfg.Bucket)
	if err != nil {
		return fmt.Errorf("failed to connect to S3: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket %s does not exist", cfg.Bucket)
	}

	S3Client = client
	S3Cfg = cfg
	UseS3Storage = true

	fmt.Printf("S3: Connected successfully\n")
	return nil
}

// UploadFileToS3 uploads a file to the configured S3 bucket.
// Passing size as -1 lets minio-go handle multipart upload automatically.
// Returns the public URL of the uploaded file.
func UploadFileToS3(reader io.Reader, filename string, contentType string) (string, error) {
	if S3Client == nil {
		return "", fmt.Errorf("S3 storage not initialized")
	}

	// Fall back to extension-based detection if content type is not provided
	if contentType == "" {
		contentType = detectContentType(filename)
	}

	_, err := S3Client.PutObject(context.TODO(), S3Cfg.Bucket, filename, reader, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return GetFileURL(filename), nil
}

// DeleteFileFromS3 removes an object from the configured S3 bucket by key.
func DeleteFileFromS3(key string) error {
	if S3Client == nil {
		return fmt.Errorf("S3 storage not initialized")
	}

	err := S3Client.RemoveObject(context.TODO(), S3Cfg.Bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// GetFileURL constructs the public URL for a stored object.
// Priority: CustomDomain > path-style endpoint > virtual-hosted style.
func GetFileURL(key string) string {
	if S3Cfg == nil {
		return ""
	}

	// Use custom domain if configured (e.g. CDN domain)
	if S3Cfg.CustomDomain != "" {
		return fmt.Sprintf("https://%s/%s", S3Cfg.CustomDomain, key)
	}

	if S3Cfg.ForcePath {
		// Path-style URL: https://endpoint/bucket/key
		endpoint := S3Cfg.Endpoint
		if !strings.HasPrefix(endpoint, "http") {
			endpoint = "https://" + endpoint
		}
		endpoint = strings.TrimSuffix(endpoint, "/")
		return fmt.Sprintf("%s/%s/%s", endpoint, S3Cfg.Bucket, key)
	}

	endpoint := S3Cfg.Endpoint
	if endpoint == "" {
		// Standard AWS virtual-hosted style URL
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", S3Cfg.Bucket, S3Cfg.Region, key)
	}

	// Virtual-hosted style for custom endpoints: https://bucket.endpoint/key
	endpoint = strings.Split(endpoint, ":")[0] // strip port if present
	return fmt.Sprintf("https://%s.%s/%s", S3Cfg.Bucket, endpoint, key)
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
