package handler

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"vexgo/backend/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	S3Client     *s3.Client
	S3Cfg        *config.S3Config
	S3Uploader   *manager.Uploader
	UseS3Storage bool
)

// InitS3 initializes S3 client if S3 storage is enabled
func InitS3(cfg *config.S3Config) error {
	if !cfg.IsEnabled() {
		UseS3Storage = false
		return nil
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("S3 configuration error: %w", err)
	}

	// Debug logging for configuration
	fmt.Printf("S3 Debug: Initializing with Endpoint=%s, Region=%s, Bucket=%s, ForcePath=%v, CustomDomain=%s\n", cfg.Endpoint, cfg.Region, cfg.Bucket, cfg.ForcePath, cfg.CustomDomain)

	// Create credential provider
	credProvider := credentials.NewStaticCredentialsProvider(
		cfg.AccessKey,
		cfg.SecretKey,
		"", // session token, not needed
	)

	// Build AWS config
	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credProvider,
	}

	// Set custom endpoint if provided
	if cfg.Endpoint != "" {
		endpoint := cfg.Endpoint
		if !strings.HasPrefix(endpoint, "http") {
			endpoint = "https://" + endpoint
		}
		// Trim trailing slash
		endpoint = strings.TrimSuffix(endpoint, "/")
		awsCfg.BaseEndpoint = aws.String(endpoint)
		fmt.Printf("S3 Debug: Using custom endpoint: %s\n", endpoint)
	} else {
		fmt.Printf("S3 Debug: Using standard AWS endpoint\n")
	}

	// Create S3 client with proper configuration for S3-compatible services
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// Force path-style URLs for S3-compatible services (MinIO, Wasabi, Garage, etc.)
		if cfg.ForcePath {
			o.UsePathStyle = true
			fmt.Printf("S3 Debug: Forcing path-style URLs\n")
		} else {
			fmt.Printf("S3 Debug: Using virtual-hosted style URLs\n")
		}
	})

	// Create uploader with reasonable defaults
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = 10 * 1024 * 1024 // 10MB per part
		u.Concurrency = 5             // 5 concurrent uploads
	})

	S3Client = client
	S3Uploader = uploader
	S3Cfg = cfg
	UseS3Storage = true

	// Test connection by listing buckets (optional)
	// This can help diagnose connection issues early
	fmt.Printf("S3 Debug: Testing connection by listing buckets...\n")
	_, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		fmt.Printf("S3 Debug: Connection test failed: %v\n", err)
		return fmt.Errorf("failed to connect to S3: %w", err)
	}
	fmt.Printf("S3 Debug: Connection test successful\n")

	return nil
}

// UploadFile uploads a file to S3 and returns the public URL
func UploadFileToS3(reader io.Reader, filename string, contentType string) (string, error) {
	if S3Client == nil || S3Uploader == nil {
		return "", fmt.Errorf("S3 storage not initialized")
	}

	// Determine content type if not provided
	if contentType == "" {
		contentType = detectContentType(filename)
	}

	// Upload to S3
	result, err := S3Uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(S3Cfg.Bucket),
		Key:         aws.String(filename),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Generate public URL
	url := GetFileURL(filename)
	_ = result // result contains Location, ETag, etc.

	return url, nil
}

// DeleteFile deletes a file from S3
func DeleteFileFromS3(key string) error {
	if S3Client == nil {
		return fmt.Errorf("S3 storage not initialized")
	}

	_, err := S3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(S3Cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// GetFileURL returns the public URL for a file stored in S3
func GetFileURL(key string) string {
	if S3Cfg == nil {
		return ""
	}

	if S3Cfg.CustomDomain != "" {
		return fmt.Sprintf("https://%s/%s", S3Cfg.CustomDomain, key)
	}

	// Default S3 URL format
	if S3Cfg.ForcePath {
		// Path-style: https://endpoint/bucket/key
		endpoint := S3Cfg.Endpoint
		if !strings.HasPrefix(endpoint, "http") {
			endpoint = "https://" + endpoint
		}
		// Remove trailing slash if present
		endpoint = strings.TrimSuffix(endpoint, "/")
		return fmt.Sprintf("%s/%s/%s", endpoint, S3Cfg.Bucket, key)
	}

	// Virtual-hosted style: https://bucket.s3.region.amazonaws.com/key
	// or for custom endpoint: https://bucket.endpoint.com/key
	endpoint := S3Cfg.Endpoint
	if endpoint == "" {
		// Standard AWS endpoint
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", S3Cfg.Bucket, S3Cfg.Region, key)
	}
	// Custom endpoint (e.g., MinIO, Wasabi) - remove port if present
	endpoint = strings.Split(endpoint, ":")[0]
	return fmt.Sprintf("https://%s.%s/%s", S3Cfg.Bucket, endpoint, key)
}

// detectContentType detects MIME type based on file extension
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
