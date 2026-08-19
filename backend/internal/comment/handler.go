package comment

import (
	"errors"
	"net/http"
	"strconv"

	"vexgo/backend/internal/middleware"
	"vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// Handler exposes the comment domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates a comment HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// GetComments gets comments for a specific post
func (h *Handler) GetComments(c *gin.Context) {
	postID := c.Param("id")

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
		if userMap, ok := userContext.(map[string]any); ok {
			if role, ok := userMap["role"].(string); ok {
				currentUserRole = role
			}
		}
	}

	comments, _ := h.svc.ListByPost(postID, currentUserID, currentUserRole)

	c.JSON(http.StatusOK, gin.H{"comments": comments})
}

// CreateComment creates a comment (requires login)
func (h *Handler) CreateComment(c *gin.Context) {
	// Support postId as number or string from frontend
	var req struct {
		PostID   any    `json:"postId" binding:"required"`
		Content  string `json:"content" binding:"required"`
		ParentID *uint  `json:"parentId"`
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

	comment, count, err := h.svc.Create(postID, userID, req.Content, req.ParentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":            "Comment created successfully",
		"comment":            comment,
		"commentsCount":      count,
		"requiresModeration": comment.Status == model.CommentStatusPending,
	})
}

// DeleteComment deletes a comment (requires login, author or admin)
func (h *Handler) DeleteComment(c *gin.Context) {
	id := c.Param("id")

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

	count, err := h.svc.Delete(id, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrCommentNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Comment does not exist"})
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this comment"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete comment"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comment deleted", "commentsCount": count})
}

// GetCommentModerationConfig gets comment moderation configuration
func (h *Handler) GetCommentModerationConfig(c *gin.Context) {
	config, err := h.svc.GetModerationConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comment moderation configuration"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateCommentModerationConfig updates comment moderation configuration
func (h *Handler) UpdateCommentModerationConfig(c *gin.Context) {
	var req struct {
		Enabled            bool    `json:"enabled"`
		ModelProvider      string  `json:"modelProvider"`
		ApiKey             string  `json:"apiKey"` // if empty, don't update
		ApiEndpoint        string  `json:"apiEndpoint"`
		ModelName          string  `json:"modelName"`
		ModerationPrompt   string  `json:"moderationPrompt"`
		BlockKeywords      string  `json:"blockKeywords"`
		AutoApproveEnabled bool    `json:"autoApproveEnabled"`
		MinScoreThreshold  float64 `json:"minScoreThreshold"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.svc.UpdateModerationConfig(UpdateModerationConfigRequest{
		Enabled:            req.Enabled,
		ModelProvider:      req.ModelProvider,
		ApiKey:             req.ApiKey,
		ApiEndpoint:        req.ApiEndpoint,
		ModelName:          req.ModelName,
		ModerationPrompt:   req.ModerationPrompt,
		BlockKeywords:      req.BlockKeywords,
		AutoApproveEnabled: req.AutoApproveEnabled,
		MinScoreThreshold:  req.MinScoreThreshold,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update comment moderation configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Comment moderation configuration updated successfully",
		"config":  config,
	})
}

// GetPendingComments gets pending comments for moderation
func (h *Handler) GetPendingComments(c *gin.Context) {
	h.listModeration(c, model.CommentStatusPending)
}

// GetApprovedComments gets approved comments
func (h *Handler) GetApprovedComments(c *gin.Context) {
	h.listModeration(c, model.CommentStatusPublished)
}

// GetRejectedComments gets rejected comments
func (h *Handler) GetRejectedComments(c *gin.Context) {
	h.listModeration(c, model.CommentStatusRejected)
}

// listModeration renders the moderation queue for a given comment status.
func (h *Handler) listModeration(c *gin.Context, status model.CommentStatus) {
	page, _ := c.GetQuery("page")
	if page == "" {
		page = "1"
	}
	pageNum := 1
	if val, err := strconv.Atoi(page); err == nil && val > 0 {
		pageNum = val
	}

	limit, _ := c.GetQuery("limit")
	if limit == "" {
		limit = "10"
	}
	limitNum := 10
	if val, err := strconv.Atoi(limit); err == nil && val > 0 && val <= 100 {
		limitNum = val
	}

	comments, total, _ := h.svc.ListModeration(status, pageNum, limitNum)

	totalPages := (int(total) + limitNum - 1) / limitNum
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"comments": comments,
		"pagination": gin.H{
			"total":      total,
			"page":       pageNum,
			"limit":      limitNum,
			"totalPages": totalPages,
		},
	})
}

// ApproveComment approves a comment
func (h *Handler) ApproveComment(c *gin.Context) {
	h.setStatus(c, model.CommentStatusPublished, "Comment approved", "Failed to approve comment")
}

// RejectComment rejects a comment
func (h *Handler) RejectComment(c *gin.Context) {
	h.setStatus(c, model.CommentStatusRejected, "Comment rejected", "Failed to reject comment")
}

// setStatus approves or rejects a comment and renders the result.
func (h *Handler) setStatus(c *gin.Context, status model.CommentStatus, successMsg, failureMsg string) {
	comment, err := h.svc.SetStatus(c.Param("id"), status)
	if err != nil {
		if errors.Is(err, ErrCommentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Comment does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": failureMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": successMsg,
		"comment": comment,
	})
}
