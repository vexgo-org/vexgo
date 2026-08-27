package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// S3Config holds S3-compatible storage configuration
type S3Config struct {
	Enabled                  bool   `yaml:"enabled"`                      // Enable S3 storage
	Endpoint                 string `yaml:"endpoint"`                     // S3 endpoint URL (e.g., "https://s3.amazonaws.com" or MinIO endpoint)
	Region                   string `yaml:"region"`                       // AWS region (e.g., "us-east-1")
	Bucket                   string `yaml:"bucket"`                       // S3 bucket name
	AccessKey                string `yaml:"access_key"`                   // S3 access key ID
	SecretKey                string `yaml:"secret_key"`                   // S3 secret access key
	ForcePath                bool   `yaml:"force_path"`                   // Force path-style URLs (for MinIO, Wasabi, etc.)
	CustomDomain             string `yaml:"custom_domain"`                // Optional custom domain for S3 URLs (e.g., "cdn.example.com")
	DisableBucketInCustomURL bool   `yaml:"disable_bucket_in_custom_url"` // Disable including bucket in custom domain URLs (default: false, meaning include bucket by default)
}

// IsEnabled returns true if S3 storage is enabled
func (s *S3Config) IsEnabled() bool {
	return s.Enabled
}

// GetURL returns the public URL for an object in S3
func (s *S3Config) GetURL(key string) string {
	slog.Debug(
		"generating S3 object URL",
		"customDomain", s.CustomDomain,
		"disableBucketInCustomURL", s.DisableBucketInCustomURL,
		"bucket", s.Bucket,
		"key", key,
	)
	if s.CustomDomain != "" {
		domain := s.CustomDomain
		if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
			domain = "https://" + domain
		}
		slog.Debug("s3 geturl", "domain", domain)
		if !s.DisableBucketInCustomURL {
			url := fmt.Sprintf("%s/%s/%s", domain, s.Bucket, key)
			slog.Debug("s3 geturl: including bucket", "url", url)
			return url
		}
		url := fmt.Sprintf("%s/%s", domain, key)
		slog.Debug("s3 geturl: not including bucket", "url", url)
		return url
	}
	// Default S3 URL format
	if s.ForcePath {
		domain := s.Endpoint
		if after, ok := strings.CutPrefix(domain, "http://"); ok {
			domain = after
		} else if after, ok := strings.CutPrefix(domain, "https://"); ok {
			domain = after
		}
		return fmt.Sprintf("https://%s/%s/%s", domain, s.Bucket, key)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.Bucket, s.Region, key)
}

// LoadFromEnv loads S3 configuration from environment variables
func (s *S3Config) LoadFromEnv() {
	// S3_ENABLED: "true" or "false"
	if env := os.Getenv("S3_ENABLED"); env != "" {
		if b, err := strconv.ParseBool(env); err == nil {
			s.Enabled = b
		}
	}

	// S3_ENDPOINT: S3 endpoint URL
	if env := os.Getenv("S3_ENDPOINT"); env != "" {
		s.Endpoint = env
	}

	// S3_REGION: AWS region
	if env := os.Getenv("S3_REGION"); env != "" {
		s.Region = env
	}

	// S3_BUCKET: bucket name
	if env := os.Getenv("S3_BUCKET"); env != "" {
		s.Bucket = env
	}

	// S3_ACCESS_KEY: access key ID
	if env := os.Getenv("S3_ACCESS_KEY"); env != "" {
		s.AccessKey = env
	}

	// S3_SECRET_KEY: secret access key
	if env := os.Getenv("S3_SECRET_KEY"); env != "" {
		s.SecretKey = env
	}

	// S3_FORCE_PATH: "true" or "false" for path-style URLs
	if env := os.Getenv("S3_FORCE_PATH"); env != "" {
		if b, err := strconv.ParseBool(env); err == nil {
			s.ForcePath = b
		}
	}

	// S3_CUSTOM_DOMAIN: custom CDN domain
	if env := os.Getenv("S3_CUSTOM_DOMAIN"); env != "" {
		s.CustomDomain = env
	}

	// S3_DISABLE_BUCKET_IN_CUSTOM_URL: "true" or "false" for disabling bucket in custom URL
	if env := os.Getenv("S3_DISABLE_BUCKET_IN_CUSTOM_URL"); env != "" {
		if b, err := strconv.ParseBool(env); err == nil {
			s.DisableBucketInCustomURL = b
		}
	}
}

// MergeFromConfig merges configuration from another S3Config (only fills empty fields)
func (s *S3Config) MergeFromConfig(other *S3Config) {
	if other.Enabled {
		s.Enabled = other.Enabled
	}
	if other.Endpoint != "" {
		s.Endpoint = other.Endpoint
	}
	if other.Region != "" {
		s.Region = other.Region
	}
	if other.Bucket != "" {
		s.Bucket = other.Bucket
	}
	if other.AccessKey != "" {
		s.AccessKey = other.AccessKey
	}
	if other.SecretKey != "" {
		s.SecretKey = other.SecretKey
	}
	if other.ForcePath {
		s.ForcePath = other.ForcePath
	}
	if other.CustomDomain != "" {
		s.CustomDomain = other.CustomDomain
	}
	if !s.DisableBucketInCustomURL && other.DisableBucketInCustomURL {
		s.DisableBucketInCustomURL = other.DisableBucketInCustomURL
	}
}

// Validate checks if the configuration is valid
func (s *S3Config) Validate() error {
	if !s.Enabled {
		return nil // Not enabled, validation passes
	}
	if s.Endpoint == "" {
		return fmt.Errorf("s3 endpoint is required when s3 is enabled")
	}
	if s.Region == "" {
		return fmt.Errorf("s3 region is required when s3 is enabled")
	}
	if s.Bucket == "" {
		return fmt.Errorf("s3 bucket is required when s3 is enabled")
	}
	if s.AccessKey == "" {
		return fmt.Errorf("s3 access key is required when s3 is enabled")
	}
	if s.SecretKey == "" {
		return fmt.Errorf("s3 secret key is required when s3 is enabled")
	}
	return nil
}
