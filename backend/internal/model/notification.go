package model

import (
	"time"
)

type (
	NotificationType        string
	NotificationRelatedType string
)

const (
	NotificationTypeRole    NotificationType = "role"
	NotificationTypeComment NotificationType = "comment"
	NotificationTypeLike    NotificationType = "like"
	NotificationTypeReply   NotificationType = "reply"
	NotificationTypeReview  NotificationType = "review"
)

const (
	NotificationRelatedTypePost               NotificationRelatedType = "post"
	NotificationRelatedTypeComment            NotificationRelatedType = "comment"
	NotificationRelatedTypeCreatorApplication NotificationRelatedType = "creator_application"
)

// Notification notification model
type Notification struct {
	ID          uint                    `gorm:"primaryKey" json:"id"`
	UserID      uint                    `json:"user_id"`      // Receiving user ID
	Type        NotificationType        `json:"type"`         // Notification type: comment, like, reply, review, role
	Title       string                  `json:"title"`        // Notification title
	Content     string                  `json:"content"`      // Notification content
	RelatedID   string                  `json:"related_id"`   // Related resource ID
	RelatedType NotificationRelatedType `json:"related_type"` // Related resource type
	IsRead      bool                    `json:"is_read"`      // Whether it has been read
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

// TableName specifies table name
func (Notification) TableName() string {
	return "notifications"
}
