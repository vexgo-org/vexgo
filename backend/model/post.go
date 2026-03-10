// backend/model/post.go
package model

import "time"

type Post struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Title           string    `json:"title" binding:"required" gorm:"size:255"`
	Content         string    `json:"content" binding:"required" gorm:"type:text"`
	Excerpt         string    `json:"excerpt" gorm:"type:text"`
	CoverImage      string    `json:"coverImage" gorm:"size:500"`
	ViewCount       int       `json:"viewCount" gorm:"default:0"`
	AuthorID        uint      `json:"authorId"`
	Author          User      `json:"author" gorm:"foreignKey:AuthorID"`
	Category        string    `json:"category" gorm:"size:100"`
	Tags            []Tag     `json:"tags" gorm:"many2many:post_tags;"`
	Status          string    `json:"status" gorm:"size:50"`            // draft/published/pending/rejected
	RejectionReason string    `json:"rejectionReason" gorm:"type:text"` // rejection reason
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	// Non-database field: used to include like count and whether the current user has liked in API response
	LikesCount int  `json:"likesCount" gorm:"-"`
	IsLiked    bool `json:"isLiked" gorm:"-"`
	// Non-database field: comment count
	CommentsCount int `json:"commentsCount" gorm:"-"`
}

type User struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	Username          string     `json:"username" binding:"required" gorm:"size:100;uniqueIndex"`
	Email             string     `json:"email" binding:"required,email" gorm:"size:255;uniqueIndex"`
	Password          string     `json:"-"`                                       // not serialized
	Role              string     `json:"role" gorm:"size:50"`                     // super_admin/admin/author/contributor/guest
	Avatar            string     `json:"avatar,omitempty"`                        // avatar URL
	EmailVerified     bool       `json:"email_verified"`                          // whether email is verified
	VerificationToken string     `json:"verification_token" gorm:"size:255"`      // verification token
	TokenExpiresAt    *time.Time `json:"token_expires_at"`                        // token expiration time (can be NULL)
	PendingEmail      string     `json:"pending_email,omitempty" gorm:"size:255"` // new email pending confirmation (for email change)
	PasswordVersion   int        `json:"-" gorm:"default:1"`                      // password version, used to invalidate old tokens after password modification
	Birthday          string     `json:"birthday,omitempty"`                      // birthday
	Bio               string     `json:"bio,omitempty"`                           // personal bio
	// Privacy settings
	ProfileVisibility string `json:"profile_visibility,omitempty" gorm:"size:20;default:'public'"` // public/private
	HideEmail         bool   `json:"hide_email,omitempty" gorm:"default:false"`                    // hide email
	HideBirthday      bool   `json:"hide_birthday,omitempty" gorm:"default:false"`                 // hide birthday
	HideBio           bool   `json:"hide_bio,omitempty" gorm:"default:false"`                      // hide bio
}

type Tag struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"size:100;uniqueIndex"`
}

type Category struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"size:100;uniqueIndex"`
	Description string `json:"description"`
}

// Comment model
// Supports parent comments (parentId) for nesting
// Associated with User to return author information
// Associated with Post for cascading on statistics or deletion
// GORM automatically creates foreign key

type Comment struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	PostID    uint      `json:"postId"`
	Post      Post      `json:"-" gorm:"foreignKey:PostID"`
	UserID    uint      `json:"userId"`
	User      User      `json:"author" gorm:"foreignKey:UserID"`
	Content   string    `json:"content" gorm:"type:text"`
	Status    string    `json:"status" gorm:"size:20;default:'published'"` // published, pending, rejected
	ParentID  *uint     `json:"parentId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Like model
// Each record represents a user's like for a post

type Like struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	PostID    uint      `json:"postId"`
	Post      Post      `json:"-" gorm:"foreignKey:PostID"`
	UserID    uint      `json:"userId"`
	User      User      `json:"-" gorm:"foreignKey:UserID"`
	CreatedAt time.Time `json:"createdAt"`
}

// Media file model, used to record user uploaded resources

type MediaFile struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	URL       string    `json:"url" gorm:"size:500"`
	Size      int64     `json:"size"`
	Type      string    `json:"type" gorm:"size:50"` // image/video etc.
	UserID    uint      `json:"userId"`
	User      User      `json:"-" gorm:"foreignKey:UserID"`
	CreatedAt time.Time `json:"createdAt"`
}
