// Wire types for the notification domain. Swag reads the
// JSON tags to populate the OpenAPI spec; orval turns the
// generated schemas into TypeScript interfaces.
package notification

import (
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// NotificationResponse is one notification row. The receiving user id is
// implicit in the request (the list is always the caller's own), so it is not
// echoed back. Field names keep the snake_case the SPA already reads.
type NotificationResponse struct {
	ID            uint                          `json:"id" example:"11"`
	Type          model.NotificationType        `json:"type" example:"comment"`
	Title         string                        `json:"title" example:"New comment"`
	Content       string                        `json:"content"`
	RelatedID     uint                          `json:"related_id" example:"42"`
	RelatedType   model.NotificationRelatedType `json:"related_type" example:"post"`
	RelatedPostID *uint                         `json:"related_post_id" swaggertype:"primitive,integer" example:"42"`
	IsRead        bool                          `json:"is_read" example:"false"`
	CreatedAt     time.Time                     `json:"created_at"`
	UpdatedAt     time.Time                     `json:"updated_at"`
}

// NotificationListResponse is the body of GET
// /api/notifications. The `pagination` shape is the same as
// the rest of the REST surface.
type NotificationListResponse struct {
	Notifications []NotificationResponse `json:"notifications"`
	Pagination    api.Pagination         `json:"pagination"`
}

// UnreadCountResponse is the body of GET
// /api/notifications/unread-count. The number is the count
// of unread notifications for the authenticated user.
type UnreadCountResponse struct {
	UnreadCount int64 `json:"unreadCount" example:"3"`
}
