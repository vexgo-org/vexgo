// Package comment handles user comments on posts plus the
// moderation queue.
package comment

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Handler exposes the comment domain over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a comment HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps)}
}

// ---------- input / output types ----------

type listPostCommentsInput struct {
	PostID string `path:"id" doc:"Numeric post ID"`
}

type listPostCommentsOutput struct {
	Body api.CommentsListResponse
}

type createCommentInput struct {
	Body api.CreateCommentRequest
}

type createCommentOutput struct {
	Status int                           `status:"201" required:"" doc:"HTTP status; always 201 for a successful create"`
	Body   api.CreateCommentResponse
}

type deleteCommentInput struct {
	ID string `path:"id" doc:"Numeric comment ID"`
}

type commentMutationOutput struct {
	Body api.CommentMutationResponse
}

type moderationListInput struct {
	Page  int `query:"page" default:"1"`
	Limit int `query:"limit" default:"10"`
}

type moderationListOutput struct {
	Body api.ModerationCommentsResponse
}

type moderationActionInput struct {
	ID string `path:"id" doc:"Numeric comment ID"`
}

type moderationConfigOutput struct {
	Body model.CommentModerationConfig
}

type updateModerationConfigInput struct {
	Body api.UpdateCommentModerationConfigRequest
}

type updateModerationConfigOutput struct {
	Body api.UpdateCommentModerationConfigResponse
}

type testModerationOutput struct {
	Body api.TestModerationResponse
}

// RegisterRoutes registers the comment domain operations.
func (h *Handler) RegisterRoutes(api huma.API) {
	adminOnly := auth.Permission(model.RoleAdmin, model.RoleSuperAdmin)

	huma.Register(api, huma.Operation{
		OperationID: "list-post-comments",
		Method:      http.MethodGet,
		Path:        "/comments/post/{id}",
		Summary:     "List comments for a post",
		Tags:        []string{"comments"},
	}, h.GetComments)

	huma.Register(api, huma.Operation{
		OperationID: "create-comment",
		Method:      http.MethodPost,
		Path:        "/comments",
		Summary:     "Create a comment",
		Tags:        []string{"comments"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.CreateComment)

	huma.Register(api, huma.Operation{
		OperationID: "delete-comment",
		Method:      http.MethodDelete,
		Path:        "/comments/{id}",
		Summary:     "Delete a comment (author or admin)",
		Tags:        []string{"comments"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.DeleteComment)

	huma.Register(api, huma.Operation{
		OperationID:   "list-pending-comments",
		Method:        http.MethodGet,
		Path:          "/moderation/comments/pending",
		Summary:       "List pending comments (admin)",
		Tags:          []string{"comment-moderation"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.GetPendingComments)

	huma.Register(api, huma.Operation{
		OperationID:   "list-approved-comments",
		Method:        http.MethodGet,
		Path:          "/moderation/comments/approved",
		Summary:       "List approved comments (admin)",
		Tags:          []string{"comment-moderation"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.GetApprovedComments)

	huma.Register(api, huma.Operation{
		OperationID:   "list-rejected-comments",
		Method:        http.MethodGet,
		Path:          "/moderation/comments/rejected",
		Summary:       "List rejected comments (admin)",
		Tags:          []string{"comment-moderation"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.GetRejectedComments)

	huma.Register(api, huma.Operation{
		OperationID:   "approve-comment",
		Method:        http.MethodPut,
		Path:          "/moderation/comments/approve/{id}",
		Summary:       "Approve a comment (admin)",
		Tags:          []string{"comment-moderation"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.ApproveComment)

	huma.Register(api, huma.Operation{
		OperationID:   "reject-comment",
		Method:        http.MethodPut,
		Path:          "/moderation/comments/reject/{id}",
		Summary:       "Reject a comment (admin)",
		Tags:          []string{"comment-moderation"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.RejectComment)

	huma.Register(api, huma.Operation{
		OperationID:   "get-comment-moderation-config",
		Method:        http.MethodGet,
		Path:          "/moderation/comments/config",
		Summary:       "Get comment moderation configuration (admin)",
		Tags:          []string{"comment-moderation"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.GetCommentModerationConfig)

	huma.Register(api, huma.Operation{
		OperationID:   "update-comment-moderation-config",
		Method:        http.MethodPut,
		Path:          "/moderation/comments/config",
		Summary:       "Update comment moderation configuration (admin)",
		Tags:          []string{"comment-moderation"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.UpdateCommentModerationConfig)

	huma.Register(api, huma.Operation{
		OperationID:   "test-comment-moderation",
		Method:        http.MethodPost,
		Path:          "/moderation/comments/config/test",
		Summary:       "Test the stored LLM moderation configuration (admin)",
		Tags:          []string{"comment-moderation"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Middlewares:   huma.Middlewares{adminOnly},
	}, h.TestModerationConfig)
}

// ---------- handlers ----------

func (h *Handler) GetComments(ctx context.Context, in *listPostCommentsInput) (*listPostCommentsOutput, error) {
	u, _ := auth.UserFromContext(ctx)
	comments, err := h.svc.ListByPost(ctx, in.PostID, u.ID, u.Role)
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch comments")
	}
	return &listPostCommentsOutput{Body: api.CommentsListResponse{Comments: comments}}, nil
}

func (h *Handler) CreateComment(ctx context.Context, in *createCommentInput) (*createCommentOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, huma.NewError(401, "Not logged in")
	}
	if len(in.Body.Content) > 100 {
		return nil, huma.NewError(400, "Comment cannot exceed 100 characters")
	}
	postID, err := parseUintAny(in.Body.PostID)
	if err != nil {
		return nil, huma.NewError(400, "Invalid postId")
	}
	comment, count, err := h.svc.Create(ctx, CreateRequest{
		PostID:   postID,
		UserID:   userID,
		Content:  in.Body.Content,
		ParentID: in.Body.ParentID,
	})
	if err != nil {
		return nil, huma.NewError(500, "Failed to create comment")
	}
	return &createCommentOutput{
		Status: http.StatusCreated,
		Body: api.CreateCommentResponse{
			Message:           "Comment created successfully",
			Comment:           comment,
			CommentsCount:     count,
			RequiresModeration: comment.Status == model.CommentStatusPending,
		},
	}, nil
}

func (h *Handler) DeleteComment(ctx context.Context, in *deleteCommentInput) (*commentMutationOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, huma.NewError(401, "Not logged in")
	}
	count, err := h.svc.Delete(ctx, in.ID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrCommentNotFound), errors.Is(err, ErrUserNotFound):
			return nil, huma.NewError(404, err.Error())
		case errors.Is(err, ErrForbidden):
			return nil, huma.NewError(403, "Not authorized to delete this comment")
		default:
			return nil, huma.NewError(500, "Failed to delete comment")
		}
	}
	return &commentMutationOutput{Body: api.CommentMutationResponse{
		Message:       "Comment deleted",
		CommentsCount: count,
	}}, nil
}

func (h *Handler) GetPendingComments(ctx context.Context, in *moderationListInput) (*moderationListOutput, error) {
	return h.listModeration(ctx, model.CommentStatusPending, in.Page, in.Limit)
}

func (h *Handler) GetApprovedComments(ctx context.Context, in *moderationListInput) (*moderationListOutput, error) {
	return h.listModeration(ctx, model.CommentStatusPublished, in.Page, in.Limit)
}

func (h *Handler) GetRejectedComments(ctx context.Context, in *moderationListInput) (*moderationListOutput, error) {
	return h.listModeration(ctx, model.CommentStatusRejected, in.Page, in.Limit)
}

func (h *Handler) listModeration(ctx context.Context, status model.CommentStatus, page, limit int) (*moderationListOutput, error) {
	page, limit = middleware.ParsePaginationValues(strconv.Itoa(page), strconv.Itoa(limit), middleware.DefaultPaginationLimit)
	comments, total, err := h.svc.ListModeration(ctx, status, page, limit)
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch moderation comments")
	}
	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}
	return &moderationListOutput{
		Body: api.ModerationCommentsResponse{
			Comments: comments,
			Pagination: api.Pagination{
				Total: total, Page: page, Limit: limit, TotalPages: totalPages,
			},
		},
	}, nil
}

func (h *Handler) ApproveComment(ctx context.Context, in *moderationActionInput) (*commentMutationOutput, error) {
	return h.setStatus(ctx, in.ID, model.CommentStatusPublished, "Comment approved", "Failed to approve comment")
}

func (h *Handler) RejectComment(ctx context.Context, in *moderationActionInput) (*commentMutationOutput, error) {
	return h.setStatus(ctx, in.ID, model.CommentStatusRejected, "Comment rejected", "Failed to reject comment")
}

func (h *Handler) setStatus(ctx context.Context, id string, status model.CommentStatus, successMsg, failureMsg string) (*commentMutationOutput, error) {
	comment, err := h.svc.SetStatus(ctx, id, status)
	if err != nil {
		if errors.Is(err, ErrCommentNotFound) {
			return nil, huma.NewError(404, "Comment does not exist")
		}
		return nil, huma.NewError(500, failureMsg)
	}
	return &commentMutationOutput{Body: api.CommentMutationResponse{
		Message: successMsg,
		Comment: comment,
	}}, nil
}

func (h *Handler) GetCommentModerationConfig(ctx context.Context, _ *struct{}) (*moderationConfigOutput, error) {
	config, err := h.svc.GetModerationConfig(ctx)
	if err != nil {
		return nil, huma.NewError(500, "Failed to get comment moderation configuration")
	}
	return &moderationConfigOutput{Body: config}, nil
}

func (h *Handler) UpdateCommentModerationConfig(ctx context.Context, in *updateModerationConfigInput) (*updateModerationConfigOutput, error) {
	req := UpdateModerationConfigRequest{}
	if in.Body.ManualReviewEnabled != nil {
		req.ManualReviewEnabled = *in.Body.ManualReviewEnabled
	}
	if in.Body.KeywordFilterEnabled != nil {
		req.KeywordFilterEnabled = *in.Body.KeywordFilterEnabled
	}
	if in.Body.LLMReviewEnabled != nil {
		req.LLMReviewEnabled = *in.Body.LLMReviewEnabled
	}
	req.ModelProvider = in.Body.ModelProvider
	req.ApiKey = in.Body.ApiKey
	req.ApiEndpoint = in.Body.ApiEndpoint
	req.ModelName = in.Body.ModelName
	req.ModerationPrompt = in.Body.ModerationPrompt
	req.BlockKeywords = in.Body.BlockKeywords
	config, err := h.svc.UpdateModerationConfig(ctx, req)
	if err != nil {
		if errors.Is(err, ErrLLMConfigIncomplete) {
			return nil, huma.NewError(400, err.Error())
		}
		slog.Error("failed to update comment moderation configuration", "err", err)
		return nil, huma.NewError(500, "Failed to update comment moderation configuration")
	}
	return &updateModerationConfigOutput{
		Body: api.UpdateCommentModerationConfigResponse{
			Message: "Comment moderation configuration updated successfully",
			Config:  config,
		},
	}, nil
}

func (h *Handler) TestModerationConfig(ctx context.Context, _ *struct{}) (*testModerationOutput, error) {
	result, err := h.svc.TestModerationLLM(ctx)
	if err != nil {
		if errors.Is(err, ErrLLMConfigIncomplete) {
			return nil, huma.NewError(400, err.Error())
		}
		slog.Error("LLM moderation test failed", "err", err)
		return nil, huma.NewError(500, "Failed to test LLM moderation endpoint")
	}
	return &testModerationOutput{
		Body: api.TestModerationResponse{
			Message:  result.Message,
			Response: result.Response,
		},
	}, nil
}

// parseUintAny accepts either a number or a numeric string (the
// frontend sometimes posts strings). Out-of-range or negative
// values are rejected.
func parseUintAny(v any) (uint, error) {
	switch x := v.(type) {
	case float64:
		if x < 1 || x > math.MaxUint32 {
			return 0, errors.New("out of range")
		}
		return uint(x), nil
	case string:
		id64, err := strconv.ParseUint(x, 10, 64)
		if err != nil {
			return 0, err
		}
		return uint(id64), nil
	case int:
		if x < 1 {
			return 0, errors.New("non-positive")
		}
		return uint(x), nil
	case uint:
		return x, nil
	default:
		return 0, errors.New("unsupported type")
	}
}
