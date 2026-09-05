package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// UsersResponse is the body of GET /api/users.
type UsersResponse struct {
	Users      []model.User `json:"users"`
	Pagination Pagination   `json:"pagination"`
}

// UserRoleUpdateResponse is the body of PUT /api/users/:id/role.
type UserRoleUpdateResponse struct {
	Message string     `json:"message"`
	User    model.User `json:"user"`
}

// CreatorApplicationApplyResponse is the body of POST /api/users/apply-creator.
type CreatorApplicationApplyResponse struct {
	Message       string `json:"message"`
	ApplicationID uint   `json:"applicationId"`
}

// CreatorApplicationView is a creator application row as rendered for the
// admin review list: the applicant's identity is flattened so the table does
// not need to join user objects client-side. Dates are formatted by the
// handler (RFC3339) rather than serialized from time.Time.
type CreatorApplicationView struct {
	ID          uint                           `json:"id"`
	UserID      uint                           `json:"userId"`
	Username    string                         `json:"username"`
	Email       string                         `json:"email"`
	CurrentRole string                         `json:"currentRole"`
	Status      model.CreatorApplicationStatus `json:"status"`
	Reason      string                         `json:"reason"`
	CreatedAt   string                         `json:"createdAt"`
	UpdatedAt   string                         `json:"updatedAt"`
}

// CreatorApplicationsResponse is the body of GET /api/users/creator-applications.
type CreatorApplicationsResponse struct {
	Applications []CreatorApplicationView `json:"applications"`
	Pagination   Pagination               `json:"pagination"`
}

// UpdateUserRoleRequest is the body of PUT /api/users/:id/role.
type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// ApplyForCreatorRequest is the body of POST /api/users/apply-creator.
type ApplyForCreatorRequest struct {
	Reason string `json:"reason,omitempty"`
}

// ReviewCreatorApplicationRequest is the body of PUT
// /api/users/creator-applications/:id/review.
type ReviewCreatorApplicationRequest struct {
	Action string `json:"action" binding:"required"`
	Reason string `json:"reason,omitempty"`
}
