// Package api holds the HTTP response types shared by the API
// handlers. huma derives the OpenAPI 3.1 schema from these
// structs (backend/cmd/openapi-spec walks the registry and
// writes docs/openapi.json); orval consumes that spec to
// generate the typed axios client under
// frontend/src/api/generated/. The JSON wire shape and the
// Go struct definitions are the single source of truth.
package api

// Pagination describes a paged list result. Embedded in any
// response that includes pagination.
type Pagination struct {
	Total      int64 `json:"total" doc:"Total number of items across all pages"`
	Page       int   `json:"page" doc:"Current 1-based page index"`
	Limit      int   `json:"limit" doc:"Page size"`
	TotalPages int   `json:"totalPages" doc:"Total number of pages"`
}

// MessageResponse is the envelope for endpoints that only report
// an outcome.
type MessageResponse struct {
	Message string `json:"message" doc:"Human-readable outcome message"`
}
