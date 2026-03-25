package model

import (
	"time"
)

// Message message model
type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SenderID  uint      `json:"sender_id"` // Sender ID, 0 for system messages
	ReceiverID uint     `json:"receiver_id"` // Receiver ID
	Type      string    `json:"type"` // Message type: comment, like, reply, review, role
	Title     string    `json:"title"` // Message title
	Content   string    `json:"content"` // Message content
	RelatedID string    `json:"related_id"` // Related resource ID, such as post ID, comment ID
	RelatedType string  `json:"related_type"` // Related resource type: post, comment
	Status    string    `json:"status"` // Message status: sent, read
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies table name
func (Message) TableName() string {
	return "messages"
}

// Notification notification model
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"user_id"` // Receiving user ID
	Type      string    `json:"type"` // Notification type: comment, like, reply, review, role
	Title     string    `json:"title"` // Notification title
	Content   string    `json:"content"` // Notification content
	RelatedID string    `json:"related_id"` // Related resource ID
	RelatedType string  `json:"related_type"` // Related resource type
	IsRead    bool      `json:"is_read"` // Whether it has been read
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies table name
func (Notification) TableName() string {
	return "notifications"
}