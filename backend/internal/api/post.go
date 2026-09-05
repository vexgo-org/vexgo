package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// PostsResponse is the body of GET /api/posts and other paginated
// post list endpoints (my-posts, drafts, user-posts, moderation).
type PostsResponse struct {
	Posts      []model.Post `json:"posts" doc:"Post rows"`
	Pagination Pagination   `json:"pagination" doc:"Paging metadata"`
}

// PostsListResponse is the body of GET /api/stats/popular-posts
// and GET /api/stats/latest-posts — paginated-light variants
// without explicit paging metadata.
type PostsListResponse struct {
	Posts []model.Post `json:"posts" doc:"Post rows"`
}

// PostResponse is the body of GET /api/posts/{slug} and
// /api/posts/by-id/{id}.
type PostResponse struct {
	Post *model.Post `json:"post" doc:"The fetched post"`
}

// CreatePostRequest is the body of POST /api/posts.
type CreatePostRequest struct {
	Slug       string   `json:"slug" required:"" doc:"URL-safe slug; must be unique"`
	Title      string   `json:"title" required:""`
	Content    string   `json:"content" required:""`
	Category   any      `json:"category" required:"" doc:"Category ID (number or string) or category name"`
	Tags       []string `json:"tags,omitempty" doc:"Tag names to attach"`
	Excerpt    string   `json:"excerpt,omitempty"`
	CoverImage string   `json:"coverImage,omitempty"`
	Status     string   `json:"status,omitempty" doc:"draft|pending|published|rejected"`
}

// CreatePostResponse is the body of POST /api/posts.
type CreatePostResponse struct {
	Message string      `json:"message"`
	Post    *model.Post `json:"post"`
}

// UpdatePostRequest is the body of PUT /api/posts/{id}. All
// fields are optional; the service decides which ones to apply.
type UpdatePostRequest struct {
	Slug       string   `json:"slug,omitempty"`
	Title      string   `json:"title,omitempty"`
	Content    string   `json:"content,omitempty"`
	Category   any      `json:"category,omitempty" doc:"Category ID or name (optional)"`
	Tags       []string `json:"tags,omitempty"`
	Excerpt    string   `json:"excerpt,omitempty"`
	CoverImage string   `json:"coverImage,omitempty"`
	Status     string   `json:"status,omitempty"`
}

// PostMutationResponse is the body of approve/reject/resubmit/update.
type PostMutationResponse struct {
	Message string      `json:"message"`
	Post    *model.Post `json:"post"`
}

// RejectPostRequest is the body of PUT /api/moderation/reject/{id}.
type RejectPostRequest struct {
	RejectionReason string `json:"rejectionReason,omitempty" doc:"Optional reviewer note"`
}

// CreateCategoryRequest is the body of POST /api/categories. The
// handler enforces name/description length limits so the
// validation message stays generic; huma-side validation is
// omitted to preserve the legacy 400 status (not 422).
type CreateCategoryRequest struct {
	Name        string `json:"name" required:"" doc:"1-100 characters"`
	Description string `json:"description,omitempty" doc:"Up to 500 characters"`
}

// CreateCategoryResponse is the body of POST /api/categories.
type CreateCategoryResponse struct {
	Message  string         `json:"message"`
	Category *model.Category `json:"category" doc:"The new category"`
}

// CreateTagRequest is the body of POST /api/tags. The handler
// enforces the name length limit to keep the validation
// response generic; huma-side validation is omitted to preserve
// the legacy 400 status.
type CreateTagRequest struct {
	Name string `json:"name" required:"" doc:"1-100 characters"`
}

// CreateTagResponse is the body of POST /api/tags.
type CreateTagResponse struct {
	Message string     `json:"message"`
	Tag     *model.Tag `json:"tag"`
}

// CategoriesResponse is the body of GET /api/categories.
type CategoriesResponse struct {
	Categories []model.Category `json:"categories"`
}

// TagsResponse is the body of GET /api/tags.
type TagsResponse struct {
	Tags []model.Tag `json:"tags"`
}

// LikeStatusResponse is the body of GET /api/likes/{postId}.
type LikeStatusResponse struct {
	PostID     uint `json:"postId"`
	IsLiked    bool `json:"isLiked"`
	LikesCount int  `json:"likesCount"`
}

// LikeToggleResponse is the body of POST /api/likes/{postId}.
type LikeToggleResponse struct {
	Message    string `json:"message"`
	PostID     uint   `json:"postId"`
	IsLiked    bool   `json:"isLiked"`
	LikesCount int    `json:"likesCount"`
}
