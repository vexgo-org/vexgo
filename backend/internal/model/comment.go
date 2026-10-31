package model

import "time"

// CommentStatus is the moderation state of a comment.
type CommentStatus string

const (
	CommentStatusPending   CommentStatus = "pending"
	CommentStatusPublished CommentStatus = "published"
	CommentStatusRejected  CommentStatus = "rejected"
)

// Comment model
// Supports parent comments (parentId) for nesting
// Associated with User to return author information
// Associated with Post for cascading on statistics or deletion
// GORM automatically creates foreign key

type Comment struct {
	ID      uint `gorm:"primaryKey"`
	PostID  uint
	Post    Post `gorm:"foreignKey:PostID"`
	UserID  uint
	User    User          `gorm:"foreignKey:UserID"`
	Content string        `gorm:"type:text"`
	Status  CommentStatus `gorm:"size:20;default:'published'"` // published, pending, rejected
	// ModerationReason records why the comment was rejected or held for
	// manual review (keyword hit, LLM verdict, or LLM failure).
	ModerationReason string `gorm:"size:500"`
	ParentID         *uint
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
