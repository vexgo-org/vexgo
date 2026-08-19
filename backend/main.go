package main

import (
	"fmt"
	"strings"

	"vexgo/backend/internal/auth"
	"vexgo/backend/internal/comment"
	"vexgo/backend/internal/config"
	"vexgo/backend/internal/database"
	"vexgo/backend/internal/home"
	"vexgo/backend/internal/message"
	"vexgo/backend/internal/post"
	"vexgo/backend/internal/public"
	"vexgo/backend/internal/router"
	"vexgo/backend/internal/settings"
	"vexgo/backend/internal/sso"
	"vexgo/backend/internal/upload"
	"vexgo/backend/internal/user"
	"vexgo/backend/internal/verification"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// 1. Parse command line arguments (also loads .env)
	cfg := config.ParseFlags()

	// 2. Setup logging
	setupLogging(cfg.LogLevel)

	// 3. Compute derived config (JWT secret, SSO from env/file)
	cfg.ComputeJWTSecret()
	cfg.LoadSSOFromEnv()
	cfg.LoadSSOFromConfig()

	// 4. Initialize file storage: local disk by default, S3-compatible when enabled
	var storage upload.Storage = upload.NewLocalStorage(cfg.DataDir)
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
		if s3Storage, err := upload.NewS3Storage(s3Cfg); err != nil {
			logrus.WithError(err).Fatal("Failed to initialize S3 storage")
		} else if s3Storage != nil {
			storage = s3Storage
		}
		logrus.Info("S3 storage initialized")
	} else {
		logrus.Info("Using local file storage")
	}

	// 5. Initialize database connection
	db := database.Open(cfg, cfg.DataDir)
	database.AutoMigrate(db)
	database.Seed(db)

	// 6. Create Gin engine instance
	r := gin.Default()

	// 6.1 Create the SSR renderer
	renderer := public.NewRenderer(db, fmt.Sprintf("http://%s", cfg.GetListenAddr()), cfg.DataDir)
	logrus.WithField("baseURL", renderer.BaseURL()).Info("Base URL set for server-side rendering")

	// Configure trusted proxies
	if cfg.BehindReverseProxy {
		if len(cfg.TrustedProxies) > 0 {
			if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
				logrus.WithError(err).Fatal("Invalid trusted proxies configuration")
			}
			logrus.WithField("proxies", cfg.TrustedProxies).Info("Trusted proxies configured")
		} else {
			defaultProxies := []string{
				"127.0.0.1",
				"::1",
				"192.168.0.0/16",
				"10.0.0.0/8",
				"172.16.0.0/12",
			}
			if err := r.SetTrustedProxies(defaultProxies); err != nil {
				logrus.WithError(err).Fatal("Invalid default trusted proxies configuration")
			}
			logrus.WithField("proxies", defaultProxies).Info("Trusted proxies set to common private networks (behind reverse proxy)")
		}
	} else {
		if err := r.SetTrustedProxies(nil); err != nil {
			logrus.WithError(err).Fatal("Failed to disable trusted proxies")
		}
		logrus.Info("No trusted proxies configured (not behind reverse proxy)")
	}

	// ===================== Core API routing group =====================
	router.RegisterAPIRoutes(r, router.Deps{
		DB:        db,
		JWTSecret: cfg.JWTSecret,
		Message: message.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
		},
		Comment: comment.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Notifier:  message.NewService(message.Deps{DB: db, JWTSecret: cfg.JWTSecret}),
		},
		Post: post.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Notifier:  message.NewService(message.Deps{DB: db, JWTSecret: cfg.JWTSecret}),
			Files:     storage,
		},
		Upload: upload.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Storage:   storage,
		},
		User: user.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Notifier:  message.NewService(message.Deps{DB: db, JWTSecret: cfg.JWTSecret}),
			Files:     storage,
		},
		Verification: verification.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
		},
		Auth: auth.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Files:     storage,
		},
		SSO: sso.Deps{
			DB:        db,
			SSO:       &cfg.SSO,
			JWTSecret: cfg.JWTSecret,
		},
		Home: home.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
		},
		Settings: settings.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Themes:    renderer,
		},
	})

	// ===================== Static file hosting =====================
	renderer.RegisterStaticRoutes(r, cfg.S3Enabled)

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
