// Wire types for the post domain. Swag reads the JSON tags
// to populate the OpenAPI spec; orval turns the generated
// schemas into TypeScript interfaces.
package post

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Pagination describes a paged result. It is the same shape
// used by the rest of the REST surface.
type Pagination struct {
	Total      int64 `json:"total" example:"42"`
	Page       int   `json:"page" example:"1"`
	Limit      int   `json:"limit" example:"10"`
	TotalPages int   `json:"totalPages" example:"5"`
}

// PostListResponse is the body of any paged post list.
type PostListResponse struct {
	Posts      []model.Post `json:"posts"`
	Pagination Pagination   `json:"pagination"`
}

// PostListResponseData is the body of a list that doesn't
// include pagination (popular posts, latest posts).
type PostListResponseData struct {
	Posts []model.Post `json:"posts"`
}

// PostSingleResponse is the body of GET /api/posts/{slug}
// and GET /api/posts/by-id/{id}.
type PostSingleResponse struct {
	Post *model.Post `json:"post"`
}

// PostMessageResponse is the body of POST/PUT/DELETE
// /api/posts endpoints that return the updated row.
type PostMessageResponse struct {
	Message string      `json:"message" example:"Post created successfully"`
	Post    *model.Post `json:"post"`
}

// PostDeleteResponse is the body of DELETE /api/posts/{id}.
type PostDeleteResponse struct {
	Message string `json:"message" example:"Post deleted successfully"`
}

// CreatePostRequest is the body of POST /api/posts.
type CreatePostRequest struct {
	Slug       string   `json:"slug" binding:"required" example:"my-first-post"`
	Title      string   `json:"title" binding:"required" example:"My First Post"`
	Content    string   `json:"content" binding:"required" example:"<p>Hello world</p>"`
	Category   any      `json:"category" binding:"required" swaggertype:"primitive,integer" example:"1"`
	Tags       []string `json:"tags" example:"intro,personal"`
	Excerpt    string   `json:"excerpt" example:"A short summary"`
	CoverImage string   `json:"coverImage" example:"https://example.com/cover.jpg"`
	Status     string   `json:"status" enums:"draft,pending,published" example:"published"`
}

// UpdatePostRequest is the body of PUT /api/posts/{id}. All
// fields are optional; only the supplied ones are updated.
type UpdatePostRequest struct {
	Slug       string   `json:"slug" example:"my-first-post"`
	Title      string   `json:"title" example:"My First Post"`
	Content    string   `json:"content" example:"<p>Hello world</p>"`
	Category   any      `json:"category" swaggertype:"primitive,integer" example:"1"`
	Tags       []string `json:"tags" example:"intro,personal"`
	Excerpt    string   `json:"excerpt" example:"A short summary"`
	CoverImage string   `json:"coverImage" example:"https://example.com/cover.jpg"`
	Status     string   `json:"status" enums:"draft,pending,published" example:"published"`
}

// CategoriesListResponse is the body of GET /api/categories.
type CategoriesListResponse struct {
	Categories []model.Category `json:"categories"`
}

// CreateCategoryRequest is the body of POST /api/categories.
type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required,max=100" example:"Tutorials"`
	Description string `json:"description" binding:"max=500" example:"Step-by-step guides"`
}

// CreateCategoryResponse is the body of POST /api/categories.
type CreateCategoryResponse struct {
	Message  string          `json:"message" example:"Category created successfully"`
	Category *model.Category `json:"category"`
}

// TagsListResponse is the body of GET /api/tags.
type TagsListResponse struct {
	Tags []model.Tag `json:"tags"`
}

// CreateTagRequest is the body of POST /api/tags.
type CreateTagRequest struct {
	Name string `json:"name" binding:"required,max=100" example:"golang"`
}

// CreateTagResponse is the body of POST /api/tags.
type CreateTagResponse struct {
	Message string     `json:"message" example:"Tag created successfully"`
	Tag     *model.Tag `json:"tag"`
}

// DeleteMessageResponse is the body of DELETE on
// /api/categories/{id} and /api/tags/{id}.
type DeleteMessageResponse struct {
	Message string `json:"message" example:"Category deleted successfully"`
}

// RejectPostRequest is the body of PUT /api/moderation/reject/{id}.
type RejectPostRequest struct {
	RejectionReason string `json:"rejectionReason" example:"Off-topic"`
}

// LikeResponse is the body of POST /api/likes/{postId}.
type LikeResponse struct {
	Message    string `json:"message" example:"Liked successfully"`
	PostID     uint   `json:"postId" example:"42"`
	IsLiked    bool   `json:"isLiked" example:"true"`
	LikesCount int64  `json:"likesCount" example:"7"`
}

// LikeStatusResponse is the body of GET /api/likes/{postId}.
type LikeStatusResponse struct {
	PostID     uint  `json:"postId" example:"42"`
	IsLiked    bool  `json:"isLiked" example:"false"`
	LikesCount int64 `json:"likesCount" example:"7"`
}
