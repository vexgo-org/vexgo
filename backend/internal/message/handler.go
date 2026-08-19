package message

import (
	"net/http"
	"strconv"

	"vexgo/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Handler exposes the message domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates a message HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// GetMessages retrieves the message list
func (h *Handler) GetMessages(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	// Pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	notifications, total, err := h.svc.List(uid, page, limit, c.Query("type"), c.Query("is_read"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"pagination": gin.H{
			"total":      total,
			"page":       page,
			"limit":      limit,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// MarkAsRead marks a message as read
func (h *Handler) MarkAsRead(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	rowsAffected, err := h.svc.MarkAsRead(uid, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark message as read"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found or not updated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message marked as read"})
}

// MarkAllAsRead marks all messages as read
func (h *Handler) MarkAllAsRead(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	if err := h.svc.MarkAllAsRead(uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all messages as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All messages marked as read"})
}

// DeleteMessage deletes a message
func (h *Handler) DeleteMessage(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	rowsAffected, err := h.svc.Delete(uid, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete message"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found or not deleted"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted"})
}

// GetUnreadCount retrieves the number of unread messages
func (h *Handler) GetUnreadCount(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	count, err := h.svc.UnreadCount(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch unread count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"unreadCount": count})
}
