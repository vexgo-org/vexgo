package page

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Pagination describes a paged result.
type Pagination struct {
	Total      int64 `json:"total" example:"42"`
	Page       int   `json:"page" example:"1"`
	Limit      int   `json:"limit" example:"10"`
	TotalPages int   `json:"totalPages" example:"5"`
}

// PageListResponse is the body of any paged page list.
type PageListResponse struct {
	Pages      []model.Page `json:"pages"`
	Pagination Pagination   `json:"pagination"`
}

// PageSingleResponse is the body of GET /api/pages/{slug}.
type PageSingleResponse struct {
	Page *model.Page `json:"page"`
}

// PageMessageResponse is the body of POST/PUT /api/pages endpoints.
type PageMessageResponse struct {
	Message string      `json:"message" example:"Page created successfully"`
	Page    *model.Page `json:"page"`
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
