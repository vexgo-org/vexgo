package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// PostsResponse is the body of the paged post list endpoints
// (GET /api/posts and the moderation queue).
type PostsResponse struct {
	Posts      []model.Post `json:"posts"`
	Pagination Pagination   `json:"pagination"`
}

// PostResponse is the body of GET /api/posts/:slug and GET /api/posts/by-id/:id.
type PostResponse struct {
	Post model.Post `json:"post"`
}

// PostListResponse is the body of the unpaged post list endpoints
// (GET /api/stats/popular-posts and GET /api/stats/latest-posts).
type PostListResponse struct {
	Posts []model.Post `json:"posts"`
}

// PostMutationResponse is the body of create/update/moderation actions that
// return the affected post together with a message.
type PostMutationResponse struct {
	Message string     `json:"message"`
	Post    model.Post `json:"post"`
}

// LikeResponse is the body of POST /api/likes/:postId and GET /api/likes/:postId.
// The message is only present on the toggle path.
type LikeResponse struct {
	Message    string `json:"message,omitempty"`
	PostID     uint   `json:"postId"`
	IsLiked    bool   `json:"isLiked"`
	LikesCount int64  `json:"likesCount"`
}

// CategoryListResponse is the body of GET /api/categories.
type CategoryListResponse struct {
	Categories []model.Category `json:"categories"`
}

// CategoryCreateResponse is the body of POST /api/categories.
type CategoryCreateResponse struct {
	Message  string         `json:"message"`
	Category model.Category `json:"category"`
}

// TagListResponse is the body of GET /api/tags.
type TagListResponse struct {
	Tags []model.Tag `json:"tags"`
}

// TagCreateResponse is the body of POST /api/tags.
type TagCreateResponse struct {
	Message string    `json:"message"`
	Tag     model.Tag `json:"tag"`
}

// CreatePostRequest is the body of POST /api/posts. Category accepts either a
// numeric category ID or a category name.
type CreatePostRequest struct {
	Slug       string   `json:"slug" binding:"required"`
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content" binding:"required"`
	Category   any      `json:"category" binding:"required" tstype:"number | string"`
	Tags       []string `json:"tags,omitempty"`
	Excerpt    string   `json:"excerpt,omitempty"`
	CoverImage string   `json:"coverImage,omitempty"`
	Status     string   `json:"status,omitempty"`
}

// UpdatePostRequest is the body of PUT /api/posts/:id. Empty values are
// applied as-is, so callers send every field they want persisted.
type UpdatePostRequest struct {
	Slug       string   `json:"slug,omitempty"`
	Title      string   `json:"title,omitempty"`
	Content    string   `json:"content,omitempty"`
	Category   any      `json:"category,omitempty" tstype:"number | string"`
	Tags       []string `json:"tags,omitempty"`
	Excerpt    string   `json:"excerpt,omitempty"`
	CoverImage string   `json:"coverImage,omitempty"`
	Status     string   `json:"status,omitempty"`
}

// CreateCategoryRequest is the body of POST /api/categories.
type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description,omitempty" binding:"max=500"`
}

// CreateTagRequest is the body of POST /api/tags.
type CreateTagRequest struct {
	Name string `json:"name" binding:"required,max=100"`
}

// RejectPostRequest is the body of PUT /api/moderation/reject/:id.
type RejectPostRequest struct {
	RejectionReason string `json:"rejectionReason,omitempty"`
}
