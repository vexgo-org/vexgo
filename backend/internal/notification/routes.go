package notification

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the notification domain routes on the /api group.
// The route paths and middleware chains are identical to the original
// registration in the legacy handler package.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/notifications", h.mw.JWTAuth(), h.GetNotifications)
	api.GET("/notifications/unread-count", h.mw.JWTAuth(), h.GetUnreadCount)
	api.PUT("/notifications/:id/read", h.mw.JWTAuth(), h.MarkAsRead)
	api.PUT("/notifications/read-all", h.mw.JWTAuth(), h.MarkAllAsRead)
	api.DELETE("/notifications/:id", h.mw.JWTAuth(), h.DeleteNotification)
}
