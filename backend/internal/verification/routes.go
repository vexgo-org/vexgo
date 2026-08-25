package verification

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the verification domain routes on the /api group.
// Route paths and middleware chains are identical to the original registration
// in the legacy handler package.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/captcha", h.GenerateCaptcha)
	api.POST("/captcha/verify", h.VerifyCaptcha)

	auth := api.Group("/auth")
	auth.GET("/verification-status", h.mw.JWTAuth(), h.GetVerificationStatus)
}
