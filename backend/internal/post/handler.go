package post

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
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

// GetPosts godoc
//
//	@Summary		List posts
//	@Description	Returns the published post list, paginated and
//	@Description	optionally filtered by category or free-text search.
//	@Description	Anonymous callers see a reduced view (no
//	@Description	pending posts).
//	@Tags			posts
//	@Produce		json
//	@Param			page		query		int		false	"page number (1-based)"	default(1)
//	@Param			limit		query		int		false	"page size"				default(10)
//	@Param			category	query		string	false	"category id or slug filter"
//	@Param			search		query		string	false	"free-text filter"
//	@Success		200			{object}	PostListResponse
//	@Failure		403			{object}	api.ErrorResponse	"guest view denied"
//	@Failure		500			{object}	api.ErrorResponse
//	@Router			/posts [get]
func (h *Handler) GetPosts(c *gin.Context) {
	page, limit := middleware.ParsePagination(c, 10)

	u, _ := middleware.CurrentUser(c)
	userRole, userID := u.Role, u.ID

	posts, total, err := h.svc.List(c.Request.Context(), ListQuery{
		UserRole: userRole,
		UserID:   userID,
		Page:     page,
		Limit:    limit,
		Category: c.Query("category"),
		Search:   c.Query("search"),
	})
	if err != nil {
		if errors.Is(err, ErrGuestViewDenied) {
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "You must be logged in to view posts"})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch posts"})
		return
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, PostListResponse{
		Posts: posts,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	})
}

// GetPostByID godoc
//
//	@Summary		Look up a post by numeric id
//	@Description	Used by internal callers (notifications, moderation) that
//	@Description	need to resolve a post id to its slug.
//	@Tags			posts
//	@Produce		json
//	@Param			id	path		string	true	"numeric post id"
//	@Success		200	{object}	PostSingleResponse
//	@Failure		403	{object}	api.ErrorResponse			"guest view denied"
//	@Failure		404	{object}	api.NotFoundWithIDResponse	"post not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/posts/by-id/{id} [get]
func (h *Handler) GetPostByID(c *gin.Context) {
	id := c.Param("id")
	u, _ := middleware.CurrentUser(c)
	userRole, userID := u.Role, u.ID

	post, err := h.svc.Get(c.Request.Context(), id, userRole, userID)
	if err != nil {
		if errors.Is(err, ErrGuestViewDenied) {
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "You must be logged in to view this post"})
			return
		}
		c.JSON(http.StatusNotFound, api.NotFoundWithIDResponse{Error: "Post does not exist", PostID: id})
		return
	}

	c.JSON(http.StatusOK, PostSingleResponse{Post: post})
}

// GetPost godoc
//
//	@Summary	Look up a post by slug
//	@Tags		posts
//	@Produce	json
//	@Param		slug	path		string	true	"post slug"
//	@Success	200		{object}	PostSingleResponse
//	@Failure	403		{object}	api.ErrorResponse				"guest view denied"
//	@Failure	404		{object}	api.NotFoundWithSlugResponse	"post not found"
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/posts/{slug} [get]
func (h *Handler) GetPost(c *gin.Context) {
	slug := c.Param("slug")
	u, _ := middleware.CurrentUser(c)
	userRole, userID := u.Role, u.ID

	post, err := h.svc.GetBySlug(c.Request.Context(), slug, userRole, userID)
	if err != nil {
		if errors.Is(err, ErrGuestViewDenied) {
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "You must be logged in to view this post"})
			return
		}
		if errors.Is(err, ErrPostNotFound) {
			c.JSON(http.StatusNotFound, api.NotFoundWithSlugResponse{Error: "Post does not exist", Slug: slug})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to load post"})
		return
	}

	c.JSON(http.StatusOK, PostSingleResponse{Post: post})
}

// CreatePost godoc
//
//	@Summary		Create a post
//	@Description	Contributors and above can create posts. The `status`
//	@Description	field controls whether the post goes directly
//	@Description	to published, lands in pending for moderation, or
//	@Description	is saved as a draft.
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CreatePostRequest	true	"post payload"
//	@Success		201		{object}	PostMessageResponse
//	@Failure		400		{object}	api.ErrorResponse		"validation error / invalid slug"
//	@Failure		403		{object}	api.ErrorResponse		"insufficient permissions / not logged in"
//	@Failure		409		{object}	api.CodeErrorResponse	"slug already taken"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/posts [post]
func (h *Handler) CreatePost(c *gin.Context) {
	// Check if user is logged in
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Please log in first"})
		return
	}

	// Get user role information from context
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role

	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	post, err := h.svc.Create(c.Request.Context(), userRole, userID, CreateRequest{
		Slug:       req.Slug,
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
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Insufficient permissions to create a post"})
			return
		}
		if errors.Is(err, ErrInvalidStatus) {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid post status"})
			return
		}
		if errors.Is(err, model.ErrSlugTaken) {
			c.JSON(http.StatusConflict, api.CodeErrorResponse{Error: "Slug is already taken by another post", Code: "slug_taken"})
			return
		}
		if errors.Is(err, model.ErrEmptySlug) || errors.Is(err, model.ErrInvalidSlug) || errors.Is(err, model.ErrSlugTooLong) {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to create post"})
		return
	}

	c.JSON(http.StatusCreated, PostMessageResponse{Message: "Post created successfully", Post: post})
}

// UpdatePost godoc
//
//	@Summary		Update a post
//	@Description	Authors can update their own posts; admins can update
//	@Description	any post. Only the supplied fields are updated.
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string				true	"post id"
//	@Param			request	body		UpdatePostRequest	true	"updated post fields"
//	@Success		200		{object}	PostMessageResponse
//	@Failure		400		{object}	api.ErrorResponse		"invalid slug"
//	@Failure		403		{object}	api.ErrorResponse		"not author or admin"
//	@Failure		404		{object}	api.ErrorResponse		"post not found"
//	@Failure		409		{object}	api.CodeErrorResponse	"slug already taken"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/posts/{id} [put]
func (h *Handler) UpdatePost(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.CurrentUserID(c)

	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	post, err := h.svc.Update(c.Request.Context(), id, userID, UpdateRequest{
		Slug:       req.Slug,
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
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Post does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Not authorized to modify this post"})
		case errors.Is(err, ErrInvalidStatus):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid post status"})
		case errors.Is(err, model.ErrSlugTaken):
			c.JSON(http.StatusConflict, api.CodeErrorResponse{Error: "Slug is already taken by another post", Code: "slug_taken"})
		case errors.Is(err, model.ErrEmptySlug) || errors.Is(err, model.ErrInvalidSlug) || errors.Is(err, model.ErrSlugTooLong):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update post"})
		}
		return
	}

	c.JSON(http.StatusOK, PostMessageResponse{Message: "Post updated successfully", Post: post})
}

// DeletePost godoc
//
//	@Summary		Delete a post
//	@Description	Authors can delete their own posts; admins can delete
//	@Description	any post. All comments on the post are removed in
//	@Description	the same transaction.
//	@Tags			posts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"post id"
//	@Success		200	{object}	PostDeleteResponse
//	@Failure		403	{object}	api.ErrorResponse	"not author or admin"
//	@Failure		404	{object}	api.ErrorResponse	"post not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/posts/{id} [delete]
func (h *Handler) DeletePost(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.CurrentUserID(c)

	err := h.svc.Delete(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrPostNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Post does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Not authorized to delete this post"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to delete post"})
		}
		return
	}

	c.JSON(http.StatusOK, PostDeleteResponse{Message: "Post deleted successfully"})
}

// GetMyPosts godoc
//
//	@Summary		List the authenticated user's posts
//	@Description	All statuses (draft, pending, published, rejected) by
//	@Description	default; the `status` query param narrows to one.
//	@Tags			posts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page	query		int		false	"page number (1-based)"	default(1)
//	@Param			limit	query		int		false	"page size"				default(10)
//	@Param			status	query		string	false	"status filter"			Enums(draft,pending,published,rejected)
//	@Success		200		{object}	PostListResponse
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/posts/user/my-posts [get]
func (h *Handler) GetMyPosts(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	page, limit := middleware.ParsePagination(c, 10)
	status := c.DefaultQuery("status", "")

	posts, total, err := h.svc.MyPosts(c.Request.Context(), MyPostsQuery{
		UserID: userID,
		Page:   page,
		Limit:  limit,
		Status: status,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch posts"})
		return
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, PostListResponse{
		Posts: posts,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	})
}

// GetDraftPosts godoc
//
//	@Summary	List the authenticated user's drafts
//	@Tags		posts
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page	query		int	false	"page number (1-based)"	default(1)
//	@Param		limit	query		int	false	"page size"				default(10)
//	@Success	200		{object}	PostListResponse
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/posts/drafts [get]
func (h *Handler) GetDraftPosts(c *gin.Context) {
	page, limit := middleware.ParsePagination(c, 10)

	u, _ := middleware.CurrentUser(c)
	userRole, userID := u.Role, u.ID

	posts, total, err := h.svc.Drafts(c.Request.Context(), DraftsQuery{
		UserRole: userRole,
		UserID:   userID,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch drafts"})
		return
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, PostListResponse{
		Posts: posts,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	})
}

// GetUserPosts godoc
//
//	@Summary		List a specific user's posts
//	@Description	Public for published posts; pending and rejected posts
//	@Description	are only visible to the author and to admins.
//	@Tags			posts
//	@Produce		json
//	@Param			id		path		string	true	"author user id or username"
//	@Param			page	query		int		false	"page number (1-based)"	default(1)
//	@Param			limit	query		int		false	"page size"				default(10)
//	@Success		200		{object}	PostListResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid user id"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/posts/user/{id} [get]
func (h *Handler) GetUserPosts(c *gin.Context) {
	userIDStr := c.Param("id")
	page, limit := middleware.ParsePagination(c, 10)

	u, _ := middleware.CurrentUser(c)
	userRole, userID := u.Role, u.ID

	posts, total, err := h.svc.UserPosts(c.Request.Context(), UserPostsQuery{
		UserIDStr:       userIDStr,
		CurrentUserRole: userRole,
		CurrentUserID:   userID,
		Page:            page,
		Limit:           limit,
	})
	if err != nil {
		if errors.Is(err, ErrBadRequest) {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid user ID"})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch posts"})
		return
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, PostListResponse{
		Posts: posts,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	})
}

// GetPopularPosts godoc
//
//	@Summary		Popular posts
//	@Description	Top posts by view count, capped to the requested limit.
//	@Tags			posts
//	@Produce		json
//	@Param			limit	query		int	false	"max posts to return"	default(5)
//	@Success		200		{object}	PostListResponseData
//	@Failure		403		{object}	api.ErrorResponse	"guest view denied"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/stats/popular-posts [get]
func (h *Handler) GetPopularPosts(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role
	_, limit := middleware.ParsePagination(c, 5)

	posts, err := h.svc.Popular(c.Request.Context(), userRole, limit)
	if err != nil {
		if errors.Is(err, ErrGuestViewDenied) {
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "You must be logged in to view popular posts"})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch popular posts"})
		return
	}

	c.JSON(http.StatusOK, PostListResponseData{Posts: posts})
}

// GetLatestPosts godoc
//
//	@Summary		Latest posts
//	@Description	Most recently published posts, capped to the
//	@Description	requested limit.
//	@Tags			posts
//	@Produce		json
//	@Param			limit	query		int	false	"max posts to return"	default(5)
//	@Success		200		{object}	PostListResponseData
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/stats/latest-posts [get]
func (h *Handler) GetLatestPosts(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role
	_, limit := middleware.ParsePagination(c, 5)

	posts, err := h.svc.Latest(c.Request.Context(), userRole, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch latest posts"})
		return
	}

	c.JSON(http.StatusOK, PostListResponseData{Posts: posts})
}

// GetCategories godoc
//
//	@Summary	List categories
//	@Tags		categories
//	@Produce	json
//	@Success	200	{object}	CategoriesListResponse
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/categories [get]
func (h *Handler) GetCategories(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role

	categories, err := h.svc.Categories(c.Request.Context(), userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch categories"})
		return
	}

	c.JSON(http.StatusOK, CategoriesListResponse{Categories: categories})
}

// CreateCategory godoc
//
//	@Summary		Create a category
//	@Description	Contributors and above can create categories.
//	@Tags			categories
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CreateCategoryRequest	true	"category payload"
//	@Success		201		{object}	CreateCategoryResponse
//	@Failure		400		{object}	api.ErrorResponse		"name is blank"
//	@Failure		403		{object}	api.ErrorResponse		"insufficient permissions"
//	@Failure		409		{object}	api.CodeErrorResponse	"duplicate name"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/categories [post]
func (h *Handler) CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	u, _ := middleware.CurrentUser(c)
	category, err := h.svc.CreateCategory(c.Request.Context(), u.Role, req.Name, req.Description)
	if err != nil {
		switch {
		case errors.Is(err, ErrBadRequest):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Category name must not be blank"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Insufficient permissions to create a category"})
		case errors.Is(err, ErrDuplicateName):
			c.JSON(http.StatusConflict, api.CodeErrorResponse{Error: "A category with this name already exists", Code: "duplicate_name"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to create category"})
		}
		return
	}

	c.JSON(http.StatusCreated, CreateCategoryResponse{
		Message:  "Category created successfully",
		Category: category,
	})
}

// GetTags godoc
//
//	@Summary	List tags
//	@Tags		tags
//	@Produce	json
//	@Success	200	{object}	TagsListResponse
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/tags [get]
func (h *Handler) GetTags(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	userRole := u.Role

	tags, err := h.svc.Tags(c.Request.Context(), userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch tags"})
		return
	}

	c.JSON(http.StatusOK, TagsListResponse{Tags: tags})
}

// DeleteCategory godoc
//
//	@Summary		Delete an empty category
//	@Description	Returns 400 if the category is still referenced by posts.
//	@Tags			categories
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"category id"
//	@Success		200	{object}	DeleteMessageResponse
//	@Failure		400	{object}	api.ErrorResponse	"category still in use"
//	@Failure		403	{object}	api.ErrorResponse	"insufficient permissions"
//	@Failure		404	{object}	api.ErrorResponse	"category not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/categories/{id} [delete]
func (h *Handler) DeleteCategory(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Category does not exist"})
		return
	}

	u, _ := middleware.CurrentUser(c)
	err := h.svc.DeleteCategory(c.Request.Context(), u.Role, id)
	if err != nil {
		var inUse *InUseError
		switch {
		case errors.Is(err, ErrCategoryNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Category does not exist"})
		case errors.As(err, &inUse):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: inUseMessage("Category", inUse.Count)})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Insufficient permissions to delete a category"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to delete category"})
		}
		return
	}

	c.JSON(http.StatusOK, DeleteMessageResponse{Message: "Category deleted successfully"})
}

// DeleteTag godoc
//
//	@Summary		Delete an empty tag
//	@Description	Returns 400 if the tag is still referenced by posts.
//	@Tags			tags
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"tag id"
//	@Success		200	{object}	DeleteMessageResponse
//	@Failure		400	{object}	api.ErrorResponse	"tag still in use"
//	@Failure		403	{object}	api.ErrorResponse	"insufficient permissions"
//	@Failure		404	{object}	api.ErrorResponse	"tag not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/tags/{id} [delete]
func (h *Handler) DeleteTag(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Tag does not exist"})
		return
	}

	u, _ := middleware.CurrentUser(c)
	err := h.svc.DeleteTag(c.Request.Context(), u.Role, id)
	if err != nil {
		var inUse *InUseError
		switch {
		case errors.Is(err, ErrTagNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Tag does not exist"})
		case errors.As(err, &inUse):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: inUseMessage("Tag", inUse.Count)})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Insufficient permissions to delete a tag"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to delete tag"})
		}
		return
	}

	c.JSON(http.StatusOK, DeleteMessageResponse{Message: "Tag deleted successfully"})
}

// parseIDParam parses a numeric route :id, reporting whether it is valid.
// Zero, non-numeric and out-of-range values cannot identify a row and are
// treated as missing resources.
func parseIDParam(c *gin.Context) (uint, bool) {
	return parseUintParam(c, "id")
}

// parseUintParam parses a numeric route parameter by name, reporting whether it
// is valid. The bit size matches uint so the conversion can never truncate
// silently. Callers must reject an invalid value instead of proceeding with 0:
// otherwise a non-numeric path segment resolves to row 0, which can write
// orphaned records and surfaces as a 500 rather than a client error.
func parseUintParam(c *gin.Context, name string) (uint, bool) {
	id64, err := strconv.ParseUint(c.Param(name), 10, strconv.IntSize)
	if err != nil || id64 == 0 {
		return 0, false
	}
	return uint(id64), true
}

// inUseMessage renders the rejection message for a category or tag that
// posts still reference.
func inUseMessage(kind string, count int64) string {
	noun := "posts"
	if count == 1 {
		noun = "post"
	}
	return fmt.Sprintf("%s is used by %d %s", kind, count, noun)
}

// CreateTag godoc
//
//	@Summary		Create a tag
//	@Description	Contributors and above can create tags.
//	@Tags			tags
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CreateTagRequest	true	"tag payload"
//	@Success		201		{object}	CreateTagResponse
//	@Failure		400		{object}	api.ErrorResponse		"name is blank"
//	@Failure		403		{object}	api.ErrorResponse		"insufficient permissions"
//	@Failure		409		{object}	api.CodeErrorResponse	"duplicate name"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/tags [post]
func (h *Handler) CreateTag(c *gin.Context) {
	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	u, _ := middleware.CurrentUser(c)
	tag, err := h.svc.CreateTag(c.Request.Context(), u.Role, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, ErrBadRequest):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Tag name must not be blank"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Insufficient permissions to create a tag"})
		case errors.Is(err, ErrDuplicateName):
			c.JSON(http.StatusConflict, api.CodeErrorResponse{Error: "A tag with this name already exists", Code: "duplicate_name"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to create tag"})
		}
		return
	}

	c.JSON(http.StatusCreated, CreateTagResponse{Message: "Tag created successfully", Tag: tag})
}

// GetPendingPosts godoc
//
//	@Summary	List pending posts (moderation queue)
//	@Tags		posts
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page	query		int		false	"page number (1-based)"	default(1)
//	@Param		limit	query		int		false	"page size"				default(10)
//	@Param		search	query		string	false	"free-text filter"
//	@Success	200		{object}	PostListResponse
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	403		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/moderation/pending [get]
func (h *Handler) GetPendingPosts(c *gin.Context) {
	h.listModeration(c, model.PostStatusPending)
}

// GetApprovedPosts godoc
//
//	@Summary	List approved posts (moderation history)
//	@Tags		posts
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page	query		int		false	"page number (1-based)"	default(1)
//	@Param		limit	query		int		false	"page size"				default(10)
//	@Param		search	query		string	false	"free-text filter"
//	@Success	200		{object}	PostListResponse
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	403		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/moderation/approved [get]
func (h *Handler) GetApprovedPosts(c *gin.Context) {
	h.listModeration(c, model.PostStatusPublished)
}

// GetRejectedPosts godoc
//
//	@Summary	List rejected posts (moderation history)
//	@Tags		posts
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page	query		int		false	"page number (1-based)"	default(1)
//	@Param		limit	query		int		false	"page size"				default(10)
//	@Param		search	query		string	false	"free-text filter"
//	@Success	200		{object}	PostListResponse
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	403		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/moderation/rejected [get]
func (h *Handler) GetRejectedPosts(c *gin.Context) {
	h.listModeration(c, model.PostStatusRejected)
}

// listModeration renders the moderation queue for a given post status.
func (h *Handler) listModeration(c *gin.Context, status model.PostStatus) {
	page, limit := middleware.ParsePagination(c, 10)
	search := c.DefaultQuery("search", "")

	posts, total, err := h.svc.ListModeration(c.Request.Context(), ListModerationQuery{
		Status: status,
		Page:   page,
		Limit:  limit,
		Search: search,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch moderation posts"})
		return
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, PostListResponse{
		Posts: posts,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	})
}

// ApprovePost godoc
//
//	@Summary	Approve a pending post
//	@Tags		posts
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"post id"
//	@Success	200	{object}	PostMessageResponse
//	@Failure	401	{object}	api.ErrorResponse
//	@Failure	403	{object}	api.ErrorResponse
//	@Failure	404	{object}	api.ErrorResponse	"post not found"
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/moderation/approve/{id} [put]
func (h *Handler) ApprovePost(c *gin.Context) {
	post, err := h.svc.Approve(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Post does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to approve post"})
		return
	}

	c.JSON(http.StatusOK, PostMessageResponse{Message: "Post approved", Post: post})
}

// RejectPost godoc
//
//	@Summary		Reject a pending post
//	@Description	The optional rejectionReason is stored on the post and
//	@Description	shown to the author.
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string				true	"post id"
//	@Param			request	body		RejectPostRequest	true	"rejection reason"
//	@Success		200		{object}	PostMessageResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid request"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		403		{object}	api.ErrorResponse
//	@Failure		404		{object}	api.ErrorResponse	"post not found"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/moderation/reject/{id} [put]
func (h *Handler) RejectPost(c *gin.Context) {
	var req RejectPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request parameters"})
		return
	}

	post, err := h.svc.Reject(c.Request.Context(), c.Param("id"), req.RejectionReason)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Post does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to reject post"})
		return
	}

	c.JSON(http.StatusOK, PostMessageResponse{Message: "Post has been rejected", Post: post})
}

// ResubmitPost godoc
//
//	@Summary		Resubmit a rejected post
//	@Description	Authors can move a rejected post back into the
//	@Description	pending queue after editing. Only rejected posts
//	@Description	can be resubmitted.
//	@Tags			posts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"post id"
//	@Success		200	{object}	PostMessageResponse
//	@Failure		400	{object}	api.ErrorResponse	"post is not rejected"
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		403	{object}	api.ErrorResponse
//	@Failure		404	{object}	api.ErrorResponse	"post not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/moderation/resubmit/{id} [put]
func (h *Handler) ResubmitPost(c *gin.Context) {
	post, err := h.svc.Resubmit(c.Request.Context(), c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrPostNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Post does not exist"})
		case errors.Is(err, ErrBadRequest):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Only rejected posts can be resubmitted for moderation"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to resubmit post"})
		}
		return
	}

	c.JSON(http.StatusOK, PostMessageResponse{Message: "Post resubmitted for moderation", Post: post})
}

// ToggleLike godoc
//
//	@Summary		Like or unlike a post
//	@Description	Idempotent toggle: likes the post on the first call
//	@Description	from a given user, unlikes on the second. The
//	@Description	response includes the new like count.
//	@Tags			posts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			postId	path		int	true	"post id"
//	@Success		200		{object}	LikeResponse
//	@Failure		400		{object}	api.ErrorResponse
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/likes/{postId} [post]
func (h *Handler) ToggleLike(c *gin.Context) {
	postID, ok := parseUintParam(c, "postId")
	if !ok {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid post ID"})
		return
	}

	userID := middleware.CurrentUserID(c)

	isLiked, count, err := h.svc.ToggleLike(c.Request.Context(), postID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to remove like"})
		return
	}

	if isLiked {
		c.JSON(http.StatusOK, LikeResponse{Message: "Liked successfully", PostID: postID, IsLiked: true, LikesCount: count})
		return
	}
	c.JSON(http.StatusOK, LikeResponse{Message: "Like removed", PostID: postID, IsLiked: false, LikesCount: count})
}

// GetLikeStatus godoc
//
//	@Summary		Read the like status for a post
//	@Description	Public — anonymous callers see the count but always
//	@Description	get `isLiked: false`. Authenticated callers see their
//	@Description	own like state.
//	@Tags			posts
//	@Produce		json
//	@Param			postId	path		int	true	"post id"
//	@Success		200		{object}	LikeStatusResponse
//	@Failure		400		{object}	api.ErrorResponse
//	@Router			/likes/{postId} [get]
func (h *Handler) GetLikeStatus(c *gin.Context) {
	postID, ok := parseUintParam(c, "postId")
	if !ok {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid post ID"})
		return
	}

	userID := middleware.CurrentUserID(c)

	isLiked, count := h.svc.LikeStatus(c.Request.Context(), postID, userID)

	c.JSON(http.StatusOK, LikeStatusResponse{
		PostID:     postID,
		IsLiked:    isLiked,
		LikesCount: count,
	})
}
