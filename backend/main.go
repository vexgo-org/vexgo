package main

import (
	"path/filepath"
	"strings"
	"vexgo/backend/cmd"
	"vexgo/backend/config"
	"vexgo/backend/handler"
	"vexgo/backend/middleware"
	"vexgo/backend/public"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Parse command line arguments
	cfg := cmd.ParseFlags()

	// 2. Initialize configuration (load JWT secret, etc., support config files and environment variables)
	config.Init(cfg.JWTSecret)

	// Set data directory (for file uploads)
	handler.DataDir = cfg.DataDir

	// 3. Initialize database connection (ensure database driver and connection string are configured correctly)
	handler.InitDB(cfg, cfg.DataDir)
	// Set database connection to authentication middleware
	middleware.SetDB(handler.DB())

	// 4. Create Gin engine instance (includes Logger and Recovery middleware by default)
	r := gin.Default()

	// ===================== Core API routing group (all endpoints under /api) =====================
	// Match frontend Axios baseURL: /api, ensure all frontend API requests are handled
	api := r.Group("/api")
	// Optional JWT middleware: public endpoints can identify logged-in users when Authorization header is present
	api.Use(middleware.OptionalJWTAuth())
	{
		// -------------------- Public API (no JWT authentication required) --------------------
		// Public endpoints related to posts
		api.GET("/posts", handler.GetPosts)    // GET /api/posts (get posts list)
		api.GET("/posts/:id", handler.GetPost) // GET /api/posts/:id (get single post)

		// Email verification public endpoints
		api.GET("/verify-email", handler.VerifyEmail) // GET /api/verify-email (verify email)

		// Sliding puzzle captcha public endpoints
		api.GET("/captcha", handler.GenerateCaptcha)       // GET /api/captcha (generate sliding puzzle captcha)
		api.POST("/captcha/verify", handler.VerifyCaptcha) // POST /api/captcha/verify (verify sliding puzzle)

		// Category/Tag public endpoints
		api.GET("/categories", handler.GetCategories) // GET /api/categories (get categories list)
		api.GET("/tags", handler.GetTags)             // GET /api/tags (get tags list)

		// Statistics related public endpoints
		api.GET("/stats", handler.GetStats)                      // GET /api/stats (get statistics)
		api.GET("/stats/popular-posts", handler.GetPopularPosts) // GET /api/stats/popular-posts
		api.GET("/stats/latest-posts", handler.GetLatestPosts)   // GET /api/stats/latest-posts

		// Comment/Like public endpoints
		api.GET("/comments/post/:id", handler.GetComments) // GET /api/comments/post/:id (get post comments)
		api.GET("/likes/:postId", handler.GetLikeStatus)   // GET /api/likes/:postId (get like status)
		// Get user posts list
		api.GET("/posts/user/:id", handler.GetUserPosts) // GET /api/posts/user/:id (get specified user's posts)

		// -------------------- Authentication related API (/api/auth subgroup) --------------------
		// Match all frontend authApi endpoints: /api/auth/xxx
		auth := api.Group("/auth")
		{
			auth.POST("/register", handler.Register)                                                 // POST /api/auth/register (register)
			auth.POST("/login", handler.Login)                                                       // POST /api/auth/login (login)
			auth.GET("/me", middleware.JWTAuth(), handler.GetCurrentUser)                            // GET /api/auth/me (get current user, requires authentication)
			auth.GET("/user", middleware.JWTAuth(), handler.GetCurrentUser)                          // backward compatibility: /api/auth/user
			auth.PUT("/profile", middleware.JWTAuth(), handler.UpdateProfile)                        // PUT /api/auth/profile (update profile)
			auth.PUT("/password", middleware.JWTAuth(), handler.ChangePassword)                      // PUT /api/auth/password (change password)
			auth.PUT("/email", middleware.JWTAuth(), handler.UpdateEmail)                            // PUT /api/auth/email (update email)
			auth.PUT("/settings", middleware.JWTAuth(), handler.UpdateSettings)                      // PUT /api/auth/settings (update user settings)
			auth.POST("/request-password-reset", handler.RequestPasswordReset)                       // POST /api/auth/request-password-reset (request password reset)
			auth.POST("/reset-password", handler.ResetPassword)                                      // POST /api/auth/reset-password (reset password)
			auth.GET("/verification-status", middleware.JWTAuth(), handler.GetVerificationStatus)    // GET /api/auth/verification-status (get verification status)
			auth.POST("/resend-verification", middleware.JWTAuth(), handler.ResendVerificationEmail) // POST /api/auth/resend-verification (resend verification email)
		}

		// -------------------- Business API requiring JWT authentication --------------------
		// Post operations (requires login)
		api.POST("/posts", middleware.JWTAuth(), handler.CreatePost)              // POST /api/posts (create post)
		api.GET("/posts/user/my-posts", middleware.JWTAuth(), handler.GetMyPosts) // GET /api/posts/user/my-posts (my posts)
		api.GET("/posts/drafts", middleware.JWTAuth(), handler.GetDraftPosts)     // GET /api/posts/drafts (draft posts)
		api.PUT("/posts/:id", middleware.JWTAuth(), handler.UpdatePost)           // PUT /api/posts/:id (update post)
		api.DELETE("/posts/:id", middleware.JWTAuth(), handler.DeletePost)        // DELETE /api/posts/:id (delete post)

		// Category/Tag operations (requires login)
		api.POST("/categories", middleware.JWTAuth(), handler.CreateCategory) // POST /api/categories (create category)
		api.POST("/tags", middleware.JWTAuth(), handler.CreateTag)            // POST /api/tags (create tag)

		// Comment operations (requires login)
		api.POST("/comments", middleware.JWTAuth(), handler.CreateComment)       // POST /api/comments (create comment)
		api.DELETE("/comments/:id", middleware.JWTAuth(), handler.DeleteComment) // DELETE /api/comments/:id (delete comment)

		// Comment moderation related API (requires admin permission)
		api.GET("/moderation/comments/pending", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetPendingComments)           // GET /api/moderation/comments/pending (get pending comments)
		api.GET("/moderation/comments/approved", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetApprovedComments)         // GET /api/moderation/comments/approved (get approved comments)
		api.GET("/moderation/comments/rejected", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetRejectedComments)         // GET /api/moderation/comments/rejected (get rejected comments)
		api.PUT("/moderation/comments/approve/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.ApproveComment)           // PUT /api/moderation/comments/approve/:id (approve comment)
		api.PUT("/moderation/comments/reject/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.RejectComment)             // PUT /api/moderation/comments/reject/:id (reject comment)
		api.GET("/moderation/comments/config", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetCommentModerationConfig)    // GET /api/moderation/comments/config (get comment moderation config)
		api.PUT("/moderation/comments/config", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.UpdateCommentModerationConfig) // PUT /api/moderation/comments/config (update comment moderation config)

		// Like operations (requires login)
		api.POST("/likes/:postId", middleware.JWTAuth(), handler.ToggleLike) // POST /api/likes/:postId (toggle like)

		// File upload operations (requires login)
		api.POST("/upload/file", middleware.JWTAuth(), handler.UploadFile)    // POST /api/upload/file (single file upload)
		api.POST("/upload/files", middleware.JWTAuth(), handler.UploadFiles)  // POST /api/upload/files (multiple files upload)
		api.GET("/upload/my-files", middleware.JWTAuth(), handler.GetMyFiles) // GET /api/upload/my-files (my files)
		api.DELETE("/upload/:id", middleware.JWTAuth(), handler.DeleteFile)   // DELETE /api/upload/:id (delete file)

		// Post moderation related API (requires admin permission)
		api.GET("/moderation/pending", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetPendingPosts)   // GET /api/moderation/pending (get pending posts)
		api.GET("/moderation/approved", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetApprovedPosts) // GET /api/moderation/approved (get approved posts)
		api.GET("/moderation/rejected", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetRejectedPosts) // GET /api/moderation/rejected (get rejected posts)
		api.PUT("/moderation/approve/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.ApprovePost)   // PUT /api/moderation/approve/:id (approve post)
		api.PUT("/moderation/reject/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.RejectPost)     // PUT /api/moderation/reject/:id (reject post)
		api.PUT("/moderation/resubmit/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.ResubmitPost) // PUT /api/moderation/resubmit/:id (resubmit post for moderation)

		// User management related API (requires admin permission)
		api.GET("/users", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetUserList)             // GET /api/users (get user list)
		api.PUT("/users/:id/role", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.UpdateUserRole) // PUT /api/users/:id/role (update user role)

		// SMTP configuration related API (requires admin permission)
		api.GET("/config/smtp", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetSMTPConfig)    // GET /api/config/smtp (get SMTP config)
		api.PUT("/config/smtp", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.UpdateSMTPConfig) // PUT /api/config/smtp (update SMTP config)
		api.POST("/config/smtp/test", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.TestSMTP)   // POST /api/config/smtp/test (test SMTP config)

		// AI configuration related API (requires admin permission)
		api.GET("/config/ai", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetAIConfig)        // GET /api/config/ai (get AI config)
		api.PUT("/config/ai", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.UpdateAIConfig)     // PUT /api/config/ai (update AI config)
		api.POST("/config/ai/test", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.TestAI)       // POST /api/config/ai/test (test AI config)
		api.GET("/config/ai/models", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetAIModels) // GET /api/config/ai/models (get model list)

		// General settings related API (GET public, PUT requires admin permission)
		api.GET("/config/general", handler.GetGeneralSettings)                                                                                   // GET /api/config/general (get general settings, public)
		api.PUT("/config/general", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.UpdateGeneralSettings) // PUT /api/config/general (update general settings, requires admin permission)
	}

	// ===================== Static file hosting (must be after API routes) =====================
	// 1. Host uploaded files: frontend access /uploads/xxx corresponds to media folder in backend data directory
	mediaDir := filepath.Join(cfg.DataDir, "media")
	r.Static("/uploads", mediaDir)

	// 2. Host frontend built static assets: using embedded file system
	// Mount embedded dist directory to /assets path
	r.GET("/assets/*filepath", func(c *gin.Context) {
		// Remove /assets prefix
		file := strings.TrimPrefix(c.Param("filepath"), "/")
		// Read embedded file, need to add assets prefix because files are under dist/assets/
		// Use forward slashes for embed.FS compatibility across platforms
		content, err := public.ReadAsset("assets/" + file)
		if err != nil {
			c.Status(404)
			return
		}
		// Set Content-Type based on file extension
		ext := filepath.Ext(file)
		switch ext {
		case ".js":
			c.Data(200, "application/javascript", content)
		case ".css":
			c.Data(200, "text/css", content)
		case ".html":
			c.Data(200, "text/html", content)
		case ".json":
			c.Data(200, "application/json", content)
		case ".png":
			c.Data(200, "image/png", content)
		case ".jpg", ".jpeg":
			c.Data(200, "image/jpeg", content)
		case ".gif":
			c.Data(200, "image/gif", content)
		case ".svg":
			c.Data(200, "image/svg+xml", content)
		case ".ico":
			c.Data(200, "image/x-icon", content)
		case ".woff":
			c.Data(200, "font/woff", content)
		case ".woff2":
			c.Data(200, "font/woff2", content)
		default:
			c.Data(200, "application/octet-stream", content)
		}
	})

	// 3. Frontend entry page: root path / returns embedded index.html
	r.GET("/", func(c *gin.Context) {
		c.Data(200, "text/html; charset=utf-8", public.GetIndexHTML())
	})

	// ===================== Frontend SPA route compatibility (defined last) =====================
	// Handle React/Vue client-side routes (like /login, /posts/1, etc.)
	// Must be placed after all API and static file routes to ensure API requests are matched first
	r.NoRoute(func(c *gin.Context) {
		// For non-API requests, return embedded index.html to support frontend routing
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Data(200, "text/html; charset=utf-8", public.GetIndexHTML())
			return
		}
		// API request not matched, return 404
		c.JSON(404, gin.H{"error": "Not Found"})
	})

	// Start HTTP service, listen on configured address and port
	r.Run(cfg.GetListenAddr())
}
