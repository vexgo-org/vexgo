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
	ID        uint          `json:"id" gorm:"primaryKey"`
	PostID    uint          `json:"postId"`
	Post      Post          `json:"-" gorm:"foreignKey:PostID"`
	UserID    uint          `json:"userId"`
	User      User          `json:"author" gorm:"foreignKey:UserID"`
	Content   string        `json:"content" gorm:"type:text"`
	Status    CommentStatus `json:"status" gorm:"size:20;default:'published'"` // published, pending, rejected
	// ModerationReason records why the comment was rejected or held for
	// manual review (keyword hit, LLM verdict, or LLM failure).
	ModerationReason string `json:"moderationReason,omitempty" gorm:"size:500"`
	ParentID         *uint  `json:"parentId,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}
