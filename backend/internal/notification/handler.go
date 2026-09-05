package notification

import (
	"net/http"
	"strconv"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// Handler exposes the notification domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates a notification HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// GetNotifications retrieves the notification list
func (h *Handler) GetNotifications(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	page, limit := middleware.ParsePagination(c, 10)

	notifications, total, err := h.svc.List(c.Request.Context(), ListQuery{
		UserID:           uid,
		Page:             page,
		Limit:            limit,
		NotificationType: model.NotificationType(c.Query("type")),
		IsRead:           c.Query("is_read"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}

	c.JSON(http.StatusOK, api.NotificationsResponse{
		Notifications: notifications,
		Pagination: api.Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: int((total + int64(limit) - 1) / int64(limit)),
		},
	})
}

// MarkAsRead marks a notification as read
func (h *Handler) MarkAsRead(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	rowsAffected, err := h.svc.MarkAsRead(c.Request.Context(), uid, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found or not updated"})
		return
	}

	c.JSON(http.StatusOK, api.MessageResponse{Message: "Notification marked as read"})
}

// MarkAllAsRead marks all notifications as read
func (h *Handler) MarkAllAsRead(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	if err := h.svc.MarkAllAsRead(c.Request.Context(), uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all notifications as read"})
		return
	}

	c.JSON(http.StatusOK, api.MessageResponse{Message: "All notifications marked as read"})
}

// DeleteNotification deletes a notification
func (h *Handler) DeleteNotification(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	rowsAffected, err := h.svc.Delete(c.Request.Context(), uid, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete notification"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found or not deleted"})
		return
	}

	c.JSON(http.StatusOK, api.MessageResponse{Message: "Notification deleted"})
}

// GetUnreadCount retrieves the number of unread notifications
func (h *Handler) GetUnreadCount(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	count, err := h.svc.UnreadCount(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch unread count"})
		return
	}

	c.JSON(http.StatusOK, api.UnreadCountResponse{UnreadCount: count})
}
