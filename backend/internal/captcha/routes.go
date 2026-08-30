package captcha

import (
	"github.com/vexgo-org/vexgo/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the captcha domain routes on the /api group.
// Both endpoints are unauthenticated and comparatively expensive, so they sit
// behind the per-client-IP rate limit when one is configured.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	routes := api
	if limiter := middleware.NewRateLimiter(h.rateLimitPerMinute); limiter != nil {
		routes = api.Group("", limiter)
	}
	routes.GET("/captcha", h.GenerateCaptcha)
	routes.POST("/captcha/verify", h.VerifyCaptcha)
}
