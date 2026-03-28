package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// GetPendingPosts gets pending posts for moderation
func GetPendingPosts(c *gin.Context) {
	var posts []model.Post

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.DefaultQuery("search", "")

	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", "pending")

	// Add search condition if search term is provided
	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Joins("LEFT JOIN users ON posts.author_id = users.id").
			Where("posts.title LIKE ? OR posts.content LIKE ? OR users.username LIKE ?", searchTerm, searchTerm, searchTerm)
	}

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

// ApprovePost approves a post
func ApprovePost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post
	if err := db.Preload("Tags").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
		return
	}

	// Update post status to published
	post.Status = "published"
	db.Save(&post)

	// Create notification for post author
	CreateNotification(
		post.AuthorID,
		"review",
		"Post approved",
		fmt.Sprintf("Your post \"%s\" has been approved", post.Title),
		id,
		"post",
	)

	c.JSON(http.StatusOK, gin.H{"message": "Post approved", "post": post})
}

// RejectPost rejects a post
func RejectPost(c *gin.Context) {
	id := c.Param("id")

	// Parse request body to get rejection reason
	var req struct {
		RejectionReason string `json:"rejectionReason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	var post model.Post
	if err := db.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
		return
	}

	// Update post status to rejected and record rejection reason
	post.Status = "rejected"
	post.RejectionReason = req.RejectionReason
	db.Save(&post)

	// Create notification for post author
	CreateNotification(
		post.AuthorID,
		"review",
		"post rejected",
		fmt.Sprintf("Your post \"%s\" has been rejected, reason: %s", post.Title, req.RejectionReason),
		id,
		"post",
	)

	c.JSON(http.StatusOK, gin.H{"message": "Post has been rejected", "post": post})
}

// GetApprovedPosts gets approved posts list
func GetApprovedPosts(c *gin.Context) {
	var posts []model.Post

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.DefaultQuery("search", "")

	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", "published")

	// Add search condition if search term is provided
	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Joins("LEFT JOIN users ON posts.author_id = users.id").
			Where("posts.title LIKE ? OR posts.content LIKE ? OR users.username LIKE ?", searchTerm, searchTerm, searchTerm)
	}

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

// ResubmitPost resubmits post for moderation
func ResubmitPost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post
	if err := db.Preload("Tags").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
		return
	}

	// Check if post status is rejected
	if post.Status != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only rejected posts can be resubmitted for moderation"})
		return
	}

	// Update post status to pending and clear rejection reason
	post.Status = "pending"
	post.RejectionReason = ""
	db.Save(&post)

	c.JSON(http.StatusOK, gin.H{"message": "Post resubmitted for moderation", "post": post})
}

// GetRejectedPosts gets rejected posts list
func GetRejectedPosts(c *gin.Context) {
	var posts []model.Post

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.DefaultQuery("search", "")

	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", "rejected")

	// Add search condition if search term is provided
	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Joins("LEFT JOIN users ON posts.author_id = users.id").
			Where("posts.title LIKE ? OR posts.content LIKE ? OR users.username LIKE ?", searchTerm, searchTerm, searchTerm)
	}

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