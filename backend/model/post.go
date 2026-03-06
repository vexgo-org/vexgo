// backend/model/post.go
package model

import "time"

type Post struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Title      string    `json:"title" binding:"required"`
	Content    string    `json:"content" binding:"required" gorm:"type:text"`
	Excerpt    string    `json:"excerpt" gorm:"type:text"`
	CoverImage string    `json:"coverImage"`
	ViewCount  int       `json:"viewCount" gorm:"default:0"`
	AuthorID   uint      `json:"author_id"`
	Author     User      `json:"author" gorm:"foreignKey:AuthorID"`
	Category   string    `json:"category"`
	Tags       []Tag     `json:"tags" gorm:"many2many:post_tags;"`
	Status     string    `json:"status"` // draft/published/pending/rejected
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	// 非数据库字段：用于在 API 返回中包含点赞计数与当前用户是否已点赞
	LikesCount int  `json:"likesCount" gorm:"-"`
	IsLiked    bool `json:"isLiked" gorm:"-"`
}


type User struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	Username          string    `json:"username" binding:"required"`
	Email             string    `json:"email" binding:"required,email"`
	Password          string    `json:"-"`                  // 不序列化
	Role              string    `json:"role"`               // super_admin/admin/author/contributor/guest
	Avatar            string    `json:"avatar,omitempty"`   // 头像 URL
	EmailVerified     bool      `json:"email_verified"`     // 邮箱是否已验证
	VerificationToken string    `json:"verification_token"` // 验证令牌
	TokenExpiresAt    time.Time `json:"token_expires_at"`   // 令牌过期时间
}

type Tag struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"uniqueIndex"`
}

type Category struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"uniqueIndex"`
	Description string `json:"description"`
}

// 评论模型
// 支持父评论(parentId) 用于嵌套
// 关联 User 用于返回作者信息
// 关联 Post 以便统计或删除时级联
// GORM 会自动创建 foreign key

type Comment struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	PostID    uint      `json:"postId"`
	Post      Post      `json:"-" gorm:"foreignKey:PostID"`
	UserID    uint      `json:"userId"`
	User      User      `json:"author" gorm:"foreignKey:UserID"`
	Content   string    `json:"content" gorm:"type:text"`
	ParentID  *uint     `json:"parentId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// 点赞模型
// 每条记录表示一个用户对某文章的点赞

type Like struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	PostID    uint      `json:"postId"`
	Post      Post      `json:"-" gorm:"foreignKey:PostID"`
	UserID    uint      `json:"userId"`
	User      User      `json:"-" gorm:"foreignKey:UserID"`
	CreatedAt time.Time `json:"createdAt"`
}

// 媒体文件模型，用于记录用户上传的资源

type MediaFile struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	URL          string    `json:"url"`
	OriginalName string    `json:"originalName"`
	Size         int64     `json:"size"`
	Type         string    `json:"type"` // image/video 等
	UserID       uint      `json:"userId"`
	User         User      `json:"-" gorm:"foreignKey:UserID"`
	CreatedAt    time.Time `json:"createdAt"`
}
