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
	ID            uint                    `gorm:"primaryKey"`
	UserID        uint                    // Receiving user ID
	Type          NotificationType        // Notification type: comment, like, reply, review, role
	Title         string                  // Notification title
	Content       string                  // Notification content
	RelatedID     uint                    // Related resource ID
	RelatedType   NotificationRelatedType // Related resource type
	RelatedPostID *uint                   // Owning post ID for reply/comment notifications
	IsRead        bool                    // Whether it has been read
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TableName specifies table name
func (Notification) TableName() string {
	return "notifications"
}
