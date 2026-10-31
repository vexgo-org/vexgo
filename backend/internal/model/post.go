// backend/model/post.go
package model

import (
	"time"
)

// PostStatus is the lifecycle state of a post.
type PostStatus string

const (
	PostStatusPending   PostStatus = "pending"
	PostStatusPublished PostStatus = "published"
	PostStatusDraft     PostStatus = "draft"
	PostStatusRejected  PostStatus = "rejected"
)

// ValidPostStatus reports whether status is a lifecycle state the database may
// store. It guards the author-facing create/update endpoints so a
// client-supplied value cannot introduce an unknown state. It says nothing
// about which roles may set it: `rejected` is valid in storage but only the
// moderation endpoints may assign it.
func ValidPostStatus(status PostStatus) bool {
	switch status {
	case PostStatusDraft, PostStatusPending, PostStatusPublished, PostStatusRejected:
		return true
	default:
		return false
	}
}

// Post is a blog article with its author, category, tags and moderation state.
type Post struct {
	ID              uint   `gorm:"primaryKey"`
	Slug            string `gorm:"size:255;uniqueIndex"`
	Title           string `binding:"required" gorm:"size:255"`
	Content         string `binding:"required" gorm:"type:text"`
	Excerpt         string `gorm:"type:text"`
	ViewCount       int    `gorm:"default:0"`
	AuthorID        uint
	Author          User       `gorm:"foreignKey:AuthorID"`
	Category        string     `gorm:"size:100"`
	Tags            []Tag      `gorm:"many2many:post_tags;"`
	Status          PostStatus `gorm:"size:50"`   // draft/published/pending/rejected
	RejectionReason string     `gorm:"type:text"` // rejection reason
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// Non-database field: used to include like count and whether the current user has liked in API response
	LikesCount int  `gorm:"-"`
	IsLiked    bool `gorm:"-"`
	// Non-database field: comment count
	CommentsCount int `gorm:"-"`

	CoverImageID *uint
	CoverImage   *Asset `gorm:"foreignKey:CoverImageID"`
}

// Like model
// Each record represents a user's like for a post

type Like struct {
	ID        uint `gorm:"primaryKey"`
	PostID    uint `gorm:"uniqueIndex:idx_likes_post_user"`
	Post      Post `gorm:"foreignKey:PostID"`
	UserID    uint `gorm:"uniqueIndex:idx_likes_post_user"`
	User      User `gorm:"foreignKey:UserID"`
	CreatedAt time.Time
}

// Tag is a label attached to posts via a many-to-many association.
type Tag struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:100;uniqueIndex"`
	// Non-database field: number of posts referencing the tag
	PostCount int64 `gorm:"-"`
}

// Category groups posts under a named, optionally described bucket.
type Category struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"size:100;uniqueIndex"`
	Description string
	// Non-database field: number of posts referencing the category by name
	PostCount int64 `gorm:"-"`
}
