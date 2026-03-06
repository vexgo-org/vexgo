package handler

import (
	"net/http"
	"strconv"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// GetPendingPosts 获取待审核的文章列表
func GetPendingPosts(c *gin.Context) {
	var posts []model.Post

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", "pending")

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts)

	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"pagination": gin.H{
			"total":      total,
			"page":       page,
			"limit":      limit,
			"totalPages": totalPages,
		},
	})
}

// ApprovePost 审核通过文章
func ApprovePost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post
	if err := db.Preload("Tags").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	// 更新文章状态为已发布
	post.Status = "published"
	db.Save(&post)

	c.JSON(http.StatusOK, gin.H{"message": "文章审核通过", "post": post})
}

// RejectPost 拒绝文章
func RejectPost(c *gin.Context) {
	id := c.Param("id")

	// 解析请求体，获取拒绝原因
	var req struct {
		RejectionReason string `json:"rejectionReason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var post model.Post
	if err := db.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	// 更新文章状态为已拒绝，并记录拒绝原因
	post.Status = "rejected"
	post.RejectionReason = req.RejectionReason
	db.Save(&post)

	c.JSON(http.StatusOK, gin.H{"message": "文章已被拒绝", "post": post})
}

// GetApprovedPosts 获取已通过的文章列表
func GetApprovedPosts(c *gin.Context) {
	var posts []model.Post

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", "published")

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts)

	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"pagination": gin.H{
			"total":      total,
			"page":       page,
			"limit":      limit,
			"totalPages": totalPages,
		},
	})
}

// ResubmitPost 重新提交文章审核
func ResubmitPost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post
	if err := db.Preload("Tags").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	// 检查文章状态是否为已拒绝
	if post.Status != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只有已拒绝的文章才能重新提交审核"})
		return
	}

	// 更新文章状态为待审核，并清除拒绝原因
	post.Status = "pending"
	post.RejectionReason = ""
	db.Save(&post)

	c.JSON(http.StatusOK, gin.H{"message": "文章已重新提交审核", "post": post})
}

// GetRejectedPosts 获取已拒绝的文章列表
func GetRejectedPosts(c *gin.Context) {
	var posts []model.Post

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", "rejected")

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts)

	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"pagination": gin.H{
			"total":      total,
			"page":       page,
			"limit":      limit,
			"totalPages": totalPages,
		},
	})
}
