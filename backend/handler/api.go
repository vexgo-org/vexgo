package handler

import (
	"net/http"
	"strconv"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// 获取统计信息
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

// 获取热门文章
func GetPopularPosts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	var posts []model.Post

	// 先查询所有已发布的文章
	db.Where("status = ?", "published").
		Preload("Author").
		Preload("Tags").
		Find(&posts)

	// 为每篇文章计算点赞数
	for i := range posts {
		var count int64
		db.Model(&model.Like{}).Where("post_id = ?", posts[i].ID).Count(&count)
		posts[i].LikesCount = int(count)
	}

	// 按照点赞数乘5加浏览量之和排序
	// 使用自定义排序，因为GORM不支持复杂的表达式排序
	for i := 0; i < len(posts); i++ {
		for j := i + 1; j < len(posts); j++ {
			scoreI := posts[i].LikesCount*5 + posts[i].ViewCount
			scoreJ := posts[j].LikesCount*5 + posts[j].ViewCount
			if scoreJ > scoreI {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}

	// 限制返回数量
	if limit < len(posts) {
		posts = posts[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
	})
}

// 获取最新文章
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

// 获取分类列表
func GetCategories(c *gin.Context) {
	var categories []model.Category

	db.Find(&categories)

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
	})
}

// 创建分类
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建分类失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "分类创建成功",
		"category": category,
	})
}

// 获取标签列表
func GetTags(c *gin.Context) {
	var tags []model.Tag

	db.Find(&tags)

	c.JSON(http.StatusOK, gin.H{
		"tags": tags,
	})
}
