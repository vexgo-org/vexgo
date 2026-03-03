// backend/handler/post.go
package handler

import (
	"blog-system/backend/model"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

	// 2. 筛选条件（关键修正）
	categoryID := c.Query("category") // 字段名改为categoryID，对应表的category_id
	status := c.Query("status")       // 去掉默认值，前端不传则不筛选所有状态
	search := c.Query("search")

	// 初始化查询：预加载关联，不默认过滤status
	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags")

	// 状态筛选：前端传了status才过滤，否则查所有
	if status != "" {
		query = query.Where("status = ?", status)
	}
	// 分类筛选：修正为category_id（核心！）
	if categoryID != "" {
		// 先转成数字，避免SQL注入/非法值
		cid, err := strconv.Atoi(categoryID)
		if err == nil {
			query = query.Where("category_id = ?", cid)
		}
	}
	// 搜索筛选（保留原有逻辑）
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

	// 4. 计算总页数（保留原有逻辑）
	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// 5. 返回响应（格式匹配前端）
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

	// 增加浏览量（可选）
	db.Model(&post).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1))

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
	post := model.Post{
		Title:      req.Title,
		Content:    req.Content,
		Category:   catStr,
		Excerpt:    req.Excerpt,
		CoverImage: req.CoverImage,
		Status:     req.Status,
	}
	if post.Status == "" {
		post.Status = "draft"
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

	query := db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", "draft")

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
