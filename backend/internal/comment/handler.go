package comment

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// commentRateLimitPerMinute caps comment creation per client per minute so an
// authenticated account cannot flood a post's comment thread.
const commentRateLimitPerMinute = 30

// Handler exposes the comment domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
	// rateLimit stores the per-client request budget; nil keeps it in-process.
	rateLimit middleware.RateLimitStore
}

// NewHandler creates a comment HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{
		svc:       NewService(deps),
		mw:        middleware.NewAuth(deps.DB, deps.JWTSecret),
		rateLimit: deps.RateLimit,
	}
}

// GetComments godoc
//
//	@Summary		List comments for a post
//	@Description	Returns all published comments for the post. Pending
//	@Description	comments are only visible to the author and to admins
//	@Description	(the service layer filters by role).
//	@Tags			comments
//	@Produce		json
//	@Param			id	path		string	true	"post id or slug"
//	@Success		200	{object}	CommentListResponse
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/comments/post/{id} [get]
func (h *Handler) GetComments(c *gin.Context) {
	postID := c.Param("id")

	// Get current user information (for privacy filtering)
	u, _ := middleware.CurrentUser(c)
	currentUserID, currentUserRole := u.ID, u.Role

	comments, err := h.svc.ListByPost(c.Request.Context(), postID, currentUserID, currentUserRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch comments"})
		return
	}

	c.JSON(http.StatusOK, CommentListResponse{Comments: comments})
}

// CreateComment godoc
//
//	@Summary		Create a comment
//	@Description	Adds a comment to a post. Content is capped at 100
//	@Description	characters.
//	@Description	The reply is held for moderation when the
//	@Description	manual review queue is on, the keyword filter rejects
//	@Description	it, or the LLM filter rejects it.
//	@Tags			comments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CreateCommentRequest	true	"comment payload"
//	@Success		201		{object}	CreateCommentResponse
//	@Failure		400		{object}	api.ErrorResponse	"validation error or invalid postId"
//	@Failure		401		{object}	api.ErrorResponse	"not logged in"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/comments [post]
func (h *Handler) CreateComment(c *gin.Context) {
	var req CreateCommentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	// Check comment length limit (no more than 100 characters)
	if len(req.Content) > 100 {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Comment cannot exceed 100 characters"})
		return
	}

	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		// Reject unauthenticated request
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "Not logged in"})
		return
	}

	comment, count, err := h.svc.Create(c.Request.Context(), CreateRequest{
		PostID:   req.PostID,
		UserID:   userID,
		Content:  req.Content,
		ParentID: req.ParentID,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrPostNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Post does not exist"})
		case errors.Is(err, ErrParentCommentNotFound):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Parent comment does not exist on this post"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to create comment"})
		}
		return
	}

	c.JSON(http.StatusCreated, CreateCommentResponse{
		Message:            "Comment created successfully",
		Comment:            comment,
		CommentsCount:      count,
		RequiresModeration: comment.Status == model.CommentStatusPending,
	})
}

// DeleteComment godoc
//
//	@Summary		Delete a comment
//	@Description	The author or an admin may delete a comment.
//	@Tags			comments
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"comment id"
//	@Success		200	{object}	DeleteCommentResponse
//	@Failure		401	{object}	api.ErrorResponse	"not logged in"
//	@Failure		403	{object}	api.ErrorResponse	"not author or admin"
//	@Failure		404	{object}	api.ErrorResponse	"comment not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/comments/{id} [delete]
func (h *Handler) DeleteComment(c *gin.Context) {
	id := c.Param("id")

	// Get current operating user ID
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "Not logged in"})
		return
	}

	count, err := h.svc.Delete(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrCommentNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Comment does not exist"})
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "User does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Not authorized to delete this comment"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to delete comment"})
		}
		return
	}

	c.JSON(http.StatusOK, DeleteCommentResponse{Message: "Comment deleted", CommentsCount: count})
}

// GetCommentModerationConfig godoc
//
//	@Summary	Get comment moderation config
//	@Tags		comments
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	model.CommentModerationConfig
//	@Failure	401	{object}	api.ErrorResponse
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/moderation/comments/config [get]
func (h *Handler) GetCommentModerationConfig(c *gin.Context) {
	config, err := h.svc.GetModerationConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to get comment moderation configuration"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateCommentModerationConfig godoc
//
//	@Summary	Update comment moderation config
//	@Tags		comments
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		UpdateModerationConfigBody	true	"new config"
//	@Success	200		{object}	UpdateModerationConfigResponse
//	@Failure	400		{object}	api.ErrorResponse	"incomplete LLM config"
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/moderation/comments/config [put]
func (h *Handler) UpdateCommentModerationConfig(c *gin.Context) {
	var req UpdateModerationConfigBody

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	config, err := h.svc.UpdateModerationConfig(c.Request.Context(), UpdateModerationConfigRequest(req))
	if err != nil {
		if errors.Is(err, ErrLLMConfigIncomplete) {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
			return
		}
		// The wrapped error names the failing persistence step; log it here
		// so the generic client response does not lose the root cause.
		slog.Error("failed to update comment moderation configuration", "err", err)
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update comment moderation configuration"})
		return
	}

	c.JSON(http.StatusOK, UpdateModerationConfigResponse{
		Message: "Comment moderation configuration updated successfully",
		Config:  config,
	})
}

// TestModerationConfig godoc
//
//	@Summary		Test the LLM moderation endpoint
//	@Description	Issues a small test prompt against the configured LLM
//	@Description	to confirm the credentials and endpoint are wired up
//	@Description	correctly. The `response` field is whatever the model
//	@Description	replied with.
//	@Tags			comments
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	TestModerationResponse
//	@Failure		400	{object}	api.ErrorResponse	"incomplete LLM config"
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/moderation/comments/config/test [post]
func (h *Handler) TestModerationConfig(c *gin.Context) {
	result, err := h.svc.TestModerationLLM(c.Request.Context())
	if err != nil {
		if errors.Is(err, ErrLLMConfigIncomplete) {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
			return
		}
		// The raw error can carry network details and the upstream endpoint's
		// response body; log it server-side and keep the client response
		// generic.
		slog.Error("LLM moderation test failed", "err", err)
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to test LLM moderation endpoint"})
		return
	}

	c.JSON(http.StatusOK, TestModerationResponse{
		Message:  result.Message,
		Response: result.Response,
	})
}

// GetPendingComments godoc
//
//	@Summary	List pending comments (moderation queue)
//	@Tags		comments
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page	query		int	false	"page number (1-based)"	default(1)
//	@Param		limit	query		int	false	"page size"				default(10)
//	@Success	200		{object}	CommentModerationListResponse
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/moderation/comments/pending [get]
func (h *Handler) GetPendingComments(c *gin.Context) {
	h.listModeration(c, model.CommentStatusPending)
}

// GetApprovedComments godoc
//
//	@Summary	List approved comments
//	@Tags		comments
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page	query		int	false	"page number (1-based)"	default(1)
//	@Param		limit	query		int	false	"page size"				default(10)
//	@Success	200		{object}	CommentModerationListResponse
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/moderation/comments/approved [get]
func (h *Handler) GetApprovedComments(c *gin.Context) {
	h.listModeration(c, model.CommentStatusPublished)
}

// GetRejectedComments godoc
//
//	@Summary	List rejected comments
//	@Tags		comments
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page	query		int	false	"page number (1-based)"	default(1)
//	@Param		limit	query		int	false	"page size"				default(10)
//	@Success	200		{object}	CommentModerationListResponse
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/moderation/comments/rejected [get]
func (h *Handler) GetRejectedComments(c *gin.Context) {
	h.listModeration(c, model.CommentStatusRejected)
}

// listModeration renders the moderation queue for a given comment status.
func (h *Handler) listModeration(c *gin.Context, status model.CommentStatus) {
	page, _ := c.GetQuery("page")
	if page == "" {
		page = "1"
	}
	pageNum := 1
	if val, err := strconv.Atoi(page); err == nil && val > 0 {
		pageNum = val
	}

	limit, _ := c.GetQuery("limit")
	if limit == "" {
		limit = "10"
	}
	limitNum := 10
	if val, err := strconv.Atoi(limit); err == nil && val > 0 && val <= 100 {
		limitNum = val
	}

	comments, total, err := h.svc.ListModeration(c.Request.Context(), status, pageNum, limitNum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch moderation comments"})
		return
	}

	totalPages := (int(total) + limitNum - 1) / limitNum
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, CommentModerationListResponse{
		Comments: comments,
		Pagination: Pagination{
			Total:      total,
			Page:       pageNum,
			Limit:      limitNum,
			TotalPages: totalPages,
		},
	})
}

// ApproveComment godoc
//
//	@Summary		Approve a comment
//	@Description	Moves the comment from pending to published.
//	@Tags			comments
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"comment id"
//	@Success		200	{object}	CommentMessageResponse
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		404	{object}	api.ErrorResponse	"comment not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/moderation/comments/{id}/approve [put]
func (h *Handler) ApproveComment(c *gin.Context) {
	h.setStatus(c, model.CommentStatusPublished, "Comment approved", "Failed to approve comment")
}

// RejectComment godoc
//
//	@Summary		Reject a comment
//	@Description	Moves the comment from pending to rejected.
//	@Tags			comments
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"comment id"
//	@Success		200	{object}	CommentMessageResponse
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		404	{object}	api.ErrorResponse	"comment not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/moderation/comments/{id}/reject [put]
func (h *Handler) RejectComment(c *gin.Context) {
	h.setStatus(c, model.CommentStatusRejected, "Comment rejected", "Failed to reject comment")
}

// setStatus approves or rejects a comment and renders the result.
func (h *Handler) setStatus(c *gin.Context, status model.CommentStatus, successMsg, failureMsg string) {
	comment, err := h.svc.SetStatus(c.Request.Context(), c.Param("id"), status)
	if err != nil {
		if errors.Is(err, ErrCommentNotFound) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Comment does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: failureMsg})
		return
	}

	c.JSON(http.StatusOK, CommentMessageResponse{
		Message: successMsg,
		Comment: comment,
	})
}
