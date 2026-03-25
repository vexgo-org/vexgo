// backend/handler/post.go
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Get posts list (supports pagination, search, category filtering)
func GetPosts(c *gin.Context) {
	logrus.Debug("GetPosts: starting to fetch posts")
	var posts []model.Post

	// 1. Pagination parameters (add error handling to avoid invalid values)
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1 // invalid page value defaults to 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 10 // invalid limit value defaults to 10, max 100
	}
	logrus.Debugf("GetPosts: page=%d, limit=%d", page, limit)

	// 2. Filter conditions
	categoryID := c.Query("category")
	status := c.Query("status")
	search := c.Query("search")

	// Get current user role and ID (if logged in) - supports multiple storage types
	var userRole string
	var userID uint

	if uidVal, exists := c.Get("userID"); exists {
		switch v := uidVal.(type) {
		case uint:
			userID = v
		case int:
			userID = uint(v)
		case float64:
			userID = uint(v)
		}
	}

	if userContext, exists := c.Get("user"); exists {
		if userMap, ok := userContext.(map[string]interface{}); ok {
			if role, ok := userMap["role"].(string); ok {
				userRole = role
			}
			// Try multiple types to get id
			if idf, ok := userMap["id"].(float64); ok {
				userID = uint(idf)
			} else if idu, ok := userMap["id"].(uint); ok {
				userID = idu
			} else if idi, ok := userMap["id"].(int); ok {
				userID = uint(idi)
			}
		}
	}

	// Check if guest viewing is allowed
	var allowGuestView bool
	var config model.GeneralSettings
	if err := db.First(&config).Error; err != nil {
		// Default to true if config not found
		allowGuestView = true
	} else {
		allowGuestView = config.AllowGuestViewPosts
	}

	// If not logged in and guest viewing is not allowed, return empty result
	if userRole == "" && !allowGuestView {
		c.JSON(http.StatusOK, gin.H{
			"posts": posts,
			"pagination": gin.H{
				"total":      0,
				"page":       page,
				"limit":      limit,
				"totalPages": 1,
			},
		})
		return
	}

	// Build query
	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags")

	// Determine visible posts based on user role
	if userRole == "" {
		// Non-logged in users can only see published posts
		query = query.Where("status = ?", "published")
	} else if userRole == model.RoleGuest {
		// Guest role can only see published posts
		query = query.Where("status = ?", "published")
	} else if userRole == model.RoleContributor {
		// Contributors can see all their own posts (including pending, drafts, etc.) and others' published posts
		query = query.Where(
			db.Where("status = ?", "published").Or("author_id = ?", userID),
		)
	} else if userRole == model.RoleAuthor || userRole == model.RoleAdmin || userRole == model.RoleSuperAdmin {
		// Authors, admins, and super admins can see all posts
		// No additional filter conditions needed
	} else {
		// Default case: only show published posts
		query = query.Where("status = ?", "published")
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Category filter
	if categoryID != "" {
		// Try to filter posts by category id or name
		// First try to convert categoryID to number as category id
		cid, err := strconv.Atoi(categoryID)
		if err == nil {
			// If conversion successful, use category id to filter posts
			query = query.Where("category = ?", cid)
		} else {
			// If conversion failed, use category name to filter posts
			query = query.Where("category = ?", categoryID)
		}
	}

	// Search filter
	if search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// 3. Count total + query data
	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts)

	// Fill likes count, view count, and current logged-in user's like status for each post in the list (if logged in)
	for i := range posts {
		var count int64
		db.Model(&model.Like{}).Where("post_id = ?", posts[i].ID).Count(&count)
		posts[i].LikesCount = int(count)
		posts[i].IsLiked = false
		if userID != 0 {
			var like model.Like
			if db.Where("post_id = ? AND user_id = ?", posts[i].ID, userID).First(&like).Error == nil {
				posts[i].IsLiked = true
			}
		}

		// Fill comments count
		var ccount int64
		db.Model(&model.Comment{}).Where("post_id = ?", posts[i].ID).Count(&ccount)
		posts[i].CommentsCount = int(ccount)

		// Apply privacy filtering to author (if not admin and not viewing own post)
		if userRole != model.RoleAdmin && userRole != model.RoleSuperAdmin && posts[i].AuthorID != userID {
			FilterUserByPrivacy(&posts[i].Author, userID, userRole)
		}
	}

	// 4. Calculate total pages
	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// 5. Return response
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

// Get single post
func GetPost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post

	// Add logs for debugging
	fmt.Printf("Attempting to fetch post with ID: %s\n", id)

	if err := db.Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {
		fmt.Printf("Error fetching post with ID %s: %v\n", id, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist", "postId": id, "details": err.Error()})
		return
	}

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

	// Check if guest viewing is allowed
	var allowGuestView bool
	var config model.GeneralSettings
	if err := db.First(&config).Error; err != nil {
		// Default to true if config not found
		allowGuestView = true
	} else {
		allowGuestView = config.AllowGuestViewPosts
	}

	// If not logged in and guest viewing is not allowed, return 403
	if currentUserRole == "" && !allowGuestView {
		c.JSON(http.StatusForbidden, gin.H{"error": "You must be logged in to view this post"})
		return
	}

	// Apply privacy filtering to author (if not admin and not viewing own post)
	if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && post.AuthorID != currentUserID {
		FilterUserByPrivacy(&post.Author, currentUserID, currentUserRole)
	}

	// Increment view count (optional)
	db.Model(&post).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1))

	// Fill likes count and current logged-in user's like status
	var count int64
	db.Model(&model.Like{}).Where("post_id = ?", post.ID).Count(&count)
	post.LikesCount = int(count)
	post.IsLiked = false
	if uid, exists := c.Get("userID"); exists {
		switch v := uid.(type) {
		case uint:
			var like model.Like
			if db.Where("post_id = ? AND user_id = ?", post.ID, v).First(&like).Error == nil {
				post.IsLiked = true
			}
		case int:
			var like model.Like
			if db.Where("post_id = ? AND user_id = ?", post.ID, uint(v)).First(&like).Error == nil {
				post.IsLiked = true
			}
		case float64:
			var like model.Like
			if db.Where("post_id = ? AND user_id = ?", post.ID, uint(v)).First(&like).Error == nil {
				post.IsLiked = true
			}
		}
	}
	// Fill comments count
	var ccount int64
	db.Model(&model.Comment{}).Where("post_id = ?", post.ID).Count(&ccount)
	post.CommentsCount = int(ccount)

	fmt.Printf("Successfully fetched post ID: %d, Title: %s, Author: %s (ID: %d)\n", post.ID, post.Title, post.Author.Username, post.AuthorID)
	c.JSON(http.StatusOK, gin.H{"post": post})
}

// Wrapper struct for frontend request data, not exactly same as model.Post
// tags uses string array, status can be provided by frontend
// Override fields are assigned as needed

type postRequest struct {
	Title      string      `json:"title" binding:"required"`
	Content    string      `json:"content" binding:"required"`
	Category   interface{} `json:"category" binding:"required"`
	Tags       []string    `json:"tags"`
	Excerpt    string      `json:"excerpt"`
	CoverImage string      `json:"coverImage"`
	Status     string      `json:"status"`
}

// Helper: take names and return slice of Tag models (create if missing)
func resolveTags(names []string) ([]model.Tag, error) {
	var tags []model.Tag
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var tag model.Tag
		if err := db.Where("name = ?", name).First(&tag).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				tag = model.Tag{Name: name}
				db.Create(&tag)
			} else {
				return nil, err
			}
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// Create post (requires login)
func CreatePost(c *gin.Context) {
	// Check if user is logged in
	userID, exists := c.Get("userID")
	if !exists || userID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，请先登录"})
		return
	}

	// Get user role information from context
	var userRole string
	if userContext, exists := c.Get("user"); exists {
		if userMap, ok := userContext.(map[string]interface{}); ok {
			if role, ok := userMap["role"].(string); ok {
				userRole = role
			}
		}
	}

	// If role info not available from context, query from database
	if userRole == "" {
		var user model.User
		if uid, ok := userID.(uint); ok {
			if err := db.First(&user, uid).Error; err == nil {
				userRole = user.Role
			}
		}
	}

	// Check if user has permission to create posts
	if userRole == "" || userRole == model.RoleGuest {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，无法创建文章"})
		return
	}

	// Add debug information
	fmt.Printf("User ID: %+v\n", userID)
	fmt.Printf("User Role: %s\n", userRole)

	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert category to string regardless of number or string type
	var catStr string
	switch v := req.Category.(type) {
	case string:
		catStr = v
	case float64:
		catStr = strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		catStr = strconv.Itoa(v)
	case int64:
		catStr = strconv.FormatInt(v, 10)
	default:
		catStr = fmt.Sprintf("%v", v)
	}

	// Determine initial post status based on user role
	initialStatus := req.Status
	fmt.Printf("Requested status: %s\n", initialStatus)
	if initialStatus == "" {
		// Contributors need moderation
		if userRole == "contributor" {
			initialStatus = "pending"
			// Authors can publish directly
		} else if userRole == "author" || userRole == "admin" || userRole == "super_admin" {
			initialStatus = "published"
		} else {
			initialStatus = "draft"
		}
		fmt.Printf("Setting status based on role %s: %s\n", userRole, initialStatus)
	}

	post := model.Post{
		Title:      req.Title,
		Content:    req.Content,
		Category:   catStr,
		Excerpt:    req.Excerpt,
		CoverImage: req.CoverImage,
		Status:     initialStatus,
	}

	if uid, ok := userID.(uint); ok {
		post.AuthorID = uid
	}

	if len(req.Tags) > 0 {
		tags, err := resolveTags(req.Tags)
		if err == nil {
			post.Tags = tags
		}
	}

	result := db.Create(&post)

	// Add logs for debugging
	fmt.Printf("Creating post: Title: %s, AuthorID: %d\n", post.Title, post.AuthorID)
	fmt.Printf("Create result: %+v\n", result)
	fmt.Printf("Created post ID: %d\n", post.ID)

	c.JSON(http.StatusCreated, gin.H{"message": "Post created successfully", "post": post})
}

// Get current logged-in user's own posts (pagination/status)
func GetMyPosts(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)
	var posts []model.Post

	// Pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.DefaultQuery("status", "")

	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("author_id = ?", uid)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts)

	// Fill likes count, view count, and comments count for each post
	for i := range posts {
		var count int64
		db.Model(&model.Like{}).Where("post_id = ?", posts[i].ID).Count(&count)
		posts[i].LikesCount = int(count)
		posts[i].IsLiked = false
		if userID != 0 {
			var like model.Like
			if db.Where("post_id = ? AND user_id = ?", posts[i].ID, userID).First(&like).Error == nil {
				posts[i].IsLiked = true
			}
		}

		// Fill comments count
		var ccount int64
		db.Model(&model.Comment{}).Where("post_id = ?", posts[i].ID).Count(&ccount)
		posts[i].CommentsCount = int(ccount)
	}

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

// Get all draft articles for admin
func GetDraftPosts(c *gin.Context) {
	var posts []model.Post

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// Get current user role
	var userRole string
	var userID uint
	if userContext, exists := c.Get("user"); exists {
		if userMap, ok := userContext.(map[string]interface{}); ok {
			if role, ok := userMap["role"].(string); ok {
				userRole = role
			}
			if id, ok := userMap["id"].(float64); ok {
				userID = uint(id)
			}
		}
	}

	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags")

	// Determine visible draft posts based on user role
	if userRole != "" && (userRole == model.RoleAdmin || userRole == model.RoleSuperAdmin) {
		// Admins and super admins can see all draft posts
		query = query.Where("status = ?", "draft")
	} else {
		// Other users can only see their own draft posts
		userIDFromContext, _ := c.Get("userID")
		if uid, ok := userIDFromContext.(uint); ok {
			userID = uid
		}
		query = query.Where("author_id = ? AND status = ?", userID, "draft")
	}

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts)

	// Fill likes count, view count, and comments count for each post
	for i := range posts {
		var count int64
		db.Model(&model.Like{}).Where("post_id = ?", posts[i].ID).Count(&count)
		posts[i].LikesCount = int(count)
		posts[i].IsLiked = false
		if userID != 0 {
			var like model.Like
			if db.Where("post_id = ? AND user_id = ?", posts[i].ID, userID).First(&like).Error == nil {
				posts[i].IsLiked = true
			}
		}

		// Fill comments count
		var ccount int64
		db.Model(&model.Comment{}).Where("post_id = ?", posts[i].ID).Count(&ccount)
		posts[i].CommentsCount = int(ccount)
	}

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

// Get posts by specific user
func GetUserPosts(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var posts []model.Post

	// Pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// Get current user role and ID (if logged in)
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

	// Check if guest viewing is allowed
	var allowGuestView bool
	var config model.GeneralSettings
	if err := db.First(&config).Error; err != nil {
		// Default to true if config not found
		allowGuestView = true
	} else {
		allowGuestView = config.AllowGuestViewPosts
	}

	// If not logged in and guest viewing is not allowed, return empty result
	if currentUserRole == "" && !allowGuestView {
		c.JSON(http.StatusOK, gin.H{
			"posts": posts,
			"pagination": gin.H{
				"total":      0,
				"page":       page,
				"limit":      limit,
				"totalPages": 1,
			},
		})
		return
	}

	// Build query
	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("author_id = ?", userID)

	// Determine visible posts based on user role
	if currentUserRole == "" || currentUserRole == model.RoleGuest {
		// Non-logged in users or guests can only see published posts
		query = query.Where("status = ?", "published")
	} else if currentUserRole == model.RoleContributor {
		// Contributors can see all their own posts and others' published posts
		if uint(userID) != currentUserID {
			query = query.Where("status = ?", "published")
		}
	}
	// Authors, admins, and super admins can see all posts

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts)

	// Fill likes count, view count, and comments count for each post
	for i := range posts {
		var count int64
		db.Model(&model.Like{}).Where("post_id = ?", posts[i].ID).Count(&count)
		posts[i].LikesCount = int(count)
		posts[i].IsLiked = false
		if currentUserID != 0 {
			var like model.Like
			if db.Where("post_id = ? AND user_id = ?", posts[i].ID, currentUserID).First(&like).Error == nil {
				posts[i].IsLiked = true
			}
		}

		// Fill comments count
		var ccount int64
		db.Model(&model.Comment{}).Where("post_id = ?", posts[i].ID).Count(&ccount)
		posts[i].CommentsCount = int(ccount)

		// Apply privacy filtering to author
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && uint(userID) != currentUserID {
			// If not admin and not viewing own post, apply privacy filtering
			FilterUserByPrivacy(&posts[i].Author, currentUserID, currentUserRole)
		}
	}

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
