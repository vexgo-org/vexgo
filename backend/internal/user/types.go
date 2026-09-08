// Wire types for the user domain. Swag reads the JSON tags
// to populate the OpenAPI spec; orval turns the generated
// schemas into TypeScript interfaces.
package user

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// UserListResponse is the body of GET /api/users (admin only).
type UserListResponse struct {
	Users      []model.User `json:"users"`
	Pagination Pagination   `json:"pagination"`
}

// UserMessageResponse is the uniform `{ "message", "user" }`
// body used by PUT /api/users/{id}/role.
type UserMessageResponse struct {
	Message string      `json:"message" example:"User role updated successfully"`
	User    *model.User `json:"user"`
}

// MessageResponse is a uniform `{ "message": "..." }` body.
type MessageResponse struct {
	Message string `json:"message" example:"User deleted successfully"`
}

// ApplyForCreatorRequest is the body of POST
// /api/users/apply-creator. The reason is optional but
// encouraged so admins have context for the decision.
type ApplyForCreatorRequest struct {
	Reason string `json:"reason" example:"I've been writing drafts and want to publish"`
}

// ApplyForCreatorResponse is the body of POST
// /api/users/apply-creator on success.
type ApplyForCreatorResponse struct {
	Message       string `json:"message" example:"Application submitted successfully"`
	ApplicationID string `json:"applicationId" example:"42"`
}

// CreatorApplicationListResponse is the body of GET
// /api/users/creator-applications. The application shape is
// the CreatorApplication struct from model; the pagination
// shape is the same as everywhere else.
type CreatorApplicationListResponse struct {
	Applications []model.CreatorApplication `json:"applications"`
	Pagination   Pagination                 `json:"pagination"`
}

// ReviewCreatorApplicationBody is the body of PUT
// /api/users/creator-applications/{id}/review. The service
// layer's ReviewCreatorApplicationRequest is its own struct;
// this is the JSON wire shape only.
type ReviewCreatorApplicationBody struct {
	// Action is "approve" or "reject". The service layer
	// rejects anything else.
	Action string `json:"action" binding:"required" enums:"approve,reject" example:"approve"`
	// Reason is a human-readable explanation shown to the
	// applicant in the notification. Optional.
	Reason string `json:"reason" example:"Your writing samples look good"`
}

// Pagination describes a paged result. It is the same shape
// used by the rest of the REST surface; declaring it here
// (rather than pulling it out into a shared api package)
// keeps each domain self-contained for swag.
type Pagination struct {
	Total      int64 `json:"total" example:"42"`
	Page       int   `json:"page" example:"1"`
	Limit      int   `json:"limit" example:"10"`
	TotalPages int64 `json:"totalPages" example:"5"`
}
