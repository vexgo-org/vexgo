package post

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the post domain routes on the /api group.
// Route paths and middleware chains are identical to the original registration
// in the legacy handler package.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/posts", h.GetPosts)
	api.GET("/posts/:id", h.GetPost)

	api.GET("/categories", h.GetCategories)
	api.GET("/tags", h.GetTags)

	api.GET("/stats/popular-posts", h.GetPopularPosts)
	api.GET("/stats/latest-posts", h.GetLatestPosts)

	api.GET("/likes/:postId", h.GetLikeStatus)
	api.GET("/posts/user/:id", h.GetUserPosts)

	api.POST("/posts", h.mw.JWTAuth(), h.CreatePost)
	api.GET("/posts/user/my-posts", h.mw.JWTAuth(), h.GetMyPosts)
	api.GET("/posts/drafts", h.mw.JWTAuth(), h.GetDraftPosts)
	api.PUT("/posts/:id", h.mw.JWTAuth(), h.UpdatePost)
	api.DELETE("/posts/:id", h.mw.JWTAuth(), h.DeletePost)

	api.POST("/categories", h.mw.JWTAuth(), h.CreateCategory)
	api.POST("/tags", h.mw.JWTAuth(), h.CreateTag)

	api.POST("/likes/:postId", h.mw.JWTAuth(), h.ToggleLike)

	admin := h.mw.Permission(model.RoleAdmin, model.RoleSuperAdmin)
	api.GET("/moderation/pending", h.mw.JWTAuth(), admin, h.GetPendingPosts)
	api.GET("/moderation/approved", h.mw.JWTAuth(), admin, h.GetApprovedPosts)
	api.GET("/moderation/rejected", h.mw.JWTAuth(), admin, h.GetRejectedPosts)
	api.PUT("/moderation/approve/:id", h.mw.JWTAuth(), admin, h.ApprovePost)
	api.PUT("/moderation/reject/:id", h.mw.JWTAuth(), admin, h.RejectPost)
	api.PUT("/moderation/resubmit/:id", h.mw.JWTAuth(), admin, h.ResubmitPost)
}
