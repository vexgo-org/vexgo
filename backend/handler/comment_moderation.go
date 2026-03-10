package handler

import (
	"net/http"
	"strconv"
	"strings"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetCommentModerationConfig gets comment moderation configuration
func GetCommentModerationConfig(c *gin.Context) {
	var config model.CommentModerationConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default configuration
			c.JSON(http.StatusOK, model.CommentModerationConfig{
				Enabled:            false,
				ModelProvider:      "",
				ApiKey:             "",
				ApiEndpoint:        "",
				ModelName:          "gpt-3.5-turbo",
				ModerationPrompt:   "Please review the following comment for compliance. If the comment contains illegal content, personal attacks, or inappropriate material, return 'REJECT'; if the comment is compliant, return 'APPROVE'. Only return the result, no explanation.\n\nComment content:\n{{content}}",
				BlockKeywords:      "",
				AutoApproveEnabled: true,
				MinScoreThreshold:  0.5,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comment moderation configuration"})
		return
	}

	// Don't return sensitive information like API key
	config.ApiKey = ""
	c.JSON(http.StatusOK, config)
}

// UpdateCommentModerationConfig updates comment moderation configuration
func UpdateCommentModerationConfig(c *gin.Context) {
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

	// Get existing configuration
	var config model.CommentModerationConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new configuration
			config = model.CommentModerationConfig{
				Enabled:            req.Enabled,
				ModelProvider:      req.ModelProvider,
				ApiKey:             req.ApiKey,
				ApiEndpoint:        req.ApiEndpoint,
				ModelName:          req.ModelName,
				ModerationPrompt:   req.ModerationPrompt,
				BlockKeywords:      req.BlockKeywords,
				AutoApproveEnabled: req.AutoApproveEnabled,
				MinScoreThreshold:  req.MinScoreThreshold,
			}
			if err := db.Create(&config).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment moderation configuration"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comment moderation configuration"})
			return
		}
	} else {
		// Update existing configuration
		config.Enabled = req.Enabled
		config.ModelProvider = req.ModelProvider
		config.ApiEndpoint = req.ApiEndpoint
		config.ModelName = req.ModelName
		config.ModerationPrompt = req.ModerationPrompt
		config.BlockKeywords = req.BlockKeywords
		config.AutoApproveEnabled = req.AutoApproveEnabled
		config.MinScoreThreshold = req.MinScoreThreshold

		// Only update if new API key is provided
		if req.ApiKey != "" {
			config.ApiKey = req.ApiKey
		}

		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update comment moderation configuration"})
			return
		}
	}

	// Don't return sensitive information
	config.ApiKey = ""
	c.JSON(http.StatusOK, gin.H{
		"message": "Comment moderation configuration updated successfully",
		"config":  config,
	})
}

// GetPendingComments gets pending comments for moderation
func GetPendingComments(c *gin.Context) {
	var comments []model.Comment

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

	query := db.Model(&model.Comment{}).
		Preload("User").
		Preload("Post").
		Where("status = ?", "pending")

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((pageNum - 1) * limitNum).
		Limit(limitNum).
		Find(&comments)

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

// GetApprovedComments gets approved comments
func GetApprovedComments(c *gin.Context) {
	var comments []model.Comment

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

	query := db.Model(&model.Comment{}).
		Preload("User").
		Preload("Post").
		Where("status = ?", "published")

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((pageNum - 1) * limitNum).
		Limit(limitNum).
		Find(&comments)

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

// GetRejectedComments gets rejected comments
func GetRejectedComments(c *gin.Context) {
	var comments []model.Comment

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

	query := db.Model(&model.Comment{}).
		Preload("User").
		Preload("Post").
		Where("status = ?", "rejected")

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((pageNum - 1) * limitNum).
		Limit(limitNum).
		Find(&comments)

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
func ApproveComment(c *gin.Context) {
	id := c.Param("id")
	var comment model.Comment
	if err := db.First(&comment, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Comment does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comment"})
		return
	}

	comment.Status = "published"
	if err := db.Save(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve comment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Comment approved",
		"comment": comment,
	})
}

// RejectComment rejects a comment
func RejectComment(c *gin.Context) {
	id := c.Param("id")
	var comment model.Comment
	if err := db.First(&comment, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Comment does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comment"})
		return
	}

	comment.Status = "rejected"
	if err := db.Save(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject comment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Comment rejected",
		"comment": comment,
	})
}

// ModerateCommentAI uses simulated AI moderation (should be replaced with real AI API call in production)
func ModerateCommentAI(content string, config model.CommentModerationConfig) (bool, string, error) {
	if !config.Enabled {
		return true, "", nil // if not enabled, auto approve
	}

	// Check blocked keywords
	if config.BlockKeywords != "" {
		keywords := strings.Split(config.BlockKeywords, ",")
		for _, keyword := range keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" && strings.Contains(strings.ToLower(content), strings.ToLower(keyword)) {
				return false, "Contains blocked keyword: " + keyword, nil
			}
		}
	}

	// Simulate AI moderation logic (should be replaced with real AI API call in production)
	// This is just a simple keyword check as an example
	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, "垃圾") || strings.Contains(lowerContent, "spam") ||
		strings.Contains(lowerContent, "广告") || strings.Contains(lowerContent, "ad") {
		return false, "AI detected non-compliant content", nil
	}

	// Simulate AI moderation approval
	return true, "", nil
}
