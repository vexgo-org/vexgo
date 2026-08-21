package comment

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the comment domain routes on the /api group.
// Route paths and middleware chains are identical to the original registration
// in the legacy handler package.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/comments/post/:id", h.GetComments)
	api.POST("/comments", h.mw.JWTAuth(), h.CreateComment)
	api.DELETE("/comments/:id", h.mw.JWTAuth(), h.DeleteComment)

	admin := h.mw.Permission(model.RoleAdmin, model.RoleSuperAdmin)
	api.GET("/moderation/comments/pending", h.mw.JWTAuth(), admin, h.GetPendingComments)
	api.GET("/moderation/comments/approved", h.mw.JWTAuth(), admin, h.GetApprovedComments)
	api.GET("/moderation/comments/rejected", h.mw.JWTAuth(), admin, h.GetRejectedComments)
	api.PUT("/moderation/comments/approve/:id", h.mw.JWTAuth(), admin, h.ApproveComment)
	api.PUT("/moderation/comments/reject/:id", h.mw.JWTAuth(), admin, h.RejectComment)
	api.GET("/moderation/comments/config", h.mw.JWTAuth(), admin, h.GetCommentModerationConfig)
	api.PUT("/moderation/comments/config", h.mw.JWTAuth(), admin, h.UpdateCommentModerationConfig)
}
