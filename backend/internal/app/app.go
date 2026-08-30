// Package app is the composition root (bootstrap) for the application.
// It wires all dependencies together and provides the entry point.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/cache"
	"github.com/vexgo-org/vexgo/backend/internal/captcha"
	"github.com/vexgo-org/vexgo/backend/internal/comment"
	"github.com/vexgo-org/vexgo/backend/internal/config"
	"github.com/vexgo-org/vexgo/backend/internal/database"
	"github.com/vexgo-org/vexgo/backend/internal/home"
	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/notification"
	"github.com/vexgo-org/vexgo/backend/internal/post"
	"github.com/vexgo-org/vexgo/backend/internal/public"
	"github.com/vexgo-org/vexgo/backend/internal/router"
	"github.com/vexgo-org/vexgo/backend/internal/settings"
	"github.com/vexgo-org/vexgo/backend/internal/sso"
	"github.com/vexgo-org/vexgo/backend/internal/upload"
	"github.com/vexgo-org/vexgo/backend/internal/user"

	"github.com/gin-gonic/gin"
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

	if err := cfg.ComputeJWTSecret(); err != nil {
		return nil, fmt.Errorf("compute JWT secret: %w", err)
	}

	cipher, err := initCipher(cfg)
	if err != nil {
		return nil, fmt.Errorf("init cipher: %w", err)
	}

	storage, err := initStorage(cfg)
	if err != nil {
		return nil, fmt.Errorf("init storage: %w", err)
	}

	appCache, err := initCache(cfg)
	if err != nil {
		return nil, fmt.Errorf("init cache: %w", err)
	}

	// Distributed rate limiting only makes sense with a shared store; without
	// valkey the endpoint groups keep their per-process in-memory budget.
	var distributedRateLimit middleware.RateLimitStore
	if cfg.ValkeyEnabled {
		distributedRateLimit = middleware.NewFixedWindowRateLimitStore(appCache)
	}
	// The sso domain keeps its sweeping in-process state store without
	// valkey; a distributed store shares one OAuth state across instances.
	var ssoStateStore sso.StateStore
	if cfg.ValkeyEnabled {
		ssoStateStore = appCache
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

	// Encrypt still-plaintext secrets in place when a key is configured.
	// Idempotent: values already carrying the encrypted marker are skipped.
	if cipher != nil {
		migrated, err := database.MigrateSecretsAtRest(db, cipher)
		if err != nil {
			return nil, fmt.Errorf("migrate secrets at rest: %w", err)
		}
		if migrated > 0 {
			slog.Info("encrypted plaintext secrets at rest", "count", migrated)
		}
	}

	// gin.New (no default logger) + explicit recovery: request logging is done
	// once by middleware.RequestLogger below, avoiding double log lines.
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.SecurityHeaders())

	renderer := public.NewRenderer(db, fmt.Sprintf("http://%s", cfg.GetListenAddr()), cfg.DataDir)
	slog.Info("base url set for server-side rendering", "baseURL", renderer.BaseURL())

	configureProxies(r, cfg)

	// Shared service instances: construct each once and reuse it across the
	// domains that depend on it.
	notificationSvc := notification.NewService(notification.Deps{DB: db, JWTSecret: cfg.JWTSecret})
	mailerSvc := mailer.NewService(mailer.Deps{DB: db, Cipher: cipher})
	captchaSvc := captcha.NewService(captcha.Deps{DB: db, JWTSecret: cfg.JWTSecret})

	router.RegisterAPIRoutes(r, router.Deps{
		DB:        db,
		JWTSecret: cfg.JWTSecret,
		Notification: notification.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
		},
		Comment: comment.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Notifier:  notificationSvc,
			Cipher:    cipher,
		},
		Post: post.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Notifier:  notificationSvc,
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
			Notifier:  notificationSvc,
			Files:     storage,
		},
		Captcha: captcha.Deps{
			DB:                 db,
			JWTSecret:          cfg.JWTSecret,
			RateLimitPerMinute: cfg.CaptchaRateLimitPerMinute,
			RateLimit:          distributedRateLimit,
		},
		Auth: auth.Deps{
			DB:                 db,
			JWTSecret:          cfg.JWTSecret,
			Files:              storage,
			Mailer:             mailerSvc,
			Captcha:            captchaSvc,
			BaseURL:            cfg.BaseURL,
			BehindReverseProxy: cfg.BehindReverseProxy,
			RateLimitPerMinute: cfg.AuthRateLimitPerMinute,
			RateLimit:          distributedRateLimit,
		},
		SSO: sso.Deps{
			DB:         db,
			SSO:        &cfg.SSO,
			JWTSecret:  cfg.JWTSecret,
			Mailer:     mailerSvc,
			BaseURL:    cfg.BaseURL,
			StateStore: ssoStateStore,
		},
		Home: home.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
		},
		Settings: settings.Deps{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Themes:    renderer,
			Mailer:    mailerSvc,
			Cipher:    cipher,
		},
	})

	renderer.RegisterStaticRoutes(r, cfg.S3Enabled)

	return &App{cfg: cfg, db: db, engine: r}, nil
}

// Run starts the HTTP server. Explicit timeouts guard against slow-loris and
// slow-body connection exhaustion, which the net/http defaults (no timeouts)
// do not protect against.
func (a *App) Run() error {
	srv := &http.Server{
		Addr:              a.cfg.GetListenAddr(),
		Handler:           a.engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	slog.Info("starting server", "address", a.cfg.GetListenAddr())
	return srv.ListenAndServe()
}

// initCache returns the cache backend: an in-process memory cache by default,
// or a Valkey (Redis-compatible) connection when valkey is enabled. Enabling
// requires a reachable server: the connection is verified with a PING so
// misconfiguration fails at startup instead of on first use.
func initCache(cfg *config.Config) (cache.Cache, error) {
	if !cfg.ValkeyEnabled {
		slog.Info("using in-process cache")
		return cache.NewMemory(), nil
	}
	if cfg.ValkeyURL == "" {
		return nil, fmt.Errorf("valkey_enabled requires valkey_url to be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	vc, err := cache.NewValkey(ctx, cfg.ValkeyURL)
	if err != nil {
		return nil, err
	}
	slog.Info("using valkey cache")
	return vc, nil
}

// The cache backends satisfy the consumer-declared seams structurally, so the
// consumers never import internal/cache.
var (
	_ middleware.CounterStore = cache.Cache(nil)
	_ sso.StateStore          = cache.Cache(nil)
)

// initStorage returns the file storage backend: local disk by default, or an
// S3-compatible storage when S3 is enabled in the config.
func initStorage(cfg *config.Config) (upload.Storage, error) {
	var storage upload.Storage = upload.NewLocalStorage(cfg.DataDir)
	if !cfg.S3Enabled {
		slog.Info("using local file storage")
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
	slog.Info(
		"s3 config loaded",
		"enabled", s3Cfg.Enabled,
		"endpoint", s3Cfg.Endpoint,
		"region", s3Cfg.Region,
		"bucket", s3Cfg.Bucket,
	)

	s3Storage, err := upload.NewS3Storage(s3Cfg)
	if err != nil {
		return nil, fmt.Errorf("init S3 storage: %w", err)
	}
	if s3Storage != nil {
		storage = s3Storage
	}
	slog.Info("s3 storage initialized")
	return storage, nil
}

// configureProxies sets gin's trusted proxies: none when not behind a reverse
// proxy, the configured list when behind one, and common private networks as
// the fallback when the list is empty.
func configureProxies(r *gin.Engine, cfg *config.Config) {
	if !cfg.BehindReverseProxy {
		if err := r.SetTrustedProxies(nil); err != nil {
			slog.Error("failed to disable trusted proxies", "err", err)
			os.Exit(1)
		}
		return
	}

	if len(cfg.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
			slog.Error("invalid trusted proxies configuration", "err", err)
			os.Exit(1)
		}
		slog.Info("trusted proxies configured", "proxies", cfg.TrustedProxies)
	} else {
		defaultProxies := []string{"127.0.0.1", "::1", "192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"}
		if err := r.SetTrustedProxies(defaultProxies); err != nil {
			slog.Error("invalid default trusted proxies configuration", "err", err)
			os.Exit(1)
		}
		slog.Info("trusted proxies set to common private networks")
	}
}

// setupLogging sets the global slog level and formatter from the config,
// falling back to info when the level string is invalid.
func setupLogging(levelStr string) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		slog.Warn("invalid log level, fallback to info level", "level", levelStr, "err", err)
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	slog.SetDefault(slog.New(handler))
}
