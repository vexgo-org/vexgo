// Package user exposes user administration and creator application
// endpoints. The handler is built around huma; the service layer
// (service.go, repository.go) is unchanged.
package user

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Handler exposes the user domain over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a user HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps)}
}

// ---------- input / output types ----------

type getUserListInput struct {
	Page   int    `query:"page" default:"1" doc:"1-based page index"`
	Limit  int    `query:"limit" default:"10" doc:"page size; capped at 100"`
	Search string `query:"search" default:"" doc:"Substring filter on username/email"`
}

type getUserListOutput struct {
	Body api.UsersResponse
}

type userIDPath struct {
	ID string `path:"id" doc:"Numeric user ID"`
}

type updateUserRoleInput struct {
	Body struct {
		Role string `json:"role" required:"" doc:"Target role (e.g. admin, author, contributor)"`
	}
	userIDPath
}

type userRoleUpdateOutput struct {
	Body api.UserRoleUpdateResponse
}

type messageOutput struct {
	Body api.MessageResponse
}

type applyForCreatorInput struct {
	Body api.ApplyForCreatorRequest
}

type applyForCreatorOutput struct {
	Body api.CreatorApplicationApplyResponse
}

type getCreatorApplicationsInput struct {
	Page   int    `query:"page" default:"1"`
	Limit  int    `query:"limit" default:"10"`
	Status string `query:"status" default:"pending" doc:"Filter by status (pending|approved|rejected)"`
}

type getCreatorApplicationsOutput struct {
	Body api.CreatorApplicationsResponse
}

type reviewCreatorApplicationInput struct {
	Body api.ReviewCreatorApplicationRequest
	userIDPath
}

// RegisterRoutes registers the user domain operations on the given
// huma.API. Admin-only endpoints are protected by
// auth.Permission(admin, super_admin).
func (h *Handler) RegisterRoutes(api huma.API) {
	adminOnly := auth.Permission(model.RoleAdmin, model.RoleSuperAdmin)

	huma.Register(api, huma.Operation{
		OperationID:   "list-users",
		Method:        http.MethodGet,
		Path:          "/users",
		Summary:       "List users (admin only)",
		Tags:          []string{"users"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.GetUserList)

	huma.Register(api, huma.Operation{
		OperationID:   "update-user-role",
		Method:        http.MethodPut,
		Path:          "/users/{id}/role",
		Summary:       "Update a user's role (admin only)",
		Tags:          []string{"users"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.UpdateUserRole)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-user",
		Method:        http.MethodDelete,
		Path:          "/users/{id}",
		Summary:       "Delete a user (admin only)",
		Tags:          []string{"users"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.DeleteUser)

	huma.Register(api, huma.Operation{
		OperationID: "apply-for-creator",
		Method:      http.MethodPost,
		Path:        "/users/apply-creator",
		Summary:     "Apply to become a creator",
		Tags:        []string{"users"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.ApplyForCreator)

	huma.Register(api, huma.Operation{
		OperationID:   "list-creator-applications",
		Method:        http.MethodGet,
		Path:          "/users/creator-applications",
		Summary:       "List creator applications (admin only)",
		Tags:          []string{"users"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.GetCreatorApplications)

	huma.Register(api, huma.Operation{
		OperationID:   "review-creator-application",
		Method:        http.MethodPut,
		Path:          "/users/creator-applications/{id}/review",
		Summary:       "Approve or reject a creator application (admin only)",
		Tags:          []string{"users"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.ReviewCreatorApplication)
}

// ---------- handlers ----------

func (h *Handler) GetUserList(ctx context.Context, in *getUserListInput) (*getUserListOutput, error) {
	page, limit := middleware.ParsePaginationValues(
		strconv.Itoa(in.Page), strconv.Itoa(in.Limit), middleware.DefaultPaginationLimit,
	)
	users, total, err := h.svc.ListUsers(ctx, in.Search, page, limit)
	if err != nil {
		return nil, huma.NewError(500, "Failed to query users")
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}
	return &getUserListOutput{
		Body: api.UsersResponse{
			Users: users,
			Pagination: api.Pagination{
				Total:      total,
				Page:       page,
				Limit:      limit,
				TotalPages: totalPages,
			},
		},
	}, nil
}

func (h *Handler) UpdateUserRole(ctx context.Context, in *updateUserRoleInput) (*userRoleUpdateOutput, error) {
	actor, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, huma.NewError(401, "No user information provided")
	}
	id, err := strconv.ParseUint(in.ID, 10, 32)
	if err != nil {
		return nil, huma.NewError(400, "Invalid user ID")
	}
	user, err := h.svc.UpdateRole(ctx, actor, uint(id), in.Body.Role)
	if err != nil {
		return nil, mapUserError(err)
	}
	return &userRoleUpdateOutput{
		Body: api.UserRoleUpdateResponse{
			Message: "User role updated successfully",
			User:    *user,
		},
	}, nil
}

func (h *Handler) DeleteUser(ctx context.Context, in *userIDPath) (*messageOutput, error) {
	actor, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, huma.NewError(401, "No user information provided")
	}
	id, err := strconv.ParseUint(in.ID, 10, 32)
	if err != nil {
		return nil, huma.NewError(400, "Invalid user ID")
	}
	if err := h.svc.DeleteUser(ctx, actor, uint(id)); err != nil {
		return nil, mapUserError(err)
	}
	return &messageOutput{
		Body: api.MessageResponse{Message: "User deleted successfully"},
	}, nil
}

func (h *Handler) ApplyForCreator(ctx context.Context, in *applyForCreatorInput) (*applyForCreatorOutput, error) {
	actor, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, huma.NewError(401, "No user information provided")
	}
	slog.Info("creator application submission", "actor", actor.ID)
	id, err := h.svc.ApplyForCreator(ctx, actor, in.Body.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrRoleNotEligible), errors.Is(err, ErrAlreadyPending):
			return nil, huma.NewError(400, err.Error())
		default:
			return nil, huma.NewError(500, "Failed to create application")
		}
	}
	return &applyForCreatorOutput{
		Body: api.CreatorApplicationApplyResponse{
			Message:       "Application submitted successfully",
			ApplicationID: id,
		},
	}, nil
}

func (h *Handler) GetCreatorApplications(ctx context.Context, in *getCreatorApplicationsInput) (*getCreatorApplicationsOutput, error) {
	actor, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, huma.NewError(401, "No user information provided")
	}
	page, limit := middleware.ParsePaginationValues(
		strconv.Itoa(in.Page), strconv.Itoa(in.Limit), middleware.DefaultPaginationLimit,
	)
	applications, total, err := h.svc.ListCreatorApplications(ctx, ListCreatorApplicationsQuery{
		ActorRole: actor.Role,
		Status:    model.CreatorApplicationStatus(in.Status),
		Page:      page,
		Limit:     limit,
	})
	if err != nil {
		if errors.Is(err, ErrNoPermissionAccessApps) {
			return nil, huma.NewError(403, err.Error())
		}
		return nil, huma.NewError(500, "Failed to query creator applications")
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}
	out := make([]api.CreatorApplicationView, 0, len(applications))
	for _, app := range applications {
		out = append(out, api.CreatorApplicationView{
			ID:          app.ID,
			UserID:      app.UserID,
			Username:    app.User.Username,
			Email:       app.User.Email,
			CurrentRole: app.User.Role,
			Status:      app.Status,
			Reason:      app.Reason,
			CreatedAt:   app.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:   app.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	return &getCreatorApplicationsOutput{
		Body: api.CreatorApplicationsResponse{
			Applications: out,
			Pagination: api.Pagination{
				Total:      total,
				Page:       page,
				Limit:      limit,
				TotalPages: totalPages,
			},
		},
	}, nil
}

func (h *Handler) ReviewCreatorApplication(ctx context.Context, in *reviewCreatorApplicationInput) (*messageOutput, error) {
	actor, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, huma.NewError(401, "No user information provided")
	}
	id, err := strconv.ParseUint(in.ID, 10, 32)
	if err != nil {
		return nil, huma.NewError(400, "Invalid application ID")
	}
	if err := h.svc.ReviewCreatorApplication(ctx, ReviewCreatorApplicationRequest{
		Actor:  actor,
		AppID:  uint(id),
		Action: in.Body.Action,
		Reason: in.Body.Reason,
	}); err != nil {
		switch {
		case errors.Is(err, ErrNoPermissionReviewApps):
			return nil, huma.NewError(403, err.Error())
		case errors.Is(err, ErrApplicationNotFound):
			return nil, huma.NewError(404, "Application does not exist")
		case errors.Is(err, ErrApplicationProcessed), errors.Is(err, ErrInvalidAction):
			return nil, huma.NewError(400, err.Error())
		default:
			return nil, huma.NewError(500, "Failed to update application")
		}
	}
	return &messageOutput{
		Body: api.MessageResponse{Message: "Application reviewed successfully"},
	}, nil
}

func mapUserError(err error) error {
	switch {
	case errors.Is(err, ErrUserNotFound):
		return huma.NewError(404, "User does not exist")
	case errors.Is(err, ErrCannotModifySelf),
		errors.Is(err, ErrInvalidRole),
		errors.Is(err, ErrCannotDeleteSelf):
		return huma.NewError(400, err.Error())
	case errors.Is(err, ErrModifySuperAdmin),
		errors.Is(err, ErrSuperAdminRestricted),
		errors.Is(err, ErrAdminRoleRestricted),
		errors.Is(err, ErrNoPermission),
		errors.Is(err, ErrAdminDeleteRestricted),
		errors.Is(err, ErrNoPermissionToDelete):
		return huma.NewError(403, err.Error())
	default:
		return huma.NewError(500, "Failed to process user request")
	}
}
