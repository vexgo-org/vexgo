package handler

import (
	"net/http"
	"strconv"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// Get statistics
func GetStats(c *gin.Context) {
	var postsCount, usersCount, categoriesCount, tagsCount, commentsCount int64

	db.Model(&model.Post{}).Count(&postsCount)
	db.Model(&model.User{}).Count(&usersCount)
	db.Model(&model.Category{}).Count(&categoriesCount)
	db.Model(&model.Tag{}).Count(&tagsCount)
	db.Model(&model.Comment{}).Count(&commentsCount)

	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"posts":      postsCount,
			"users":      usersCount,
			"comments":   commentsCount,
			"categories": categoriesCount,
			"tags":       tagsCount,
		},
	})
}

// Get popular posts
func GetPopularPosts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	var posts []model.Post

	// First query all published articles
	db.Where("status = ?", "published").
		Preload("Author").
		Preload("Tags").
		Find(&posts)

	// Calculate likes count for each post
	for i := range posts {
		var count int64
		db.Model(&model.Like{}).Where("post_id = ?", posts[i].ID).Count(&count)
		posts[i].LikesCount = int(count)
	}

	// Sort by the sum of likes count multiplied by 5 plus view count
	// Use custom sorting because GORM doesn't support complex expression sorting
	for i := 0; i < len(posts); i++ {
		for j := i + 1; j < len(posts); j++ {
			scoreI := posts[i].LikesCount*5 + posts[i].ViewCount
			scoreJ := posts[j].LikesCount*5 + posts[j].ViewCount
			if scoreJ > scoreI {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}

	// Limit the number of returned items
	if limit < len(posts) {
		posts = posts[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
	})
}

// Get latest posts
func GetLatestPosts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	var posts []model.Post

	db.Where("status = ?", "published").
		Order("created_at DESC").
		Limit(limit).
		Preload("Author").
		Preload("Tags").
		Find(&posts)

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
	})
}

// Get category list
func GetCategories(c *gin.Context) {
	var categories []model.Category

	db.Find(&categories)

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
	})
}

// Create category
func CreateCategory(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	category := model.Category{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := db.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Category created successfully",
		"category": category,
	})
}

// Get tag list
func GetTags(c *gin.Context) {
	var tags []model.Tag

	db.Find(&tags)

	c.JSON(http.StatusOK, gin.H{
		"tags": tags,
	})
}
