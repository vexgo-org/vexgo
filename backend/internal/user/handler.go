package user

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
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

// GetUserList gets user list
func (h *Handler) GetUserList(c *gin.Context) {
	// Pagination parameters
	page, limit := middleware.ParsePagination(c, 10)

	search := c.DefaultQuery("search", "")

	users, total, err := h.svc.ListUsers(c.Request.Context(), search, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query users"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, api.UsersResponse{
		Users: users,
		Pagination: api.Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	})
}

// UpdateUserRole updates user role
func (h *Handler) UpdateUserRole(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No user information provided"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req api.UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	user, err := h.svc.UpdateRole(c.Request.Context(), actor, uint(id), req.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		case errors.Is(err, ErrCannotModifySelf):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrModifySuperAdmin):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrSuperAdminRestricted):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrAdminRoleRestricted):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrNoPermission):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user role"})
		}
		return
	}

	c.JSON(http.StatusOK, api.UserRoleUpdateResponse{
		Message: "User role updated successfully",
		User:    *user,
	})
}

// DeleteUser deletes user and all their posts and comments
func (h *Handler) DeleteUser(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No user information provided"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.svc.DeleteUser(c.Request.Context(), actor, uint(id))
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		case errors.Is(err, ErrCannotDeleteSelf):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrAdminDeleteRestricted):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrNoPermissionToDelete):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		}
		return
	}

	c.JSON(http.StatusOK, api.MessageResponse{Message: "User deleted successfully"})
}

// ApplyForCreator handles creator application submission
func (h *Handler) ApplyForCreator(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No user information provided"})
		return
	}

	var req api.ApplyForCreatorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	applicationID, err := h.svc.ApplyForCreator(c.Request.Context(), actor, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrRoleNotEligible):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrAlreadyPending):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create application"})
		}
		return
	}

	c.JSON(http.StatusOK, api.CreatorApplicationApplyResponse{
		Message:       "Application submitted successfully",
		ApplicationID: applicationID,
	})
}

// GetCreatorApplications gets creator applications for admin review
func (h *Handler) GetCreatorApplications(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No user information provided"})
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
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query creator applications"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}

	response := make([]api.CreatorApplicationView, 0, len(applications))
	for _, app := range applications {
		response = append(response, api.CreatorApplicationView{
			ID:          app.ID,
			UserID:      app.UserID,
			Username:    app.User.Username,
			Email:       app.User.Email,
			CurrentRole: app.User.Role,
			Status:      app.Status,
			Reason:      app.Reason,
			CreatedAt:   app.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:   app.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, api.CreatorApplicationsResponse{
		Applications: response,
		Pagination: api.Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	})
}

// ReviewCreatorApplication handles creator application review
func (h *Handler) ReviewCreatorApplication(c *gin.Context) {
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No user information provided"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	var req api.ReviewCreatorApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
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
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrApplicationNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Application does not exist"})
		case errors.Is(err, ErrApplicationProcessed):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidAction):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update application"})
		}
		return
	}

	c.JSON(http.StatusOK, api.MessageResponse{Message: "Application reviewed successfully"})
}
