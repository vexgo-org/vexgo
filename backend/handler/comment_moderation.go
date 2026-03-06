package handler

import (
	"net/http"
	"strconv"
	"strings"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetCommentModerationConfig 获取评论审核配置
func GetCommentModerationConfig(c *gin.Context) {
	var config model.CommentModerationConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回默认配置
			c.JSON(http.StatusOK, model.CommentModerationConfig{
				Enabled:            false,
				ModelProvider:      "",
				ApiKey:             "",
				ApiEndpoint:        "",
				ModelName:          "gpt-3.5-turbo",
				ModerationPrompt:   "请审核以下评论内容是否合规。如果评论包含违法不良信息、人身攻击、色情低俗等内容，请返回 'REJECT'；如果评论合规，请返回 'APPROVE'。只需返回结果，不要解释。\n\n评论内容：\n{{content}}",
				BlockKeywords:      "",
				AutoApproveEnabled: true,
				MinScoreThreshold:  0.5,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取评论审核配置失败"})
		return
	}

	// 不返回敏感信息如API密钥
	config.ApiKey = ""
	c.JSON(http.StatusOK, config)
}

// UpdateCommentModerationConfig 更新评论审核配置
func UpdateCommentModerationConfig(c *gin.Context) {
	var req struct {
		Enabled            bool    `json:"enabled"`
		ModelProvider      string  `json:"modelProvider"`
		ApiKey             string  `json:"apiKey"` // 如果为空则不更新
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

	// 获取现有配置
	var config model.CommentModerationConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新配置
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": "创建评论审核配置失败"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取评论审核配置失败"})
			return
		}
	} else {
		// 更新现有配置
		config.Enabled = req.Enabled
		config.ModelProvider = req.ModelProvider
		config.ApiEndpoint = req.ApiEndpoint
		config.ModelName = req.ModelName
		config.ModerationPrompt = req.ModerationPrompt
		config.BlockKeywords = req.BlockKeywords
		config.AutoApproveEnabled = req.AutoApproveEnabled
		config.MinScoreThreshold = req.MinScoreThreshold

		// 只有当提供了新API密钥时才更新
		if req.ApiKey != "" {
			config.ApiKey = req.ApiKey
		}

		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新评论审核配置失败"})
			return
		}
	}

	// 不返回敏感信息
	config.ApiKey = ""
	c.JSON(http.StatusOK, gin.H{
		"message": "评论审核配置更新成功",
		"config":  config,
	})
}

// GetPendingComments 获取待审核评论
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

// GetApprovedComments 获取已通过审核的评论
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

// GetRejectedComments 获取已拒绝的评论
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

// ApproveComment 批准评论
func ApproveComment(c *gin.Context) {
	id := c.Param("id")
	var comment model.Comment
	if err := db.First(&comment, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取评论失败"})
		return
	}

	comment.Status = "published"
	if err := db.Save(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批准评论失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "评论已批准",
		"comment": comment,
	})
}

// RejectComment 拒绝评论
func RejectComment(c *gin.Context) {
	id := c.Param("id")
	var comment model.Comment
	if err := db.First(&comment, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取评论失败"})
		return
	}

	comment.Status = "rejected"
	if err := db.Save(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "拒绝评论失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "评论已拒绝",
		"comment": comment,
	})
}

// ModerateCommentAI 使用模拟AI审核（实际项目中应替换为真实AI API调用）
func ModerateCommentAI(content string, config model.CommentModerationConfig) (bool, string, error) {
	if !config.Enabled {
		return true, "", nil // 如果未启用，则自动批准
	}

	// 检查屏蔽关键词
	if config.BlockKeywords != "" {
		keywords := strings.Split(config.BlockKeywords, ",")
		for _, keyword := range keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" && strings.Contains(strings.ToLower(content), strings.ToLower(keyword)) {
				return false, "包含屏蔽关键词: " + keyword, nil
			}
		}
	}

	// 模拟AI审核逻辑（在实际项目中应替换为真实的AI API调用）
	// 这里只是简单的关键词检查作为示例
	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, "垃圾") || strings.Contains(lowerContent, "spam") ||
		strings.Contains(lowerContent, "广告") || strings.Contains(lowerContent, "ad") {
		return false, "AI检测到不合规内容", nil
	}

	// 模拟AI审核通过
	return true, "", nil
}
