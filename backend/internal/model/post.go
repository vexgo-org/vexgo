// backend/model/post.go
package model

import (
	"encoding/json"
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

// Post is a blog article with its author, category, tags and moderation state.
type Post struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	Slug            string     `json:"slug" gorm:"size:255;uniqueIndex"`
	Title           string     `json:"title" binding:"required" gorm:"size:255"`
	Content         string     `json:"content" binding:"required" gorm:"type:text"`
	Excerpt         string     `json:"excerpt" gorm:"type:text"`
	CoverImage      string     `json:"coverImage" gorm:"size:500"`
	ViewCount       int        `json:"viewCount" gorm:"default:0"`
	AuthorID        uint       `json:"authorId"`
	Author          User       `json:"author" gorm:"foreignKey:AuthorID"`
	Category        string     `json:"category" gorm:"size:100"`
	Tags            []Tag      `json:"tags" gorm:"many2many:post_tags;"`
	Status          PostStatus `json:"status" gorm:"size:50"`            // draft/published/pending/rejected
	RejectionReason string     `json:"rejectionReason" gorm:"type:text"` // rejection reason
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	// Non-database field: used to include like count and whether the current user has liked in API response
	LikesCount int  `json:"likesCount" gorm:"-"`
	IsLiked    bool `json:"isLiked" gorm:"-"`
	// Non-database field: comment count
	CommentsCount int `json:"commentsCount" gorm:"-"`
}

// Tag is a label attached to posts via a many-to-many association.
type Tag struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"size:100;uniqueIndex"`
}

// Category groups posts under a named, optionally described bucket.
type Category struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"size:100;uniqueIndex"`
	Description string `json:"description"`
}

// Like model
// Each record represents a user's like for a post

type Like struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	PostID    uint      `json:"postId" gorm:"uniqueIndex:idx_likes_post_user"`
	Post      Post      `json:"-" gorm:"foreignKey:PostID"`
	UserID    uint      `json:"userId" gorm:"uniqueIndex:idx_likes_post_user"`
	User      User      `json:"-" gorm:"foreignKey:UserID"`
	CreatedAt time.Time `json:"createdAt"`
}

// ToJSON converts a slice of Post to JSON string
func ToJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
