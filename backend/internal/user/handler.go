package user

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

// Handler exposes the user domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates a user HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// GetUserList godoc
//
//	@Summary		List users (admin only)
//	@Description	Paginated list of users with optional free-text search
//	@Description	over username and email.
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int		false	"page number (1-based)"	default(1)
//	@Param			limit		query		int		false	"page size"				default(10)
//	@Param			search		query		string	false	"username/email filter"
//	@Success		200			{object}	UserListResponse
//	@Failure		401			{object}	api.ErrorResponse
//	@Failure		403			{object}	api.ErrorResponse
//	@Failure		500			{object}	api.ErrorResponse
//	@Router			/users [get]
func (h *Handler) GetUserList(c *gin.Context) {
	// Pagination parameters
	page, limit := middleware.ParsePagination(c, 10)

	search := c.DefaultQuery("search", "")

	users, total, err := h.svc.ListUsers(c.Request.Context(), search, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to query users"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, UserListResponse{
		Users: users,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: int64(totalPages),
		},
	})
}

// UpdateUserRole godoc
//
//	@Summary		Update a user's role
//	@Description	Admins promote/demote other users. Super admins can
//	@Description	also demote admins. Users cannot modify their own
//	@Description	role; super admins cannot be demoted.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"target user id"
//	@Param			request	body		UpdateUserRoleBody	true	"new role"
//	@Success		200		{object}	UserMessageResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid id, payload, or self-modification"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		403		{object}	api.ErrorResponse	"super admin protected / no permission"
//	@Failure		404		{object}	api.ErrorResponse	"user not found"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/users/{id}/role [put]
func (h *Handler) UpdateUserRole(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "No user information provided"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	var req UpdateUserRoleBody
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	user, err := h.svc.UpdateRole(c.Request.Context(), actor, uint(id), req.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "User does not exist"})
		case errors.Is(err, ErrCannotModifySelf):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrModifySuperAdmin):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrSuperAdminRestricted):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrAdminRoleRestricted):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrNoPermission):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update user role"})
		}
		return
	}

	c.JSON(http.StatusOK, UserMessageResponse{
		Message: "User role updated successfully",
		User:    user,
	})
}

// UpdateUserRoleBody is the body of PUT /api/users/{id}/role.
type UpdateUserRoleBody struct {
	Role string `json:"role" binding:"required" enums:"super_admin,admin,author,contributor,guest" example:"author"`
}

// DeleteUser godoc
//
//	@Summary		Delete a user
//	@Description	Admins can delete any non-super-admin user; super admins
//	@Description	can delete any user except themselves. All of the
//	@Description	deleted user's posts and comments are removed in the
//	@Description	same transaction.
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"target user id"
//	@Success		200	{object}	MessageResponse
//	@Failure		400	{object}	api.ErrorResponse	"self-deletion or invalid id"
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		403	{object}	api.ErrorResponse	"admin delete restricted or no permission"
//	@Failure		404	{object}	api.ErrorResponse	"user not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/users/{id} [delete]
func (h *Handler) DeleteUser(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "No user information provided"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	err = h.svc.DeleteUser(c.Request.Context(), actor, uint(id))
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "User does not exist"})
		case errors.Is(err, ErrCannotDeleteSelf):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrAdminDeleteRestricted):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrNoPermissionToDelete):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to delete user"})
		}
		return
	}

	c.JSON(http.StatusOK, MessageResponse{Message: "User deleted successfully"})
}

// ApplyForCreator godoc
//
//	@Summary		Apply for the creator role
//	@Description	Contributors and authors can apply to be promoted to
//	@Description	creator (a special author role with bulk publishing
//	@Description	permissions). The reason is shown to admins in the
//	@Description	review queue. One pending application per user at a time.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		ApplyForCreatorRequest	true	"application reason"
//	@Success		200		{object}	ApplyForCreatorResponse
//	@Failure		400		{object}	api.ErrorResponse	"role not eligible or pending application exists"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/users/apply-creator [post]
func (h *Handler) ApplyForCreator(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "No user information provided"})
		return
	}

	var req ApplyForCreatorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	applicationID, err := h.svc.ApplyForCreator(c.Request.Context(), actor, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrRoleNotEligible):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrAlreadyPending):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to create application"})
		}
		return
	}

	c.JSON(http.StatusOK, ApplyForCreatorResponse{
		Message:       "Application submitted successfully",
		ApplicationID: strconv.FormatUint(uint64(applicationID), 10),
	})
}

// GetCreatorApplications godoc
//
//	@Summary		List creator applications (admin only)
//	@Description	Returns the creator application queue, paginated and
//	@Description	optionally filtered by status (pending, approved, rejected).
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page	query		int		false	"page number (1-based)"	default(1)
//	@Param			limit	query		int		false	"page size"				default(10)
//	@Param			status	query		string	false	"status filter"	Enums(pending,approved,rejected)	default(pending)
//	@Success		200		{object}	CreatorApplicationListResponse
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		403		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/users/creator-applications [get]
func (h *Handler) GetCreatorApplications(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "No user information provided"})
		return
	}

	page, limit := middleware.ParsePagination(c, 10)
	statusStr := c.DefaultQuery("status", string(model.CreatorApplicationStatusPending))
	status := model.CreatorApplicationStatus(statusStr)

	applications, total, err := h.svc.ListCreatorApplications(c.Request.Context(), ListCreatorApplicationsQuery{
		ActorRole: actor.Role,
		Status:    status,
		Page:      page,
		Limit:     limit,
	})
	if err != nil {
		if errors.Is(err, ErrNoPermissionAccessApps) {
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to query creator applications"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, CreatorApplicationListResponse{
		Applications: applications,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: int64(totalPages),
		},
	})
}

// ReviewCreatorApplication godoc
//
//	@Summary		Approve or reject a creator application
//	@Description	Admins approve to grant the creator role; the optional
//	@Description	reason is forwarded to the applicant in the
//	@Description	notification.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int									true	"application id"
//	@Param			request	body		ReviewCreatorApplicationBody		true	"approve or reject"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid id, payload, or already processed"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		403		{object}	api.ErrorResponse	"no permission to review"
//	@Failure		404		{object}	api.ErrorResponse	"application not found"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/users/creator-applications/{id}/review [put]
func (h *Handler) ReviewCreatorApplication(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "No user information provided"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid application ID"})
		return
	}

	var req ReviewCreatorApplicationBody
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	err = h.svc.ReviewCreatorApplication(c.Request.Context(), ReviewCreatorApplicationRequest{
		Actor:  actor,
		AppID:  uint(id),
		Action: req.Action,
		Reason: req.Reason,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrNoPermissionReviewApps):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrApplicationNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Application does not exist"})
		case errors.Is(err, ErrApplicationProcessed):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrInvalidAction):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update application"})
		}
		return
	}

	c.JSON(http.StatusOK, MessageResponse{Message: "Application reviewed successfully"})
}
