package auth

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the auth domain routes on the /api group.
// Route paths and middleware chains are identical to the original registration
// in the legacy handler package.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.GET("/me", h.mw.JWTAuth(), h.GetCurrentUser)
		auth.GET("/user", h.mw.JWTAuth(), h.GetCurrentUser)
		auth.PUT("/profile", h.mw.JWTAuth(), h.UpdateProfile)
		auth.PUT("/password", h.mw.JWTAuth(), h.ChangePassword)
		auth.PUT("/email", h.mw.JWTAuth(), h.UpdateEmail)
		auth.PUT("/settings", h.mw.JWTAuth(), h.UpdateSettings)
		auth.POST("/request-password-reset", h.RequestPasswordReset)
		auth.POST("/resend-verification", h.ResendVerification)
		auth.POST("/reset-password", h.ResetPassword)
		auth.GET("/verification-status", h.mw.JWTAuth(), h.GetVerificationStatus)
	}

	api.GET("/verify-email", h.VerifyEmail)
}
