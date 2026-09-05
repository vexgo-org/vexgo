// Package notification implements the notification domain.
package notification

import (
	"context"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Handler exposes the notification domain over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a notification HTTP handler with the given
// dependencies. The middleware field is kept for backward
// compatibility with the previous gin-based implementation; huma
// handlers read the user from the request context instead.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps)}
}

// getNotificationsInput carries the pagination + filter parameters
// for GET /api/notifications.
type getNotificationsInput struct {
	Page  int    `query:"page" default:"1" doc:"1-based page index"`
	Limit int    `query:"limit" default:"10" doc:"page size; capped at 100"`
	Type  string `query:"type" doc:"Filter by notification type"`
	IsRead string `query:"is_read" doc:"Filter by read state (\"true\"/\"false\")"`
}

type getNotificationsOutput struct {
	Body api.NotificationsResponse
}

type idPathInput struct {
	ID string `path:"id" doc:"Numeric notification ID"`
}

type markAsReadOutput struct {
	Body api.MessageResponse
}

type unreadCountOutput struct {
	Body api.UnreadCountResponse
}

// RegisterRoutes registers the notification domain operations on the
// given huma.API.
func (h *Handler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-notifications",
		Method:      http.MethodGet,
		Path:        "/notifications",
		Summary:     "List notifications for the current user",
		Tags:        []string{"notifications"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.GetNotifications)

	huma.Register(api, huma.Operation{
		OperationID: "get-unread-notification-count",
		Method:      http.MethodGet,
		Path:        "/notifications/unread-count",
		Summary:     "Get the current user's unread notification count",
		Tags:        []string{"notifications"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.GetUnreadCount)

	huma.Register(api, huma.Operation{
		OperationID: "mark-notification-read",
		Method:      http.MethodPut,
		Path:        "/notifications/{id}/read",
		Summary:     "Mark a notification as read",
		Tags:        []string{"notifications"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.MarkAsRead)

	huma.Register(api, huma.Operation{
		OperationID: "mark-all-notifications-read",
		Method:      http.MethodPut,
		Path:        "/notifications/read-all",
		Summary:     "Mark all notifications as read",
		Tags:        []string{"notifications"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.MarkAllAsRead)

	huma.Register(api, huma.Operation{
		OperationID: "delete-notification",
		Method:      http.MethodDelete,
		Path:        "/notifications/{id}",
		Summary:     "Delete a notification",
		Tags:        []string{"notifications"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.DeleteNotification)
}

// GetNotifications retrieves the notification list.
func (h *Handler) GetNotifications(ctx context.Context, in *getNotificationsInput) (*getNotificationsOutput, error) {
	uid := auth.UserIDFromContext(ctx)
	page, limit := middleware.ParsePaginationValues(
		itoaOr(in.Page, "1"),
		itoaOr(in.Limit, "10"),
		middleware.DefaultPaginationLimit,
	)

	notifications, total, err := h.svc.List(ctx, ListQuery{
		UserID:           uid,
		Page:             page,
		Limit:            limit,
		NotificationType: model.NotificationType(in.Type),
		IsRead:           in.IsRead,
	})
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch notifications")
	}
	totalPages := (total + int64(limit) - 1) / int64(limit)
	return &getNotificationsOutput{
		Body: api.NotificationsResponse{
			Notifications: notifications,
			Pagination: api.Pagination{
				Total:      total,
				Page:       page,
				Limit:      limit,
				TotalPages: int(totalPages),
			},
		},
	}, nil
}

// MarkAsRead marks a notification as read.
func (h *Handler) MarkAsRead(ctx context.Context, in *idPathInput) (*markAsReadOutput, error) {
	uid := auth.UserIDFromContext(ctx)
	id, err := strconv.Atoi(in.ID)
	if err != nil {
		return nil, huma.NewError(400, "Invalid notification ID")
	}
	rows, err := h.svc.MarkAsRead(ctx, uid, id)
	if err != nil {
		return nil, huma.NewError(500, "Failed to mark notification as read")
	}
	if rows == 0 {
		return nil, huma.NewError(404, "Notification not found or not updated")
	}
	return &markAsReadOutput{
		Body: api.MessageResponse{Message: "Notification marked as read"},
	}, nil
}

// MarkAllAsRead marks all notifications as read.
func (h *Handler) MarkAllAsRead(ctx context.Context, _ *struct{}) (*markAsReadOutput, error) {
	uid := auth.UserIDFromContext(ctx)
	if err := h.svc.MarkAllAsRead(ctx, uid); err != nil {
		return nil, huma.NewError(500, "Failed to mark all notifications as read")
	}
	return &markAsReadOutput{
		Body: api.MessageResponse{Message: "All notifications marked as read"},
	}, nil
}

// DeleteNotification deletes a notification.
func (h *Handler) DeleteNotification(ctx context.Context, in *idPathInput) (*markAsReadOutput, error) {
	uid := auth.UserIDFromContext(ctx)
	id, err := strconv.Atoi(in.ID)
	if err != nil {
		return nil, huma.NewError(400, "Invalid notification ID")
	}
	rows, err := h.svc.Delete(ctx, uid, id)
	if err != nil {
		return nil, huma.NewError(500, "Failed to delete notification")
	}
	if rows == 0 {
		return nil, huma.NewError(404, "Notification not found or not deleted")
	}
	return &markAsReadOutput{
		Body: api.MessageResponse{Message: "Notification deleted"},
	}, nil
}

// GetUnreadCount retrieves the number of unread notifications.
func (h *Handler) GetUnreadCount(ctx context.Context, _ *struct{}) (*unreadCountOutput, error) {
	uid := auth.UserIDFromContext(ctx)
	count, err := h.svc.UnreadCount(ctx, uid)
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch unread count")
	}
	return &unreadCountOutput{
		Body: api.UnreadCountResponse{UnreadCount: count},
	}, nil
}

func itoaOr(i int, def string) string {
	if i == 0 {
		return def
	}
	return strconv.Itoa(i)
}
