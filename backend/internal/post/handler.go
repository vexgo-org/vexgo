package post

import (
	"errors"
	"net/http"
	"strconv"

	"vexgo/backend/internal/middleware"
	"vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

func currentUser(c *gin.Context) (role string, id uint) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		return "", 0
	}
	return u.Role, u.ID
}

func (h *Handler) GetPosts(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	userRole, userID := currentUser(c)

	posts, total, err := h.svc.List(c.Request.Context(), userRole, userID, page, limit, c.Query("category"), c.Query("status"), c.Query("search"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch posts"})
		return
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

func (h *Handler) GetPost(c *gin.Context) {
	id := c.Param("id")
	userRole, userID := currentUser(c)

	post, err := h.svc.Get(c.Request.Context(), id, userRole, userID)
	if err != nil {
		if errors.Is(err, ErrGuestViewDenied) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You must be logged in to view this post"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist", "postId": id, "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": post})
}

func (h *Handler) CreatePost(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists || userIDVal == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，请先登录"})
		return
	}
	userID := userIDVal.(uint)
	userRole, _ := currentUser(c)

	var req struct {
		Title      string   `json:"title" binding:"required"`
		Content    string   `json:"content" binding:"required"`
		Category   any      `json:"category" binding:"required"`
		Tags       []string `json:"tags"`
		Excerpt    string   `json:"excerpt"`
		CoverImage string   `json:"coverImage"`
		Status     string   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post, err := h.svc.Create(c.Request.Context(), userRole, userID, CreateRequest{
		Title:      req.Title,
		Content:    req.Content,
		Category:   req.Category,
		Tags:       req.Tags,
		Excerpt:    req.Excerpt,
		CoverImage: req.CoverImage,
		Status:     model.PostStatus(req.Status),
	})
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，无法创建文章"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Post created successfully", "post": post})
}

func (h *Handler) UpdatePost(c *gin.Context) {
	id := c.Param("id")
	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(uint)

	var req struct {
		Title      string   `json:"title"`
		Content    string   `json:"content"`
		Category   any      `json:"category"`
		Tags       []string `json:"tags"`
		Excerpt    string   `json:"excerpt"`
		CoverImage string   `json:"coverImage"`
		Status     string   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post, err := h.svc.Update(c.Request.Context(), id, userID, UpdateRequest{
		Title:      req.Title,
		Content:    req.Content,
		Category:   req.Category,
		Tags:       req.Tags,
		Excerpt:    req.Excerpt,
		CoverImage: req.CoverImage,
		Status:     model.PostStatus(req.Status),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrPostNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to modify this post"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update post"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post updated successfully", "post": post})
}

func (h *Handler) DeletePost(c *gin.Context) {
	id := c.Param("id")
	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(uint)

	err := h.svc.Delete(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrPostNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this post"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete post"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

func (h *Handler) GetMyPosts(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID := userIDVal.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.DefaultQuery("status", "")

	posts, total, err := h.svc.MyPosts(c.Request.Context(), userID, page, limit, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch posts"})
		return
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

func (h *Handler) GetDraftPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	userRole, userID := currentUser(c)

	posts, total, err := h.svc.Drafts(c.Request.Context(), userRole, userID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch drafts"})
		return
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

func (h *Handler) GetUserPosts(c *gin.Context) {
	userIDStr := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	userRole, userID := currentUser(c)

	posts, total, err := h.svc.UserPosts(c.Request.Context(), userIDStr, userRole, userID, page, limit)
	if err != nil {
		if errors.Is(err, ErrBadRequest) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch posts"})
		return
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

func (h *Handler) GetPopularPosts(c *gin.Context) {
	userRole, _ := currentUser(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))

	posts, err := h.svc.Popular(c.Request.Context(), userRole, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch popular posts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

func (h *Handler) GetLatestPosts(c *gin.Context) {
	userRole, _ := currentUser(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))

	posts, err := h.svc.Latest(c.Request.Context(), userRole, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch latest posts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

func (h *Handler) GetCategories(c *gin.Context) {
	userRole, _ := currentUser(c)

	categories, err := h.svc.Categories(c.Request.Context(), userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func (h *Handler) CreateCategory(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	category, err := h.svc.CreateCategory(c.Request.Context(), req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Category created successfully",
		"category": category,
	})
}

func (h *Handler) GetTags(c *gin.Context) {
	userRole, _ := currentUser(c)

	tags, err := h.svc.Tags(c.Request.Context(), userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

func (h *Handler) CreateTag(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag, err := h.svc.CreateTag(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tag"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Tag created successfully",
		"tag":     tag,
	})
}

func (h *Handler) GetPendingPosts(c *gin.Context) {
	h.listModeration(c, model.PostStatusPending)
}

func (h *Handler) GetApprovedPosts(c *gin.Context) {
	h.listModeration(c, model.PostStatusPublished)
}

func (h *Handler) GetRejectedPosts(c *gin.Context) {
	h.listModeration(c, model.PostStatusRejected)
}

func (h *Handler) listModeration(c *gin.Context, status model.PostStatus) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.DefaultQuery("search", "")

	posts, total, err := h.svc.ListModeration(c.Request.Context(), status, page, limit, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch moderation posts"})
		return
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

func (h *Handler) ApprovePost(c *gin.Context) {
	post, err := h.svc.Approve(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post approved", "post": post})
}

func (h *Handler) RejectPost(c *gin.Context) {
	var req struct {
		RejectionReason string `json:"rejectionReason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	post, err := h.svc.Reject(c.Request.Context(), c.Param("id"), req.RejectionReason)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post has been rejected", "post": post})
}

func (h *Handler) ResubmitPost(c *gin.Context) {
	post, err := h.svc.Resubmit(c.Request.Context(), c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrPostNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
		case errors.Is(err, ErrBadRequest):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only rejected posts can be resubmitted for moderation"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resubmit post"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post resubmitted for moderation", "post": post})
}

func (h *Handler) ToggleLike(c *gin.Context) {
	postIDStr := c.Param("postId")
	id64, _ := strconv.ParseUint(postIDStr, 10, 64)
	postID := uint(id64)

	uid, _ := c.Get("userID")
	userID := uid.(uint)

	isLiked, count, err := h.svc.ToggleLike(c.Request.Context(), postID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove like"})
		return
	}

	if isLiked {
		c.JSON(http.StatusOK, gin.H{"message": "Liked successfully", "postId": postID, "isLiked": true, "likesCount": count})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Like removed", "postId": postID, "isLiked": false, "likesCount": count})
}

func (h *Handler) GetLikeStatus(c *gin.Context) {
	postIDStr := c.Param("postId")
	id64, _ := strconv.ParseUint(postIDStr, 10, 64)
	postID := uint(id64)

	var userID uint
	if uid, exists := c.Get("userID"); exists {
		userID = uid.(uint)
	}

	isLiked, count := h.svc.LikeStatus(c.Request.Context(), postID, userID)

	c.JSON(http.StatusOK, gin.H{
		"postId":     postID,
		"likesCount": count,
		"isLiked":    isLiked,
	})
}
