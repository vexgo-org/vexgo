package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Get comments for a specific post
func GetComments(c *gin.Context) {
	postID := c.Param("id")
	var comments []model.Comment
	db.Where("post_id = ? AND status = ?", postID, "published").
		Preload("User").
		Order("created_at ASC").
		Find(&comments)

	// Get current user information (for privacy filtering)
	var currentUserRole string
	var currentUserID uint
	if uidVal, exists := c.Get("userID"); exists {
		switch v := uidVal.(type) {
		case uint:
			currentUserID = v
		case int:
			currentUserID = uint(v)
		case float64:
			currentUserID = uint(v)
		}
	}
	if userContext, exists := c.Get("user"); exists {
		if userMap, ok := userContext.(map[string]interface{}); ok {
			if role, ok := userMap["role"].(string); ok {
				currentUserRole = role
			}
		}
	}

	// Apply privacy filtering to comment authors
	for i := range comments {
		author := &comments[i].User
		// If not admin and not viewing own comment, apply privacy filtering
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && author.ID != currentUserID {
			FilterUserByPrivacy(author, currentUserID, currentUserRole)
		}
	}

	c.JSON(http.StatusOK, gin.H{"comments": comments})
}

// Create comment (requires login)
func CreateComment(c *gin.Context) {
	// Support postId as number or string from frontend
	var req struct {
		PostID   interface{} `json:"postId" binding:"required"`
		Content  string      `json:"content" binding:"required"`
		ParentID *uint       `json:"parentId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check comment length limit (no more than 100 characters)
	if len(req.Content) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Comment cannot exceed 100 characters"})
		return
	}

	// Parse PostID to uint
	var postID uint
	switch v := req.PostID.(type) {
	case float64:
		postID = uint(v)
	case string:
		if id64, err := strconv.ParseUint(v, 10, 64); err == nil {
			postID = uint(id64)
		}
	case int:
		postID = uint(v)
	case uint:
		postID = v
	default:
		// If cannot parse, return error
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid postId type"})
		return
	}

	uid, _ := c.Get("userID")
	userID, ok := uid.(uint)
	if !ok {
		// Reject unauthenticated request
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	// Get comment moderation configuration
	var config model.CommentModerationConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Use default values when configuration doesn't exist
			config = model.CommentModerationConfig{
				Enabled:            false,
				ModelProvider:      "",
				ApiKey:             "",
				ApiEndpoint:        "",
				ModelName:          "gpt-3.5-turbo",
				ModerationPrompt:   "Please review the following comment for compliance. If the comment contains illegal content, personal attacks, or inappropriate material, return 'REJECT'; if the comment is compliant, return 'APPROVE'. Only return the result, no explanation.\n\nComment content:\n{{content}}",
				BlockKeywords:      "",
				AutoApproveEnabled: true,
				MinScoreThreshold:  0.5,
			}
		} else {
			// Other database errors
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comment moderation configuration"})
			return
		}
	}

	comment := model.Comment{
		PostID:  postID,
		Content: req.Content,
		UserID:  userID,
	}

	if req.ParentID != nil {
		comment.ParentID = req.ParentID
	}

	// Set comment status
	if config.Enabled {
		// If AI moderation enabled, set to pending status first
		comment.Status = "pending"
	} else {
		// If AI moderation not enabled, decide whether to auto-approve based on config
		if config.AutoApproveEnabled {
			comment.Status = "published"
		} else {
			comment.Status = "pending" // still requires manual moderation
		}
	}

	if err := db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	// If AI moderation enabled, perform moderation
	if config.Enabled {
		approved, _, err := ModerateCommentAI(req.Content, config)
		if err != nil {
			// If AI moderation fails, log error but don't affect comment creation
			fmt.Printf("AI moderation failed: %v\n", err)
			comment.Status = "published" // default to published on failure
		} else {
			if approved {
				comment.Status = "published"
			} else {
				comment.Status = "rejected"
			}
		}

		// 更新评论状态
		if err := db.Save(&comment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update comment status"})
			return
		}
	}

	// Return created comment and updated comment count
	var count int64
	db.Model(&model.Comment{}).Where("post_id = ? AND status = ?", postID, "published").Count(&count)

	// Preload author information
	db.Preload("User").First(&comment, comment.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message":            "Comment created successfully",
		"comment":            comment,
		"commentsCount":      count,
		"requiresModeration": comment.Status == "pending",
	})
}

// Delete comment (requires login, author or admin)
func DeleteComment(c *gin.Context) {
	id := c.Param("id")
	var comment model.Comment
	if err := db.First(&comment, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment does not exist"})
		return
	}

	// Get current operating user ID
	uid, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	var userID uint
	switch v := uid.(type) {
	case uint:
		userID = v
	case int:
		userID = uint(v)
	case float64:
		userID = uint(v)
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user information"})
		return
	}

	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	// Admins or super admins can delete any comment, authors can delete their own comments
	if !model.IsAdmin(user) && comment.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this comment"})
		return
	}

	if err := db.Delete(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete comment"})
		return
	}

	// Return comment count after deletion for frontend sync
	var count int64
	db.Model(&model.Comment{}).Where("post_id = ?", comment.PostID).Count(&count)

	c.JSON(http.StatusOK, gin.H{"message": "Comment deleted", "commentsCount": count})
}
