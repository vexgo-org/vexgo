// Wire types for the user domain. Swag reads the JSON tags
// to populate the OpenAPI spec; orval turns the generated
// schemas into TypeScript interfaces.
package user

import (
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// AdminUserResponse is the user shape the admin console sees. It is the only
// user type that carries an email besides the authenticated user's own
// response, and every endpoint returning it is admin-only.
type AdminUserResponse struct {
	ID        uint      `json:"id" example:"42"`
	Username  string    `json:"username" example:"alice"`
	Email     string    `json:"email" example:"alice@example.com"`
	Role      string    `json:"role" example:"author"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreatorApplicationResponse is one application in the admin review queue.
type CreatorApplicationResponse struct {
	ID           uint                           `json:"id" example:"5"`
	User         AdminUserResponse              `json:"user"`
	Status       model.CreatorApplicationStatus `json:"status" example:"pending"`
	Reason       string                         `json:"reason"`
	ReviewerID   *uint                          `json:"reviewerId,omitempty"`
	ReviewReason string                         `json:"reviewReason"`
	CreatedAt    time.Time                      `json:"createdAt"`
	UpdatedAt    time.Time                      `json:"updatedAt"`
}

// UserListResponse is the body of GET /api/users (admin only).
type UserListResponse struct {
	Users      []AdminUserResponse `json:"users"`
	Pagination api.Pagination      `json:"pagination"`
}

// UserMessageResponse is the uniform `{ "message", "user" }`
// body used by PUT /api/users/{id}/role.
type UserMessageResponse struct {
	Message string             `json:"message" example:"User role updated successfully"`
	User    *AdminUserResponse `json:"user"`
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
	Applications []CreatorApplicationResponse `json:"applications"`
	Pagination   api.Pagination               `json:"pagination"`
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
