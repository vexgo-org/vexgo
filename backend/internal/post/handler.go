package post

import (
	"errors"
	"net/http"
	"strconv"

	"vexgo/backend/internal/middleware"
	"vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// Handler exposes the post domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates a post HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// GetPosts returns the post list.
func (h *Handler) GetPosts(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	u, _ := middleware.CurrentUser(c)
	userRole, userID := u.Role, u.ID

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

// GetPost returns a single post.
func (h *Handler) GetPost(c *gin.Context) {
	id := c.Param("id")
	u, _ := middleware.CurrentUser(c)
	userRole, userID := u.Role, u.ID

	post, err := h.svc.Get(c.Request.Context(), id, userRole, userID)
	if err != nil {
		if errors.Is(err, ErrGuestViewDenied) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You must be logged in to view this post"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist", "postId": id})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": post})
}

// CreatePost creates a post.
func (h *Handler) CreatePost(c *gin.Context) {
	// Check if user is logged in
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Please log in first"})
		return
	}

	// Get user role information from context
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role

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
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to create a post"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Post created successfully", "post": post})
}

// UpdatePost updates a post (author or admin only).
func (h *Handler) UpdatePost(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.CurrentUserID(c)

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

// DeletePost deletes a post (author or admin only).
func (h *Handler) DeletePost(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.CurrentUserID(c)

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

// GetMyPosts returns the current user's own posts.
func (h *Handler) GetMyPosts(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

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

// GetDraftPosts returns draft posts.
func (h *Handler) GetDraftPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	u, _ := middleware.CurrentUser(c)
	userRole, userID := u.Role, u.ID

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

// GetUserPosts returns the posts of a specific user.
func (h *Handler) GetUserPosts(c *gin.Context) {
	userIDStr := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	u, _ := middleware.CurrentUser(c)
	userRole, userID := u.Role, u.ID

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

// GetPopularPosts returns popular posts.
func (h *Handler) GetPopularPosts(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))

	posts, err := h.svc.Popular(c.Request.Context(), userRole, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch popular posts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// GetLatestPosts returns the latest posts.
func (h *Handler) GetLatestPosts(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))

	posts, err := h.svc.Latest(c.Request.Context(), userRole, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch latest posts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// GetCategories returns the category list.
func (h *Handler) GetCategories(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role

	categories, err := h.svc.Categories(c.Request.Context(), userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// CreateCategory creates a category.
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

// GetTags returns the tag list.
func (h *Handler) GetTags(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role

	tags, err := h.svc.Tags(c.Request.Context(), userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// CreateTag creates a tag.
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

// GetPendingPosts gets pending posts for moderation.
func (h *Handler) GetPendingPosts(c *gin.Context) {
	h.listModeration(c, model.PostStatusPending)
}

// GetApprovedPosts gets approved posts list.
func (h *Handler) GetApprovedPosts(c *gin.Context) {
	h.listModeration(c, model.PostStatusPublished)
}

// GetRejectedPosts gets rejected posts list.
func (h *Handler) GetRejectedPosts(c *gin.Context) {
	h.listModeration(c, model.PostStatusRejected)
}

// listModeration renders the moderation queue for a given post status.
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

// ApprovePost approves a post.
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

// RejectPost rejects a post.
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

// ResubmitPost resubmits a rejected post for moderation.
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

// ToggleLike likes or unlikes a post.
func (h *Handler) ToggleLike(c *gin.Context) {
	postIDStr := c.Param("postId")
	id64, _ := strconv.ParseUint(postIDStr, 10, 64)
	postID := uint(id64)

	userID := middleware.CurrentUserID(c)

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

// GetLikeStatus returns the like status of a post (public, optional login).
func (h *Handler) GetLikeStatus(c *gin.Context) {
	postIDStr := c.Param("postId")
	id64, _ := strconv.ParseUint(postIDStr, 10, 64)
	postID := uint(id64)

	userID := middleware.CurrentUserID(c)

	isLiked, count := h.svc.LikeStatus(c.Request.Context(), postID, userID)

	c.JSON(http.StatusOK, gin.H{
		"postId":     postID,
		"likesCount": count,
		"isLiked":    isLiked,
	})
}
