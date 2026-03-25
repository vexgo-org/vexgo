package main

import (
	"strings"
	"vexgo/backend/cmd"
	"vexgo/backend/config"
	"vexgo/backend/handler"
	"vexgo/backend/middleware"
	"vexgo/backend/public"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// 1. Parse command line arguments
	cfg := cmd.ParseFlags()

	// 2. Setup logging
	setupLogging(cfg.LogLevel)

	// 3. Initialize configuration (load JWT secret, etc., support config files and environment variables)
	config.Init(cfg.JWTSecret)

	// 3.1 Load SSO configuration from config file (overrides environment variables)
	config.LoadFromConfig(cfg)

	// Set data directory (for file uploads, only used if S3 is not enabled)
	handler.DataDir = cfg.DataDir

	// 4. Initialize S3 storage if enabled
	if cfg.S3Enabled {
		s3Cfg := &config.S3Config{
			Enabled:                  cfg.S3Enabled,
			Endpoint:                 cfg.S3Endpoint,
			Region:                   cfg.S3Region,
			Bucket:                   cfg.S3Bucket,
			AccessKey:                cfg.S3AccessKey,
			SecretKey:                cfg.S3SecretKey,
			ForcePath:                cfg.S3ForcePath,
			CustomDomain:             cfg.S3CustomDomain,
			DisableBucketInCustomURL: cfg.S3DisableBucketInCustomURL,
		}
		logrus.WithFields(logrus.Fields{
			"enabled":                  s3Cfg.Enabled,
			"endpoint":                 s3Cfg.Endpoint,
			"region":                   s3Cfg.Region,
			"bucket":                   s3Cfg.Bucket,
			"customDomain":             s3Cfg.CustomDomain,
			"disableBucketInCustomURL": s3Cfg.DisableBucketInCustomURL,
		}).Info("S3 Config Loaded")
		if err := handler.InitS3(s3Cfg); err != nil {
			logrus.WithError(err).Fatal("Failed to initialize S3 storage")
		}
		logrus.Info("S3 storage initialized")
	} else {
		logrus.Info("Using local file storage")
	}

	// 5. Initialize database connection (ensure database driver and connection string are configured correctly)
	handler.InitDB(cfg, cfg.DataDir)
	// Set database connection to authentication middleware
	middleware.SetDB(handler.DB())
	// Set database connection to handler package
	handler.SetDB(handler.DB())

	// 6. Create Gin engine instance (includes Logger and Recovery middleware by default)
	r := gin.Default()

	// Configure trusted proxies based on environment/configuration
	// If BEHIND_REVERSE_PROXY=true, use TRUSTED_PROXIES list or common defaults
	// If BEHIND_REVERSE_PROXY=false, disable proxy trust (no warning)
	if cfg.BehindReverseProxy {
		if len(cfg.TrustedProxies) > 0 {
			// Use explicitly configured trusted proxies
			r.SetTrustedProxies(cfg.TrustedProxies)
			logrus.WithField("proxies", cfg.TrustedProxies).Info("Trusted proxies configured")
		} else {
			// Use common defaults: trust all private IP ranges and localhost
			// This is a reasonable default for self-hosted behind reverse proxy
			defaultProxies := []string{
				"127.0.0.1",
				"::1",
				"192.168.0.0/16",
				"10.0.0.0/8",
				"172.16.0.0/12",
			}
			r.SetTrustedProxies(defaultProxies)
			logrus.WithField("proxies", defaultProxies).Info("Trusted proxies set to common private networks (behind reverse proxy)")
		}
	} else {
		// Not behind a reverse proxy, disable trust
		r.SetTrustedProxies(nil)
		logrus.Info("No trusted proxies configured (not behind reverse proxy)")
	}

	// ===================== Core API routing group (all endpoints under /api) =====================
	// All API routing definitions have been moved to handler.RegisterAPIRoutes to avoid cluttering main.go.
	handler.RegisterAPIRoutes(r)

	// ===================== Static file hosting =====================
	// Register all static routes (assets, uploads, SPA fallback) in the public package
	public.RegisterStaticRoutes(r, cfg.DataDir, cfg.S3Enabled)

	// 7. Start the server
	logrus.WithField("address", cfg.GetListenAddr()).Info("Starting server")
	if err := r.Run(cfg.GetListenAddr()); err != nil {
		logrus.WithError(err).Fatal("Failed to start server")
	}
}

// setupLogging configures the logging level based on the provided string
func setupLogging(levelStr string) {
	level, err := logrus.ParseLevel(strings.ToLower(levelStr))
	if err != nil {
		logrus.Warnf("Invalid log level '%s', defaulting to 'info'", levelStr)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.Infof("Log level set to: %s", level)
}
