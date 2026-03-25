package handler

import (
	"vexgo/backend/middleware"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RegisterAPIRoutes registers all HTTP routes under /api.
// This keeps routing logic out of main.go and avoids import cycles.
func RegisterAPIRoutes(r *gin.Engine) {
	logrus.Debug("Registering API routes")
	api := r.Group("/api")
	api.Use(middleware.OptionalJWTAuth())
	{
		// -------------------- Public API (no JWT authentication required) --------------------
		logrus.Debug("Registering public API routes")
		api.GET("/posts", GetPosts)
		api.GET("/posts/:id", GetPost)

		api.GET("/verify-email", VerifyEmail)

		api.GET("/captcha", GenerateCaptcha)
		api.POST("/captcha/verify", VerifyCaptcha)

		api.GET("/categories", GetCategories)
		api.GET("/tags", GetTags)

		api.GET("/stats", GetStats)
		api.GET("/stats/popular-posts", GetPopularPosts)
		api.GET("/stats/latest-posts", GetLatestPosts)

		api.GET("/themes", GetThemes)
		api.GET("/theme/:id/preview", GetThemePreview)

		api.GET("/comments/post/:id", GetComments)
		api.GET("/likes/:postId", GetLikeStatus)
		api.GET("/posts/user/:id", GetUserPosts)

		// -------------------- Authentication API --------------------
		logrus.Debug("Registering authentication API routes")
		auth := api.Group("/auth")
		{
			auth.POST("/register", Register)
			auth.POST("/login", Login)
			auth.GET("/me", middleware.JWTAuth(), GetCurrentUser)
			auth.GET("/user", middleware.JWTAuth(), GetCurrentUser)
			auth.PUT("/profile", middleware.JWTAuth(), UpdateProfile)
			auth.PUT("/password", middleware.JWTAuth(), ChangePassword)
			auth.PUT("/email", middleware.JWTAuth(), UpdateEmail)
			auth.PUT("/settings", middleware.JWTAuth(), UpdateSettings)
			auth.POST("/request-password-reset", RequestPasswordReset)
			auth.POST("/reset-password", ResetPassword)
			auth.GET("/verification-status", middleware.JWTAuth(), GetVerificationStatus)
		}

		// -------------------- SSO --------------------
		sso := api.Group("/sso")
		{
			// Public: returns enabled providers, used by frontend to show/hide buttons
			// GET /api/sso/providers
			sso.GET("/providers", SSOProviders)

			// Step 1: open in popup → redirects to provider
			// GET /api/sso/:provider/login?method=sso_get_token|get_sso_id
			sso.GET("/:provider/login", SSOLoginRedirect)

			// Step 2: provider redirects back, popup sends postMessage → closes
			// GET /api/sso/:provider/callback?method=...&code=...&state=...
			sso.GET("/:provider/callback", func(c *gin.Context) {
				SSOCallback(c, DB())
			})
		}

		// -------------------- Business API requiring JWT authentication --------------------
		api.POST("/posts", middleware.JWTAuth(), CreatePost)
		api.GET("/posts/user/my-posts", middleware.JWTAuth(), GetMyPosts)
		api.GET("/posts/drafts", middleware.JWTAuth(), GetDraftPosts)
		api.PUT("/posts/:id", middleware.JWTAuth(), UpdatePost)
		api.DELETE("/posts/:id", middleware.JWTAuth(), DeletePost)

		api.POST("/categories", middleware.JWTAuth(), CreateCategory)
		api.POST("/tags", middleware.JWTAuth(), CreateTag)

		api.POST("/comments", middleware.JWTAuth(), CreateComment)
		api.DELETE("/comments/:id", middleware.JWTAuth(), DeleteComment)

		api.GET("/moderation/comments/pending", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetPendingComments)
		api.GET("/moderation/comments/approved", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetApprovedComments)
		api.GET("/moderation/comments/rejected", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetRejectedComments)
		api.PUT("/moderation/comments/approve/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), ApproveComment)
		api.PUT("/moderation/comments/reject/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), RejectComment)
		api.GET("/moderation/comments/config", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetCommentModerationConfig)
		api.PUT("/moderation/comments/config", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), UpdateCommentModerationConfig)

		api.POST("/likes/:postId", middleware.JWTAuth(), ToggleLike)

		api.POST("/upload/file", middleware.JWTAuth(), UploadFile)
		api.POST("/upload/files", middleware.JWTAuth(), UploadFiles)
		api.GET("/upload/my-files", middleware.JWTAuth(), GetMyFiles)
		api.DELETE("/upload/:id", middleware.JWTAuth(), DeleteFile)

		api.GET("/moderation/pending", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetPendingPosts)
		api.GET("/moderation/approved", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetApprovedPosts)
		api.GET("/moderation/rejected", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetRejectedPosts)
		api.PUT("/moderation/approve/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), ApprovePost)
		api.PUT("/moderation/reject/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), RejectPost)
		api.PUT("/moderation/resubmit/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), ResubmitPost)

		api.GET("/users", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetUserList)
		api.PUT("/users/:id/role", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), UpdateUserRole)
		api.DELETE("/users/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), DeleteUser)

		api.GET("/config/smtp", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetSMTPConfig)
		api.PUT("/config/smtp", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), UpdateSMTPConfig)
		api.POST("/config/smtp/test", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), TestSMTP)

		api.GET("/config/ai", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetAIConfig)
		api.PUT("/config/ai", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), UpdateAIConfig)
		api.POST("/config/ai/test", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), TestAI)
		api.GET("/config/ai/models", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), GetAIModels)

		api.GET("/config/general", GetGeneralSettings)
		api.PUT("/config/general", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), UpdateGeneralSettings)

		api.GET("/config/theme", GetThemeConfig)
		api.PUT("/config/theme", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), UpdateThemeConfig)

		// Theme upload endpoint
		api.POST("/themes/upload", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), UploadTheme)

		// Messages related
		api.GET("/messages", middleware.JWTAuth(), GetMessages)
		api.GET("/messages/unread-count", middleware.JWTAuth(), GetUnreadCount)
		api.PUT("/messages/:id/read", middleware.JWTAuth(), MarkAsRead)
		api.PUT("/messages/read-all", middleware.JWTAuth(), MarkAllAsRead)
		api.DELETE("/messages/:id", middleware.JWTAuth(), DeleteMessage)
	}
}
