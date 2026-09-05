package captcha

import (
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
)

// Handler exposes the captcha domain over HTTP.
type Handler struct {
	svc                *Service
	rateLimitPerMinute int
	rateLimit          middleware.RateLimitStore
}

// NewHandler creates a captcha HTTP handler with the given
// dependencies. A positive deps.RateLimitPerMinute installs a
// per-client-IP rate limit on the unauthenticated captcha endpoints.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), rateLimitPerMinute: deps.RateLimitPerMinute, rateLimit: deps.RateLimit}
}
