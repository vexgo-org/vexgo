// Wire types for the post domain. Swag reads the JSON tags
// to populate the OpenAPI spec; orval turns the generated
// schemas into TypeScript interfaces.
package post

import (
	"time"

	"github.com/google/uuid"
	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// PostResponse is the body of every post endpoint. It never embeds model.Post:
// the wire shape is decided here, not by the database schema.
type PostResponse struct {
	ID                    uint             `json:"id" example:"42"`
	Slug                  string           `json:"slug" example:"hello-world"`
	Title                 string           `json:"title" example:"Hello world"`
	Content               string           `json:"content"`
	Excerpt               string           `json:"excerpt"`
	Category              string           `json:"category" example:"engineering"`
	Tags                  []TagResponse    `json:"tags"`
	Status                model.PostStatus `json:"status" example:"published"`
	AuthorID              uint             `json:"authorId" example:"7"`
	Author                api.UserSummary  `json:"author"`
	CoverImagePublicID    *uuid.UUID       `json:"coverImagePublicID,omitempty" example:"01a15240-3847-71a4-b8bb-f8448ef3a952"`
	CoverImageOriginalURL string           `json:"coverImageOriginalURL,omitempty"`
	ViewCount             int              `json:"viewCount" example:"128"`
	LikesCount            int              `json:"likesCount" example:"3"`
	IsLiked               bool             `json:"isLiked" example:"false"`
	CommentsCount         int              `json:"commentsCount" example:"5"`
	// RejectionReason is only filled for the post's author and for admins.
	RejectionReason *string   `json:"rejectionReason,omitempty" example:"Off-topic"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// TagResponse is a tag on a post, in a tag list, or in a create/update reply.
type TagResponse struct {
	ID        uint   `json:"id" example:"3"`
	Name      string `json:"name" example:"golang"`
	PostCount int64  `json:"postCount,omitempty" example:"12"`
}

// CategoryResponse is a category in the post list and the category endpoints.
type CategoryResponse struct {
	ID          uint   `json:"id" example:"1"`
	Name        string `json:"name" example:"Tutorials"`
	Description string `json:"description"`
	PostCount   int64  `json:"postCount" example:"5"`
}

// PostListResponse is the body of any paged post list.
type PostListResponse struct {
	Posts      []PostResponse `json:"posts"`
	Pagination api.Pagination `json:"pagination"`
}

// PostListResponseData is the body of a list that doesn't
// include pagination (popular posts, latest posts).
type PostListResponseData struct {
	Posts []PostResponse `json:"posts"`
}

// PostSingleResponse is the body of GET /api/posts/{slug}
// and GET /api/posts/by-id/{id}.
type PostSingleResponse struct {
	Post *PostResponse `json:"post"`
}

// PostMessageResponse is the body of POST/PUT/DELETE
// /api/posts endpoints that return the updated row.
type PostMessageResponse struct {
	Message string        `json:"message" example:"Post created successfully"`
	Post    *PostResponse `json:"post"`
}

// PostDeleteResponse is the body of DELETE /api/posts/{id}.
type PostDeleteResponse struct {
	Message string `json:"message" example:"Post deleted successfully"`
}

// CreatePostRequest is the body of POST /api/posts.
type CreatePostRequest struct {
	Slug               string     `json:"slug" binding:"required" example:"my-first-post"`
	Title              string     `json:"title" binding:"required" example:"My First Post"`
	Content            string     `json:"content" binding:"required" example:"<p>Hello world</p>"`
	Category           string     `json:"category" binding:"required" example:"1"`
	Tags               []string   `json:"tags" example:"intro,personal"`
	Excerpt            string     `json:"excerpt" example:"A short summary"`
	CoverImagePublicID *uuid.UUID `json:"coverImagePublicID" example:"01a15240-3847-71a4-b8bb-f8448ef3a952"`
	Status             string     `json:"status" enums:"draft,pending,published" example:"published"`
}

// UpdatePostRequest is the body of PUT /api/posts/{id}. All
// fields are optional; only the supplied ones are updated.
type UpdatePostRequest struct {
	Slug               string     `json:"slug" example:"my-first-post"`
	Title              string     `json:"title" example:"My First Post"`
	Content            string     `json:"content" example:"<p>Hello world</p>"`
	Category           string     `json:"category" example:"1"`
	Tags               []string   `json:"tags" example:"intro,personal"`
	Excerpt            string     `json:"excerpt" example:"A short summary"`
	CoverImagePublicID *uuid.UUID `json:"coverImagePublicID" example:"01a15240-3847-71a4-b8bb-f8448ef3a952"`
	UnsetCoverImage    bool       `json:"unsetCoverImage" example:"false"`
	Status             string     `json:"status" enums:"draft,pending,published" example:"published"`
}

// CategoriesListResponse is the body of GET /api/categories.
type CategoriesListResponse struct {
	Categories []CategoryResponse `json:"categories"`
}

// CreateCategoryRequest is the body of POST /api/categories.
type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required,max=100" example:"Tutorials"`
	Description string `json:"description" binding:"max=500" example:"Step-by-step guides"`
}

// CreateCategoryResponse is the body of POST /api/categories.
type CreateCategoryResponse struct {
	Message  string            `json:"message" example:"Category created successfully"`
	Category *CategoryResponse `json:"category"`
}

// TagsListResponse is the body of GET /api/tags.
type TagsListResponse struct {
	Tags []TagResponse `json:"tags"`
}

// CreateTagRequest is the body of POST /api/tags.
type CreateTagRequest struct {
	Name string `json:"name" binding:"required,max=100" example:"golang"`
}

// CreateTagResponse is the body of POST /api/tags.
type CreateTagResponse struct {
	Message string       `json:"message" example:"Tag created successfully"`
	Tag     *TagResponse `json:"tag"`
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

// CreateRequest carries the fields accepted when creating a post.
type CreateRequest struct {
	Slug               string
	Title              string
	Content            string
	Category           string
	Tags               []string
	Excerpt            string
	CoverImagePublicID *uuid.UUID
	Status             model.PostStatus
}

// UpdateRequest carries the fields accepted when updating a post.
type UpdateRequest struct {
	Slug               string
	Title              string
	Content            string
	Category           string
	Tags               []string
	Excerpt            string
	CoverImagePublicID *uuid.UUID
	UnsetCoverImage    bool
	Status             model.PostStatus
}
