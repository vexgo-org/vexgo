package auth

import (
	"github.com/vexgo-org/vexgo/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the auth domain routes on the /api group.
// Route paths and middleware chains are identical to the original registration
// in the legacy handler package.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	{
		// Unauthenticated credential endpoints carry a per-IP rate limit so
		// online brute-force and mail-bombing cannot run unchecked even when
		// captcha is disabled.
		limited := middleware.NewRateLimiter(h.rateLimitPerMinute)

		if limited != nil {
			auth.POST("/register", limited, h.Register)
			auth.POST("/login", limited, h.Login)
			auth.POST("/request-password-reset", limited, h.RequestPasswordReset)
			auth.POST("/resend-verification", limited, h.ResendVerification)
			auth.POST("/reset-password", limited, h.ResetPassword)
		} else {
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
			auth.POST("/request-password-reset", h.RequestPasswordReset)
			auth.POST("/resend-verification", h.ResendVerification)
			auth.POST("/reset-password", h.ResetPassword)
		}

		auth.GET("/me", h.mw.JWTAuth(), h.GetCurrentUser)
		auth.GET("/user", h.mw.JWTAuth(), h.GetCurrentUser)
		auth.PUT("/profile", h.mw.JWTAuth(), h.UpdateProfile)
		auth.PUT("/password", h.mw.JWTAuth(), h.ChangePassword)
		auth.PUT("/email", h.mw.JWTAuth(), h.UpdateEmail)
		auth.PUT("/settings", h.mw.JWTAuth(), h.UpdateSettings)
		auth.GET("/verification-status", h.mw.JWTAuth(), h.GetVerificationStatus)
	}

	api.GET("/verify-email", h.VerifyEmail)
}
