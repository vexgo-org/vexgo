package notification

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Handler exposes the notification domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates a notification HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// GetNotifications godoc
//
//	@Summary		List current user's notifications
//	@Description	Returns the authenticated user's notifications, paginated
//	@Description	and optionally filtered by type (comment, like, reply,
//	@Description	review, role) or read state.
//	@Tags			notifications
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page	query		int		false	"page number (1-based)"	default(1)
//	@Param			limit	query		int		false	"page size"				default(10)
//	@Param			type	query		string	false	"notification type filter"
//	@Param			is_read	query		string	false	"read state filter (true/false)"
//	@Success		200		{object}	NotificationListResponse
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/notifications [get]
func (h *Handler) GetNotifications(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	page, limit := middleware.ParsePagination(c, 10)

	notifications, total, err := h.svc.List(c.Request.Context(), ListQuery{
		UserID:           uid,
		Page:             page,
		Limit:            limit,
		NotificationType: model.NotificationType(c.Query("type")),
		IsRead:           c.Query("is_read"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch notifications"})
		return
	}

	c.JSON(http.StatusOK, NotificationListResponse{
		Notifications: notifications,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// MarkAsRead godoc
//
//	@Summary	Mark a single notification as read
//	@Tags		notifications
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		int	true	"notification id"
//	@Success	200	{object}	MessageResponse
//	@Failure	400	{object}	api.ErrorResponse	"invalid id"
//	@Failure	401	{object}	api.ErrorResponse
//	@Failure	404	{object}	api.ErrorResponse	"notification not found or not updated"
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/notifications/{id} [put]
func (h *Handler) MarkAsRead(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid notification ID"})
		return
	}

	rowsAffected, err := h.svc.MarkAsRead(c.Request.Context(), uid, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to mark notification as read"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Notification not found or not updated"})
		return
	}

	c.JSON(http.StatusOK, MessageResponse{Message: "Notification marked as read"})
}

// MarkAllAsRead godoc
//
//	@Summary	Mark all notifications as read
//	@Tags		notifications
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	MessageResponse
//	@Failure	401	{object}	api.ErrorResponse
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/notifications/read-all [post]
func (h *Handler) MarkAllAsRead(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	if err := h.svc.MarkAllAsRead(c.Request.Context(), uid); err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to mark all notifications as read"})
		return
	}

	c.JSON(http.StatusOK, MessageResponse{Message: "All notifications marked as read"})
}

// DeleteNotification godoc
//
//	@Summary	Delete a notification
//	@Tags		notifications
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		int	true	"notification id"
//	@Success	200	{object}	MessageResponse
//	@Failure	400	{object}	api.ErrorResponse	"invalid id"
//	@Failure	401	{object}	api.ErrorResponse
//	@Failure	404	{object}	api.ErrorResponse	"notification not found"
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/notifications/{id} [delete]
func (h *Handler) DeleteNotification(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid notification ID"})
		return
	}

	rowsAffected, err := h.svc.Delete(c.Request.Context(), uid, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to delete notification"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Notification not found or not deleted"})
		return
	}

	c.JSON(http.StatusOK, MessageResponse{Message: "Notification deleted"})
}

// GetUnreadCount godoc
//
//	@Summary		Unread notification count
//	@Description	Returns the number of unread notifications for the
//	@Description	authenticated user. Used by the frontend header to
//	@Description	render the badge.
//	@Tags			notifications
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	UnreadCountResponse
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/notifications/unread-count [get]
func (h *Handler) GetUnreadCount(c *gin.Context) {
	uid := middleware.CurrentUserID(c)

	count, err := h.svc.UnreadCount(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch unread count"})
		return
	}

	c.JSON(http.StatusOK, UnreadCountResponse{UnreadCount: count})
}
