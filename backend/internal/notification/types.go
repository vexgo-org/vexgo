// Wire types for the notification domain. Swag reads the
// JSON tags to populate the OpenAPI spec; orval turns the
// generated schemas into TypeScript interfaces.
package notification

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// NotificationListResponse is the body of GET
// /api/notifications. The `pagination` shape is the same as
// the rest of the REST surface; the `notifications` array
// contains model.Notification rows.
type NotificationListResponse struct {
	Notifications []model.Notification `json:"notifications"`
	Pagination    Pagination           `json:"pagination"`
}

// Pagination describes a paged result. It is the same shape
// used by the rest of the REST surface; declaring it here
// (rather than pulling it out into a shared api package)
// keeps each domain self-contained for swag.
type Pagination struct {
	Total      int64 `json:"total" example:"42"`
	Page       int   `json:"page" example:"1"`
	Limit      int   `json:"limit" example:"10"`
	TotalPages int64 `json:"totalPages" example:"5"`
}

// MessageResponse is a uniform `{ "message": "..." }` body
// for endpoints that don't return data.
type MessageResponse struct {
	Message string `json:"message" example:"Notification marked as read"`
}

// UnreadCountResponse is the body of GET
// /api/notifications/unread-count. The number is the count
// of unread notifications for the authenticated user.
type UnreadCountResponse struct {
	UnreadCount int64 `json:"unreadCount" example:"3"`
}
