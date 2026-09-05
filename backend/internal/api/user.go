package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// UsersResponse is the body of GET /api/users.
type UsersResponse struct {
	Users      []model.User `json:"users" doc:"User rows"`
	Pagination Pagination   `json:"pagination" doc:"Paging metadata"`
}

// UserRoleUpdateResponse is the body of PUT /api/users/:id/role.
type UserRoleUpdateResponse struct {
	Message string     `json:"message" doc:"Human-readable outcome"`
	User    model.User `json:"user" doc:"The user after the role change"`
}

// CreatorApplicationApplyResponse is the body of POST /api/users/apply-creator.
type CreatorApplicationApplyResponse struct {
	Message       string `json:"message" doc:"Human-readable outcome"`
	ApplicationID uint   `json:"applicationId" doc:"Numeric application ID"`
}

// CreatorApplicationView is a creator application row as rendered
// for the admin review list: the applicant's identity is flattened
// so the table does not need to join user objects client-side.
type CreatorApplicationView struct {
	ID          uint                           `json:"id" doc:"Application ID"`
	UserID      uint                           `json:"userId" doc:"Applicant user ID"`
	Username    string                         `json:"username" doc:"Applicant username"`
	Email       string                         `json:"email" doc:"Applicant email"`
	CurrentRole string                         `json:"currentRole" doc:"Applicant's role at submission time"`
	Status      model.CreatorApplicationStatus `json:"status" doc:"pending|approved|rejected"`
	Reason      string                         `json:"reason" doc:"Free-text reason submitted by the applicant"`
	CreatedAt   string                         `json:"createdAt" doc:"RFC3339 timestamp"`
	UpdatedAt   string                         `json:"updatedAt" doc:"RFC3339 timestamp"`
}

// CreatorApplicationsResponse is the body of GET /api/users/creator-applications.
type CreatorApplicationsResponse struct {
	Applications []CreatorApplicationView `json:"applications" doc:"Application rows"`
	Pagination   Pagination               `json:"pagination" doc:"Paging metadata"`
}

// UpdateUserRoleRequest is the body of PUT /api/users/:id/role.
type UpdateUserRoleRequest struct {
	Role string `json:"role" required:"" doc:"Target role (super_admin, admin, author, contributor, guest)"`
}

// ApplyForCreatorRequest is the body of POST /api/users/apply-creator.
type ApplyForCreatorRequest struct {
	Reason string `json:"reason,omitempty" doc:"Free-text reason from the applicant"`
}

// ReviewCreatorApplicationRequest is the body of PUT /api/users/creator-applications/:id/review.
type ReviewCreatorApplicationRequest struct {
	Action string `json:"action" required:"" doc:"approve or reject"`
	Reason string `json:"reason,omitempty" doc:"Optional reviewer reason"`
}
