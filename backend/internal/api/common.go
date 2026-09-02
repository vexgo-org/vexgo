// Package api holds the HTTP response types shared by the API handlers.
//
// These structs are the single source of truth for the JSON response shapes:
// handlers render them (instead of ad-hoc gin.H maps) and tygo derives the
// matching TypeScript interfaces for the frontend from them.
package api

// Pagination describes a paged list result.
type Pagination struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"totalPages"`
}

// MessageResponse is the envelope for endpoints that only report an outcome.
type MessageResponse struct {
	Message string `json:"message"`
}
