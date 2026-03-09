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
	RejectionReason string    `json:"rejectionReason" gorm:"type:text"` // 拒绝原因
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	// 非数据库字段：用于在 API 返回中包含点赞计数与当前用户是否已点赞
	LikesCount int  `json:"likesCount" gorm:"-"`
	IsLiked    bool `json:"isLiked" gorm:"-"`
	// 非数据库字段：评论计数
	CommentsCount int `json:"commentsCount" gorm:"-"`
}

type User struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	Username          string     `json:"username" binding:"required" gorm:"size:100;uniqueIndex"`
	Email             string     `json:"email" binding:"required,email" gorm:"size:255;uniqueIndex"`
	Password          string     `json:"-"`                                       // 不序列化
	Role              string     `json:"role" gorm:"size:50"`                     // super_admin/admin/author/contributor/guest
	Avatar            string     `json:"avatar,omitempty"`                        // 头像 URL
	EmailVerified     bool       `json:"email_verified"`                          // 邮箱是否已验证
	VerificationToken string     `json:"verification_token" gorm:"size:255"`      // 验证令牌
	TokenExpiresAt    *time.Time `json:"token_expires_at"`                        // 令牌过期时间（可为NULL）
	PendingEmail      string     `json:"pending_email,omitempty" gorm:"size:255"` // 待确认的新邮箱（用于邮箱变更）
	PasswordVersion   int        `json:"-" gorm:"default:1"`                      // 密码版本，用于密码修改后使旧令牌失效
	Birthday          string     `json:"birthday,omitempty"`                      // 生日
	Bio               string     `json:"bio,omitempty"`                           // 个人简介
	// 隐私设置
	ProfileVisibility string `json:"profile_visibility,omitempty" gorm:"size:20;default:'public'"` // public/private
	HideEmail         bool   `json:"hide_email,omitempty" gorm:"default:false"`                    // 隐藏邮箱
	HideBirthday      bool   `json:"hide_birthday,omitempty" gorm:"default:false"`                 // 隐藏生日
	HideBio           bool   `json:"hide_bio,omitempty" gorm:"default:false"`                      // 隐藏简介
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
	Status    string    `json:"status" gorm:"size:20;default:'published'"` // published, pending, rejected
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
	ID        uint      `json:"id" gorm:"primaryKey"`
	URL       string    `json:"url" gorm:"size:500"`
	Size      int64     `json:"size"`
	Type      string    `json:"type" gorm:"size:50"` // image/video 等
	UserID    uint      `json:"userId"`
	User      User      `json:"-" gorm:"foreignKey:UserID"`
	CreatedAt time.Time `json:"createdAt"`
}
