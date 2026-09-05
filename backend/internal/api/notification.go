package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// NotificationsResponse is the body of GET /api/notifications.
type NotificationsResponse struct {
	Notifications []model.Notification `json:"notifications" doc:"Notification rows"`
	Pagination    Pagination           `json:"pagination" doc:"Paging metadata"`
}

// UnreadCountResponse is the body of GET /api/notifications/unread-count.
type UnreadCountResponse struct {
	UnreadCount int64 `json:"unreadCount" doc:"Number of unread notifications"`
}
