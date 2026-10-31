package page

import (
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// PageResponse is the wire shape of a page. The page author is not exposed:
// nothing reads it, and the API should not hand out account data for it.
type PageResponse struct {
	ID        uint             `json:"id" example:"2"`
	Slug      string           `json:"slug" example:"about"`
	Title     string           `json:"title" example:"About"`
	Content   string           `json:"content"`
	ShowInNav bool             `json:"showInNav" example:"true"`
	SortOrder int              `json:"sortOrder" example:"1"`
	Status    model.PageStatus `json:"status" example:"published"`
	AuthorID  uint             `json:"authorId" example:"7"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
}

// PageListResponse is the body of any paged page list.
type PageListResponse struct {
	Pages      []PageResponse `json:"pages"`
	Pagination api.Pagination `json:"pagination"`
}

// PageSingleResponse is the body of GET /api/pages/{slug}.
type PageSingleResponse struct {
	Page *PageResponse `json:"page"`
}

// PageMessageResponse is the body of POST/PUT /api/pages endpoints.
type PageMessageResponse struct {
	Message string        `json:"message" example:"Page created successfully"`
	Page    *PageResponse `json:"page"`
}

// PageDeleteResponse is the body of DELETE /api/pages/{id}.
type PageDeleteResponse struct {
	Message string `json:"message" example:"Page deleted successfully"`
}

// CreatePageRequest is the body of POST /api/pages.
type CreatePageRequest struct {
	Slug      string `json:"slug" binding:"required" example:"about"`
	Title     string `json:"title" binding:"required" example:"About"`
	Content   string `json:"content" binding:"required" example:"Hello"`
	ShowInNav bool   `json:"showInNav" example:"true"`
	SortOrder int    `json:"sortOrder" example:"10"`
	Status    string `json:"status" enums:"draft,published" example:"published"`
}

// UpdatePageRequest is the body of PUT /api/pages/{id}. All fields optional.
type UpdatePageRequest struct {
	Slug      string `json:"slug" example:"about"`
	Title     string `json:"title" example:"About"`
	Content   string `json:"content" example:"Hello"`
	ShowInNav *bool  `json:"showInNav"`
	SortOrder *int   `json:"sortOrder"`
	Status    string `json:"status" enums:"draft,published" example:"published"`
}
