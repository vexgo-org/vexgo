package auth

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Deps holds the dependencies required by the auth domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Files     FileRemover
	Mailer    *mailer.Service
	Captcha   CaptchaChecker

	// BaseURL is the public site origin (e.g. https://blog.example.com) used
	// to build absolute links inside emails, sourced from BASE_URL / cfg.BaseURL.
	// When set it overrides any request-supplied Host or forwarding header.
	BaseURL string
	// BehindReverseProxy enables honoring X-Forwarded-Proto when BaseURL is
	// not configured. Mirrors cfg.BehindReverseProxy / behind_reverse_proxy.
	BehindReverseProxy bool
	// RateLimitPerMinute caps unauthenticated auth requests (register, login,
	// password reset, verification resend) per client IP per minute; 0 or less
	// disables the limiter.
	RateLimitPerMinute int
	// RateLimit stores the per-IP request budget. nil keeps it in-process; a
	// distributed store shares one budget across instances.
	RateLimit middleware.RateLimitStore
}

// FileRemover is an alias for model.FileRemover kept for backward compatibility.
type FileRemover = model.FileRemover

// CaptchaChecker is the seam for checking whether captcha verification is
// enabled; implemented by the verification domain and injected so it can be
// faked in tests.
type CaptchaChecker interface {
	IsCaptchaEnabled(ctx context.Context) (bool, error)
}

// Service contains the business logic of the auth domain.
type Service struct {
	repo      Repository
	jwtSecret []byte
	files     FileRemover
	mailer    *mailer.Service
	captcha   CaptchaChecker
}

// NewService creates an auth service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{
		repo:      NewRepository(deps.DB),
		jwtSecret: deps.JWTSecret,
		files:     deps.Files,
		mailer:    deps.Mailer,
		captcha:   deps.Captcha,
	}
}
