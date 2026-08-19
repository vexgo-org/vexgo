// Package app is the composition root (bootstrap) for the application.
// It wires all dependencies together and provides the entry point.
package app

import (
	"fmt"
	"strings"

	"vexgo/backend/internal/auth"
	"vexgo/backend/internal/comment"
	"vexgo/backend/internal/config"
	"vexgo/backend/internal/database"
	"vexgo/backend/internal/home"
	"vexgo/backend/internal/mailer"
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
	"gorm.io/gorm"
)

// App is the fully-wired application.
type App struct {
	cfg    *config.Config
	db     *gorm.DB
	engine *gin.Engine
}

// New creates and wires the application from the given config.
func New(cfg *config.Config) (*App, error) {
	setupLogging(cfg.LogLevel)

	cfg.ComputeJWTSecret()
	cfg.LoadSSOFromEnv()
	cfg.LoadSSOFromConfig()

	storage, err := initStorage(cfg)
	if err != nil {
		return nil, fmt.Errorf("init storage: %w", err)
	}

	db, err := database.Open(cfg, cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	if err := database.Seed(db); err != nil {
		return nil, fmt.Errorf("seed database: %w", err)
	}

	r := gin.Default()

	renderer := public.NewRenderer(db, fmt.Sprintf("http://%s", cfg.GetListenAddr()), cfg.DataDir)
	logrus.WithField("baseURL", renderer.BaseURL()).Info("Base URL set for server-side rendering")

	configureProxies(r, cfg)

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
			Mailer:    mailer.NewMailer(db),
		},
		Auth: auth.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Files:     storage,
			Mailer:    mailer.NewMailer(db),
			Captcha:   verification.NewService(verification.Deps{DB: db, JWTSecret: cfg.JWTSecret, Mailer: mailer.NewMailer(db)}),
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

	renderer.RegisterStaticRoutes(r, cfg.S3Enabled)

	return &App{cfg: cfg, db: db, engine: r}, nil
}

// Run starts the HTTP server.
func (a *App) Run() error {
	logrus.WithField("address", a.cfg.GetListenAddr()).Info("Starting server")
	return a.engine.Run(a.cfg.GetListenAddr())
}

func initStorage(cfg *config.Config) (upload.Storage, error) {
	var storage upload.Storage = upload.NewLocalStorage(cfg.DataDir)
	if !cfg.S3Enabled {
		logrus.Info("Using local file storage")
		return storage, nil
	}

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
		"enabled":  s3Cfg.Enabled,
		"endpoint": s3Cfg.Endpoint,
		"region":   s3Cfg.Region,
		"bucket":   s3Cfg.Bucket,
	}).Info("S3 Config Loaded")

	s3Storage, err := upload.NewS3Storage(s3Cfg)
	if err != nil {
		return nil, fmt.Errorf("init S3 storage: %w", err)
	}
	if s3Storage != nil {
		storage = s3Storage
	}
	logrus.Info("S3 storage initialized")
	return storage, nil
}

func configureProxies(r *gin.Engine, cfg *config.Config) {
	if !cfg.BehindReverseProxy {
		if err := r.SetTrustedProxies(nil); err != nil {
			logrus.WithError(err).Fatal("Failed to disable trusted proxies")
		}
		return
	}

	if len(cfg.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
			logrus.WithError(err).Fatal("Invalid trusted proxies configuration")
		}
		logrus.WithField("proxies", cfg.TrustedProxies).Info("Trusted proxies configured")
	} else {
		defaultProxies := []string{"127.0.0.1", "::1", "192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"}
		if err := r.SetTrustedProxies(defaultProxies); err != nil {
			logrus.WithError(err).Fatal("Invalid default trusted proxies configuration")
		}
		logrus.Info("Trusted proxies set to common private networks")
	}
}

func setupLogging(levelStr string) {
	level, err := logrus.ParseLevel(strings.ToLower(levelStr))
	if err != nil {
		logrus.Warnf("Invalid log level '%s', defaulting to 'info'", levelStr)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
}
