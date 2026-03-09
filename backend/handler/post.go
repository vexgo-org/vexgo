// backend/handler/post.go
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 获取文章列表（支持分页、搜索、分类筛选）
func GetPosts(c *gin.Context) {
	var posts []model.Post

	// 1. 分页参数（增加错误处理，避免非法值）
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1 // 非法page值默认设为1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 10 // 限制limit范围，避免性能问题
	}

	// 2. 筛选条件
	categoryID := c.Query("category")
	status := c.Query("status")
	search := c.Query("search")

	// 获取当前用户角色和 ID（如果已登录）——支持多种存储类型
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
			// 尝试多种类型以获取 id
			if idf, ok := userMap["id"].(float64); ok {
				userID = uint(idf)
			} else if idu, ok := userMap["id"].(uint); ok {
				userID = idu
			} else if idi, ok := userMap["id"].(int); ok {
				userID = uint(idi)
			}
		}
	}

	// 构建查询
	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags")

	// 根据用户角色决定可见的文章
	if userRole == "" {
		// 未登录用户只能看到已发布的文章
		query = query.Where("status = ?", "published")
	} else if userRole == model.RoleGuest {
		// 访客角色只能看到已发布的文章
		query = query.Where("status = ?", "published")
	} else if userRole == model.RoleContributor {
		// 投稿者可以看到自己所有的文章（包括待审核、草稿等）和别人已发布的文章
		query = query.Where(
			db.Where("status = ?", "published").Or("author_id = ?", userID),
		)
	} else if userRole == model.RoleAuthor || userRole == model.RoleAdmin || userRole == model.RoleSuperAdmin {
		// 作者、管理员、超级管理员可以看到所有文章
		// 不需要添加额外的过滤条件
	} else {
		// 默认情况只显示已发布的文章
		query = query.Where("status = ?", "published")
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 分类筛选
	if categoryID != "" {
		// 尝试使用分类的 id 或名称来筛选文章
		// 首先尝试将 categoryID 转换为数字，作为分类的 id
		cid, err := strconv.Atoi(categoryID)
		if err == nil {
			// 如果转换成功，使用分类的 id 来筛选文章
			query = query.Where("category = ?", cid)
		} else {
			// 如果转换失败，使用分类的名称来筛选文章
			query = query.Where("category = ?", categoryID)
		}
	}

	// 搜索筛选
	if search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// 3. 统计总数+查询数据
	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts)

	// 为列表中的每篇文章填充点赞计数、浏览量和当前登录用户的点赞状态（如果有登录）
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

		// 填充评论计数
		var ccount int64
		db.Model(&model.Comment{}).Where("post_id = ?", posts[i].ID).Count(&ccount)
		posts[i].CommentsCount = int(ccount)

		// 对作者应用隐私过滤（如果不是管理员且不是查看自己的文章）
		if userRole != model.RoleAdmin && userRole != model.RoleSuperAdmin && posts[i].AuthorID != userID {
			FilterUserByPrivacy(&posts[i].Author, userID, userRole)
		}
	}

	// 4. 计算总页数
	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// 5. 返回响应
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

// 获取单篇文章
func GetPost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post

	// 添加日志来诊断问题
	fmt.Printf("Attempting to fetch post with ID: %s\n", id)

	if err := db.Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {
		fmt.Printf("Error fetching post with ID %s: %v\n", id, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在", "postId": id, "details": err.Error()})
		return
	}

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

	// 对作者应用隐私过滤（如果不是管理员且不是查看自己的文章）
	if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && post.AuthorID != currentUserID {
		FilterUserByPrivacy(&post.Author, currentUserID, currentUserRole)
	}

	// 增加浏览量（可选）
	db.Model(&post).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1))

	// 填充点赞计数和当前登录用户的点赞状态
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
	// 填充评论计数
	var ccount int64
	db.Model(&model.Comment{}).Where("post_id = ?", post.ID).Count(&ccount)
	post.CommentsCount = int(ccount)

	fmt.Printf("Successfully fetched post: %+v\n", post)
	c.JSON(http.StatusOK, gin.H{"post": post})
}

// 封装前端请求的数据结构，和 model.Post 不完全一致
// tags 使用字符串数组，status 可以由前端传入
// 覆盖字段按需赋值

type postRequest struct {
	Title      string      `json:"title" binding:"required"`
	Content    string      `json:"content" binding:"required"`
	Category   interface{} `json:"category" binding:"required"`
	Tags       []string    `json:"tags"`
	Excerpt    string      `json:"excerpt"`
	CoverImage string      `json:"coverImage"`
	Status     string      `json:"status"`
}

// helpers: take names and return slice of Tag models (create if missing)
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

// 创建文章（需登录）
func CreatePost(c *gin.Context) {
	userID, _ := c.Get("userID") // 从 JWT 中获取

	// 从上下文获取用户角色信息
	var userRole string
	if userContext, exists := c.Get("user"); exists {
		if userMap, ok := userContext.(map[string]interface{}); ok {
			if role, ok := userMap["role"].(string); ok {
				userRole = role
			}
		}
	}

	// 如果从上下文获取不到角色信息，则从数据库查询
	if userRole == "" {
		var user model.User
		if uid, ok := userID.(uint); ok {
			if err := db.First(&user, uid).Error; err == nil {
				userRole = user.Role
			}
		}
	}

	// 添加调试信息
	fmt.Printf("User ID: %+v\n", userID)
	fmt.Printf("User Role: %s\n", userRole)

	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// convert category to string no matter number or string
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

	// 根据用户角色确定文章初始状态
	initialStatus := req.Status
	fmt.Printf("Requested status: %s\n", initialStatus)
	if initialStatus == "" {
		// 投稿者需要审核
		if userRole == "contributor" {
			initialStatus = "pending"
			// 作者可以直接发布
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

	// 添加日志来诊断问题
	fmt.Printf("Creating post: %+v\n", post)
	fmt.Printf("Create result: %+v\n", result)
	fmt.Printf("Created post ID: %d\n", post.ID)

	c.JSON(http.StatusCreated, gin.H{"message": "文章创建成功", "post": post})
}

// 获取当前登录用户自己的文章（分页/状态）
func GetMyPosts(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)
	var posts []model.Post

	// 分页参数
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

	// 为每篇文章填充点赞计数、浏览量和评论计数
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

		// 填充评论计数
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

	// 获取当前用户角色
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

	// 根据用户角色决定可见的草稿文章
	if userRole != "" && (userRole == model.RoleAdmin || userRole == model.RoleSuperAdmin) {
		// 管理员和超级管理员可以看到所有草稿文章
		query = query.Where("status = ?", "draft")
	} else {
		// 其他用户只能看到自己的草稿文章
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

	// 为每篇文章填充点赞计数、浏览量和评论计数
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

		// 填充评论计数
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

// 获取指定用户的文章列表
func GetUserPosts(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	var posts []model.Post

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// 获取当前用户角色和ID（如果已登录）
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

	// 构建查询
	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("author_id = ?", userID)

	// 根据用户角色决定可见的文章
	if currentUserRole == "" || currentUserRole == model.RoleGuest {
		// 未登录用户或访客只能看到已发布的文章
		query = query.Where("status = ?", "published")
	} else if currentUserRole == model.RoleContributor {
		// 投稿者可以看到自己所有的文章和别人已发布的文章
		if uint(userID) != currentUserID {
			query = query.Where("status = ?", "published")
		}
	}
	// 作者、管理员、超级管理员可以看到所有文章

	var total int64
	query.Count(&total)

	query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts)

	// 为每篇文章填充点赞计数、浏览量和评论计数
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

		// 填充评论计数
		var ccount int64
		db.Model(&model.Comment{}).Where("post_id = ?", posts[i].ID).Count(&ccount)
		posts[i].CommentsCount = int(ccount)

		// 对作者应用隐私过滤
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && uint(userID) != currentUserID {
			// 如果不是管理员且不是查看自己的文章，则应用隐私过滤
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
