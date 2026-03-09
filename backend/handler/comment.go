package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 获取某篇文章的评论列表
func GetComments(c *gin.Context) {
	postID := c.Param("id")
	var comments []model.Comment
	db.Where("post_id = ? AND status = ?", postID, "published").
		Preload("User").
		Order("created_at ASC").
		Find(&comments)

	// 获取当前用户信息（用于隐私过滤）
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

	// 对评论作者应用隐私过滤
	for i := range comments {
		author := &comments[i].User
		// 如果不是管理员且不是查看自己的评论，则应用隐私过滤
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && author.ID != currentUserID {
			FilterUserByPrivacy(author, currentUserID, currentUserRole)
		}
	}

	c.JSON(http.StatusOK, gin.H{"comments": comments})
}

// 创建评论（需登录）
func CreateComment(c *gin.Context) {
	// 支持前端传入 postId 为数字或字符串
	var req struct {
		PostID   interface{} `json:"postId" binding:"required"`
		Content  string      `json:"content" binding:"required"`
		ParentID *uint       `json:"parentId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 解析 PostID 为 uint
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
		// 如果无法解析，返回错误
		c.JSON(http.StatusBadRequest, gin.H{"error": "postId 类型不合法"})
		return
	}

	uid, _ := c.Get("userID")
	userID, ok := uid.(uint)
	if !ok {
		// 拒绝未登录请求
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取评论审核配置
	var config model.CommentModerationConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 配置不存在时使用默认值
			config = model.CommentModerationConfig{
				Enabled:            false,
				ModelProvider:      "",
				ApiKey:             "",
				ApiEndpoint:        "",
				ModelName:          "gpt-3.5-turbo",
				ModerationPrompt:   "请审核以下评论内容是否合规。如果评论包含违法不良信息、人身攻击、色情低俗等内容，请返回 'REJECT'；如果评论合规，请返回 'APPROVE'。只需返回结果，不要解释。\n\n评论内容：\n{{content}}",
				BlockKeywords:      "",
				AutoApproveEnabled: true,
				MinScoreThreshold:  0.5,
			}
		} else {
			// 其他数据库错误
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取评论审核配置失败"})
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

	// 设置评论状态
	if config.Enabled {
		// 如果启用了AI审核，先设置为待审核状态
		comment.Status = "pending"
	} else {
		// 如果未启用AI审核，根据配置决定是否自动批准
		if config.AutoApproveEnabled {
			comment.Status = "published"
		} else {
			comment.Status = "pending" // 仍需人工审核
		}
	}

	if err := db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建评论失败"})
		return
	}

	// 如果启用了AI审核，则进行审核
	if config.Enabled {
		approved, _, err := ModerateCommentAI(req.Content, config)
		if err != nil {
			// 如果AI审核失败，记录错误但不影响评论创建
			fmt.Printf("AI审核失败: %v\n", err)
			comment.Status = "published" // 失败时默认发布
		} else {
			if approved {
				comment.Status = "published"
			} else {
				comment.Status = "rejected"
			}
		}

		// 更新评论状态
		if err := db.Save(&comment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新评论状态失败"})
			return
		}
	}

	// 返回创建的评论及更新后的评论计数
	var count int64
	db.Model(&model.Comment{}).Where("post_id = ? AND status = ?", postID, "published").Count(&count)

	// 预加载作者信息
	db.Preload("User").First(&comment, comment.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message":            "评论创建成功",
		"comment":            comment,
		"commentsCount":      count,
		"requiresModeration": comment.Status == "pending",
	})
}

// 删除评论（需登录，作者或管理员）
func DeleteComment(c *gin.Context) {
	id := c.Param("id")
	var comment model.Comment
	if err := db.First(&comment, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
		return
	}

	// 获取当前操作用户 ID
	uid, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户信息"})
		return
	}

	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 管理员或超级管理员可以删除任意评论，作者本人也可以删除自己的评论
	if !model.IsAdmin(user) && comment.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该评论"})
		return
	}

	if err := db.Delete(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除评论失败"})
		return
	}

	// 返回删除后的评论计数，便于前端同步
	var count int64
	db.Model(&model.Comment{}).Where("post_id = ?", comment.PostID).Count(&count)

	c.JSON(http.StatusOK, gin.H{"message": "评论已删除", "commentsCount": count})
}
