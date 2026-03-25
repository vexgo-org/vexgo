package handler

import (
	"net/http"
	"strconv"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// GetMessages retrieves the message list
func GetMessages(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	// Pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	// Filter parameters
	messageType := c.Query("type")
	isRead := c.Query("is_read")

	// Build query
	query := db.Model(&model.Notification{}).Where("user_id = ?", uid)

	// Type filter
	if messageType != "" {
		query = query.Where("type = ?", messageType)
	}

	// Read status filter
	if isRead != "" {
		readStatus := isRead == "true"
		query = query.Where("is_read = ?", readStatus)
	}

	// Calculate total count
	var total int64
	query.Count(&total)

	// Query messages
	var notifications []model.Notification
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&notifications)

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
func MarkAsRead(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	// Directly update message status to avoid the problem of querying first and then updating
	result := db.Model(&model.Notification{}).Where("id = ? AND user_id = ?", id, uid).Update("is_read", true)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark message as read"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found or not updated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message marked as read"})
}

// MarkAllAsRead marks all messages as read
func MarkAllAsRead(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	// Mark all messages as read
	result := db.Model(&model.Notification{}).Where("user_id = ?", uid).Update("is_read", true)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all messages as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All messages marked as read"})
}

// DeleteMessage deletes a message
func DeleteMessage(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	// Directly delete the message to avoid the problem of querying first and then deleting
	result := db.Where("id = ? AND user_id = ?", id, uid).Delete(&model.Notification{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete message"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found or not deleted"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted"})
}

// GetUnreadCount retrieves the number of unread messages
func GetUnreadCount(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	// Calculate the number of unread messages
	var count int64
	db.Model(&model.Notification{}).Where("user_id = ? AND is_read = ?", uid, false).Count(&count)

	c.JSON(http.StatusOK, gin.H{"unreadCount": count})
}

// CreateNotification creates a notification
func CreateNotification(userID uint, notificationType string, title string, content string, relatedID string, relatedType string) error {
	notification := model.Notification{
		UserID:      userID,
		Type:        notificationType,
		Title:       title,
		Content:     content,
		RelatedID:   relatedID,
		RelatedType: relatedType,
		IsRead:      false,
	}

	return db.Create(&notification).Error
}
