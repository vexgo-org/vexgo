package main

import (
	"blog-system/backend/config"
	"blog-system/backend/handler"
	"blog-system/backend/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 初始化配置（加载JWT密钥等环境变量）
	config.Init()

	// 2. 初始化数据库连接（确保数据库驱动、连接串配置正确）
	handler.InitDB()

	// 3. 创建Gin引擎实例（默认包含Logger和Recovery中间件）
	r := gin.Default()

	// ===================== 核心API路由分组（所有接口都在/api下） =====================
	// 匹配前端Axios的baseURL: /api，确保所有前端API请求都能命中
	api := r.Group("/api")
	{
		// -------------------- 公开API（无需JWT认证） --------------------
		// 文章相关公开接口
		api.GET("/posts", handler.GetPosts)    // GET /api/posts（获取文章列表）
		api.GET("/posts/:id", handler.GetPost) // GET /api/posts/:id（获取单篇文章）

		// 邮箱验证公开接口
		api.GET("/verify-email", handler.VerifyEmail) // GET /api/verify-email（验证邮箱）

		// 分类/标签公开接口
		api.GET("/categories", handler.GetCategories) // GET /api/categories（获取分类列表）
		api.GET("/tags", handler.GetTags)             // GET /api/tags（获取标签列表）

		// 统计相关公开接口
		api.GET("/stats", handler.GetStats)                      // GET /api/stats（获取统计数据）
		api.GET("/stats/popular-posts", handler.GetPopularPosts) // GET /api/stats/popular-posts
		api.GET("/stats/latest-posts", handler.GetLatestPosts)   // GET /api/stats/latest-posts

		// 评论/点赞公开接口
		api.GET("/comments/post/:id", handler.GetComments) // GET /api/comments/post/:id（获取文章评论）
		api.GET("/likes/:postId", handler.GetLikeStatus)   // GET /api/likes/:postId（获取点赞状态）

		// -------------------- 认证相关API（/api/auth子分组） --------------------
		// 匹配前端authApi的所有接口：/api/auth/xxx
		auth := api.Group("/auth")
		{
			auth.POST("/register", handler.Register)                                                 // POST /api/auth/register（注册）
			auth.POST("/login", handler.Login)                                                       // POST /api/auth/login（登录）
			auth.GET("/me", middleware.JWTAuth(), handler.GetCurrentUser)                            // GET /api/auth/me（获取当前用户，需认证）
			auth.GET("/user", middleware.JWTAuth(), handler.GetCurrentUser)                          // 向后兼容：/api/auth/user
			auth.PUT("/profile", middleware.JWTAuth(), handler.UpdateProfile)                        // PUT /api/auth/profile（更新个人信息）
			auth.PUT("/password", middleware.JWTAuth(), handler.ChangePassword)                      // PUT /api/auth/password（修改密码）
			auth.GET("/verification-status", middleware.JWTAuth(), handler.GetVerificationStatus)    // GET /api/auth/verification-status（获取验证状态）
			auth.POST("/resend-verification", middleware.JWTAuth(), handler.ResendVerificationEmail) // POST /api/auth/resend-verification（重新发送验证邮件）
		}

		// -------------------- 需要JWT认证的业务API --------------------
		// 文章操作（需登录）
		api.POST("/posts", middleware.JWTAuth(), handler.CreatePost)              // POST /api/posts（创建文章）
		api.GET("/posts/user/my-posts", middleware.JWTAuth(), handler.GetMyPosts) // GET /api/posts/user/my-posts（我的文章）
		api.GET("/posts/drafts", middleware.JWTAuth(), handler.GetDraftPosts)     // GET /api/posts/drafts（草稿文章）
		api.PUT("/posts/:id", middleware.JWTAuth(), handler.UpdatePost)           // PUT /api/posts/:id（更新文章）
		api.DELETE("/posts/:id", middleware.JWTAuth(), handler.DeletePost)        // DELETE /api/posts/:id（删除文章）

		// 分类/标签操作（需登录）
		api.POST("/categories", middleware.JWTAuth(), handler.CreateCategory) // POST /api/categories（创建分类）
		api.POST("/tags", middleware.JWTAuth(), handler.CreateTag)            // POST /api/tags（创建标签）

		// 评论操作（需登录）
		api.POST("/comments", middleware.JWTAuth(), handler.CreateComment)       // POST /api/comments（创建评论）
		api.DELETE("/comments/:id", middleware.JWTAuth(), handler.DeleteComment) // DELETE /api/comments/:id（删除评论）

		// 点赞操作（需登录）
		api.POST("/likes/:postId", middleware.JWTAuth(), handler.ToggleLike) // POST /api/likes/:postId（切换点赞）

		// 上传文件操作（需登录）
		api.POST("/upload/file", middleware.JWTAuth(), handler.UploadFile)    // POST /api/upload/file（单文件上传）
		api.POST("/upload/files", middleware.JWTAuth(), handler.UploadFiles)  // POST /api/upload/files（多文件上传）
		api.GET("/upload/my-files", middleware.JWTAuth(), handler.GetMyFiles) // GET /api/upload/my-files（我的文件）
		api.DELETE("/upload/:id", middleware.JWTAuth(), handler.DeleteFile)   // DELETE /api/upload/:id（删除文件）

		// 文章审核相关API（需要管理员权限）
		api.GET("/moderation/pending", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetPendingPosts)   // GET /api/moderation/pending（获取待审核文章）
		api.GET("/moderation/approved", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetApprovedPosts) // GET /api/moderation/approved（获取已通过文章）
		api.GET("/moderation/rejected", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetRejectedPosts) // GET /api/moderation/rejected（获取已拒绝文章）
		api.PUT("/moderation/approve/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.ApprovePost)   // PUT /api/moderation/approve/:id（审核通过文章）
		api.PUT("/moderation/reject/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.RejectPost)     // PUT /api/moderation/reject/:id（拒绝文章）
		api.PUT("/moderation/resubmit/:id", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.ResubmitPost) // PUT /api/moderation/resubmit/:id（重新提交审核文章）

		// 用户管理相关API（需要管理员权限）
		api.GET("/users", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetUserList)             // GET /api/users（获取用户列表）
		api.PUT("/users/:id/role", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.UpdateUserRole) // PUT /api/users/:id/role（更新用户角色）

		// SMTP 配置相关API（需要管理员权限）
		api.GET("/config/smtp", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.GetSMTPConfig)    // GET /api/config/smtp（获取SMTP配置）
		api.PUT("/config/smtp", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.UpdateSMTPConfig) // PUT /api/config/smtp（更新SMTP配置）
		api.POST("/config/smtp/test", middleware.JWTAuth(), middleware.PermissionMiddleware("admin", "super_admin"), handler.TestSMTP)   // POST /api/config/smtp/test（测试SMTP配置）
	}

	// ===================== 静态文件托管（必须在API路由之后） =====================
	// 1. 托管上传的文件：前端访问 /uploads/xxx 对应后端 ../frontend/dist/uploads 目录
	r.Static("/uploads", "../frontend/dist/uploads")
	// 2. 托管前端打包后的静态资源：/assets/xxx 对应 ../frontend/dist/assets 目录
	r.Static("/assets", "../frontend/dist/assets")
	// 3. 前端入口页面：根路径 / 返回index.html
	r.StaticFile("/", "../frontend/dist/index.html")

	// ===================== 前端SPA路由兼容（最后定义） =====================
	// 处理React/Vue的客户端路由（如 /login、/posts/1 等）
	// 必须放在所有API和静态文件路由之后，确保API请求优先匹配
	r.NoRoute(func(c *gin.Context) {
		c.File("../frontend/dist/index.html")
	})

	// 4. 启动HTTP服务，监听3001端口（和前端请求的端口保持一致）
	// 若需修改端口，需同步修改前端.env文件中的VITE_API_URL（如 http://localhost:3001/api）
	r.Run(":3001")
}
