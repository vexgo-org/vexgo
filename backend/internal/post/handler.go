// Package post handles post CRUD, categories, tags, likes, and
// the moderation queue.
package post

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Handler exposes the post domain over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a post HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps)}
}

// ---------- input / output types ----------

type listPostsInput struct {
	Page     int    `query:"page" default:"1"`
	Limit    int    `query:"limit" default:"10"`
	Category string `query:"category" default:""`
	Search   string `query:"search" default:""`
}

type listPostsOutput struct {
	Body api.PostsResponse
}

type postIDPathInput struct {
	ID string `path:"id" doc:"Numeric post ID"`
}

type getPostInput struct {
	Slug string `path:"slug" doc:"Post slug"`
}

type getPostOutput struct {
	Body api.PostResponse
}

type getUserPostsInput struct {
	ID    string `path:"id" doc:"Numeric user ID"`
	Page  int    `query:"page" default:"1"`
	Limit int    `query:"limit" default:"10"`
}

type getMyPostsInput struct {
	Page   int    `query:"page" default:"1"`
	Limit  int    `query:"limit" default:"10"`
	Status string `query:"status" default:""`
}

type getDraftsInput struct {
	Page  int `query:"page" default:"1"`
	Limit int `query:"limit" default:"10"`
}

type createPostInput struct {
	Body api.CreatePostRequest
}

type createPostOutput struct {
	Status int                `status:"201" required:""`
	Body   api.CreatePostResponse
}

type updatePostInput struct {
	Body api.UpdatePostRequest
	ID   string `path:"id" doc:"Numeric post ID"`
}

type messageOutput struct {
	Body api.MessageResponse
}

type categoryIDPathInput struct {
	ID string `path:"id" doc:"Numeric category ID"`
}

type tagIDPathInput struct {
	ID string `path:"id" doc:"Numeric tag ID"`
}

type createCategoryInput struct {
	Body api.CreateCategoryRequest
}

type createCategoryOutput struct {
	Status int                       `status:"201" required:""`
	Body   api.CreateCategoryResponse
}

type createTagInput struct {
	Body api.CreateTagRequest
}

type createTagOutput struct {
	Status int                 `status:"201" required:""`
	Body   api.CreateTagResponse
}

type popularPostsInput struct {
	Limit int `query:"limit" default:"5"`
}

type popularPostsOutput struct {
	Body api.PostsListResponse
}

type latestPostsInput struct {
	Limit int `query:"limit" default:"5"`
}

type latestPostsOutput struct {
	Body api.PostsListResponse
}

type categoriesOutput struct {
	Body api.CategoriesResponse
}

type tagsOutput struct {
	Body api.TagsResponse
}

type moderationListInput struct {
	Page   int    `query:"page" default:"1"`
	Limit  int    `query:"limit" default:"10"`
	Search string `query:"search" default:""`
}

type moderationListOutput struct {
	Body api.PostsResponse
}

type approvePostOutput struct {
	Body api.PostMutationResponse
}

type rejectPostInput struct {
	Body api.RejectPostRequest
	ID   string `path:"id" doc:"Numeric post ID"`
}

type rejectPostOutput struct {
	Body api.PostMutationResponse
}

type resubmitPostOutput struct {
	Body api.PostMutationResponse
}

type likeStatusInput struct {
	PostID string `path:"postId" doc:"Numeric post ID"`
}

type likeStatusOutput struct {
	Body api.LikeStatusResponse
}

type likeToggleOutput struct {
	Body api.LikeToggleResponse
}

type createPostEmptyInput struct{}

// RegisterRoutes registers the post domain operations on the
// given huma.API. Routes are split into three tiers:
//
//   - public (no middleware): browse, search, get, like status
//
//   - authed: create / update / delete / like / my-posts /
//     drafts — anything the JWT middleware protects
//
//   - role-gated: contributor+ for category/tag CRUD, admin
//     for moderation. auth.Permission returns 401 for
//     unauthenticated requests and 403 for the wrong role,
//     matching the legacy gin chain.
func (h *Handler) RegisterRoutes(api huma.API) {
	noGuest := auth.Permission(
		model.RoleContributor, model.RoleAuthor, model.RoleAdmin, model.RoleSuperAdmin,
	)
	adminOnly := auth.Permission(model.RoleAdmin, model.RoleSuperAdmin)

	// Public
	huma.Register(api, huma.Operation{
		OperationID: "list-posts", Method: http.MethodGet, Path: "/posts",
		Summary: "List posts", Tags: []string{"posts"},
	}, h.GetPosts)
	huma.Register(api, huma.Operation{
		OperationID: "get-post-by-slug", Method: http.MethodGet, Path: "/posts/{slug}",
		Summary: "Get a post by slug", Tags: []string{"posts"},
	}, h.GetPost)
	huma.Register(api, huma.Operation{
		OperationID: "get-post-by-id", Method: http.MethodGet, Path: "/posts/by-id/{id}",
		Summary: "Get a post by numeric ID", Tags: []string{"posts"},
	}, h.GetPostByID)
	huma.Register(api, huma.Operation{
		OperationID: "list-categories", Method: http.MethodGet, Path: "/categories",
		Summary: "List categories", Tags: []string{"taxonomy"},
	}, h.GetCategories)
	huma.Register(api, huma.Operation{
		OperationID: "list-tags", Method: http.MethodGet, Path: "/tags",
		Summary: "List tags", Tags: []string{"taxonomy"},
	}, h.GetTags)
	huma.Register(api, huma.Operation{
		OperationID: "popular-posts", Method: http.MethodGet, Path: "/stats/popular-posts",
		Summary: "List popular posts", Tags: []string{"stats"},
	}, h.GetPopularPosts)
	huma.Register(api, huma.Operation{
		OperationID: "latest-posts", Method: http.MethodGet, Path: "/stats/latest-posts",
		Summary: "List latest posts", Tags: []string{"stats"},
	}, h.GetLatestPosts)
	huma.Register(api, huma.Operation{
		OperationID: "get-like-status", Method: http.MethodGet, Path: "/likes/{postId}",
		Summary: "Get like status for a post", Tags: []string{"likes"},
	}, h.GetLikeStatus)
	huma.Register(api, huma.Operation{
		OperationID: "user-posts", Method: http.MethodGet, Path: "/posts/user/{id}",
		Summary: "List posts by a user", Tags: []string{"posts"},
	}, h.GetUserPosts)

	// Authed
	huma.Register(api, huma.Operation{
		OperationID: "create-post", Method: http.MethodPost, Path: "/posts",
		Summary: "Create a post", Tags: []string{"posts"},
		Security: []map[string][]string{{"BearerAuth": {}}},
	}, h.CreatePost)
	huma.Register(api, huma.Operation{
		OperationID: "my-posts", Method: http.MethodGet, Path: "/posts/user/my-posts",
		Summary: "List the current user's posts", Tags: []string{"posts"},
		Security: []map[string][]string{{"BearerAuth": {}}},
	}, h.GetMyPosts)
	huma.Register(api, huma.Operation{
		OperationID: "drafts", Method: http.MethodGet, Path: "/posts/drafts",
		Summary: "List draft posts", Tags: []string{"posts"},
		Security: []map[string][]string{{"BearerAuth": {}}},
	}, h.GetDraftPosts)
	huma.Register(api, huma.Operation{
		OperationID: "update-post", Method: http.MethodPut, Path: "/posts/{id}",
		Summary: "Update a post (author or admin)", Tags: []string{"posts"},
		Security: []map[string][]string{{"BearerAuth": {}}},
	}, h.UpdatePost)
	huma.Register(api, huma.Operation{
		OperationID: "delete-post", Method: http.MethodDelete, Path: "/posts/{id}",
		Summary: "Delete a post (author or admin)", Tags: []string{"posts"},
		Security: []map[string][]string{{"BearerAuth": {}}},
	}, h.DeletePost)
	huma.Register(api, huma.Operation{
		OperationID: "toggle-like", Method: http.MethodPost, Path: "/likes/{postId}",
		Summary: "Like or unlike a post", Tags: []string{"likes"},
		Security: []map[string][]string{{"BearerAuth": {}}},
	}, h.ToggleLike)

	// Role-gated
	huma.Register(api, huma.Operation{
		OperationID: "create-category", Method: http.MethodPost, Path: "/categories",
		Summary: "Create a category (contributor+)", Tags: []string{"taxonomy"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{noGuest},
	}, h.CreateCategory)
	huma.Register(api, huma.Operation{
		OperationID: "create-tag", Method: http.MethodPost, Path: "/tags",
		Summary: "Create a tag (contributor+)", Tags: []string{"taxonomy"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{noGuest},
	}, h.CreateTag)
	huma.Register(api, huma.Operation{
		OperationID: "delete-category", Method: http.MethodDelete, Path: "/categories/{id}",
		Summary: "Delete a category (contributor+)", Tags: []string{"taxonomy"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{noGuest},
	}, h.DeleteCategory)
	huma.Register(api, huma.Operation{
		OperationID: "delete-tag", Method: http.MethodDelete, Path: "/tags/{id}",
		Summary: "Delete a tag (contributor+)", Tags: []string{"taxonomy"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{noGuest},
	}, h.DeleteTag)

	// Admin
	huma.Register(api, huma.Operation{
		OperationID: "list-pending-posts", Method: http.MethodGet, Path: "/moderation/pending",
		Summary: "List pending posts (admin)", Tags: []string{"post-moderation"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.GetPendingPosts)
	huma.Register(api, huma.Operation{
		OperationID: "list-approved-posts", Method: http.MethodGet, Path: "/moderation/approved",
		Summary: "List approved posts (admin)", Tags: []string{"post-moderation"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.GetApprovedPosts)
	huma.Register(api, huma.Operation{
		OperationID: "list-rejected-posts", Method: http.MethodGet, Path: "/moderation/rejected",
		Summary: "List rejected posts (admin)", Tags: []string{"post-moderation"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.GetRejectedPosts)
	huma.Register(api, huma.Operation{
		OperationID: "approve-post", Method: http.MethodPut, Path: "/moderation/approve/{id}",
		Summary: "Approve a post (admin)", Tags: []string{"post-moderation"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.ApprovePost)
	huma.Register(api, huma.Operation{
		OperationID: "reject-post", Method: http.MethodPut, Path: "/moderation/reject/{id}",
		Summary: "Reject a post (admin)", Tags: []string{"post-moderation"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.RejectPost)
	huma.Register(api, huma.Operation{
		OperationID: "resubmit-post", Method: http.MethodPut, Path: "/moderation/resubmit/{id}",
		Summary: "Resubmit a rejected post (admin)", Tags: []string{"post-moderation"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.ResubmitPost)
}

// ---------- handlers ----------

func (h *Handler) GetPosts(ctx context.Context, in *listPostsInput) (*listPostsOutput, error) {
	page, limit := paginationFromIn(in.Page, in.Limit)
	u, _ := auth.UserFromContext(ctx)
	posts, total, err := h.svc.List(ctx, ListQuery{
		UserRole: u.Role, UserID: u.ID,
		Page: page, Limit: limit,
		Category: in.Category, Search: in.Search,
	})
	if err != nil {
		return nil, mapPostListError(err, "Failed to fetch posts")
	}
	return &listPostsOutput{Body: postsResponse(posts, total, page, limit)}, nil
}

func (h *Handler) GetPostByID(ctx context.Context, in *postIDPathInput) (*getPostOutput, error) {
	u, _ := auth.UserFromContext(ctx)
	post, err := h.svc.Get(ctx, in.ID, u.Role, u.ID)
	if err != nil {
		return nil, mapPostGetError(err, in.ID, "")
	}
	return &getPostOutput{Body: api.PostResponse{Post: post}}, nil
}

func (h *Handler) GetPost(ctx context.Context, in *getPostInput) (*getPostOutput, error) {
	u, _ := auth.UserFromContext(ctx)
	post, err := h.svc.GetBySlug(ctx, in.Slug, u.Role, u.ID)
	if err != nil {
		return nil, mapPostGetError(err, "", in.Slug)
	}
	return &getPostOutput{Body: api.PostResponse{Post: post}}, nil
}

func (h *Handler) CreatePost(ctx context.Context, in *createPostInput) (*createPostOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, huma.NewError(403, "Please log in first")
	}
	u, _ := auth.UserFromContext(ctx)
	post, err := h.svc.Create(ctx, u.Role, userID, CreateRequest{
		Slug: in.Body.Slug, Title: in.Body.Title, Content: in.Body.Content,
		Category: in.Body.Category, Tags: in.Body.Tags, Excerpt: in.Body.Excerpt,
		CoverImage: in.Body.CoverImage, Status: model.PostStatus(in.Body.Status),
	})
	if err != nil {
		return nil, mapCreatePostError(err)
	}
	return &createPostOutput{
		Status: http.StatusCreated,
		Body:   api.CreatePostResponse{Message: "Post created successfully", Post: post},
	}, nil
}

func (h *Handler) UpdatePost(ctx context.Context, in *updatePostInput) (*messageOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	_, err := h.svc.Update(ctx, in.ID, userID, UpdateRequest{
		Slug: in.Body.Slug, Title: in.Body.Title, Content: in.Body.Content,
		Category: in.Body.Category, Tags: in.Body.Tags, Excerpt: in.Body.Excerpt,
		CoverImage: in.Body.CoverImage, Status: model.PostStatus(in.Body.Status),
	})
	if err != nil {
		return nil, mapUpdatePostError(err)
	}
	return &messageOutput{
		Body: api.MessageResponse{Message: "Post updated successfully"},
	}, nil
}

func (h *Handler) DeletePost(ctx context.Context, in *postIDPathInput) (*messageOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	if err := h.svc.Delete(ctx, in.ID, userID); err != nil {
		return nil, mapDeletePostError(err)
	}
	return &messageOutput{Body: api.MessageResponse{Message: "Post deleted successfully"}}, nil
}

func (h *Handler) GetMyPosts(ctx context.Context, in *getMyPostsInput) (*listPostsOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	page, limit := paginationFromIn(in.Page, in.Limit)
	posts, total, err := h.svc.MyPosts(ctx, MyPostsQuery{
		UserID: userID, Page: page, Limit: limit, Status: in.Status,
	})
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch posts")
	}
	return &listPostsOutput{Body: postsResponse(posts, total, page, limit)}, nil
}

func (h *Handler) GetDraftPosts(ctx context.Context, in *getDraftsInput) (*listPostsOutput, error) {
	page, limit := paginationFromIn(in.Page, in.Limit)
	u, _ := auth.UserFromContext(ctx)
	posts, total, err := h.svc.Drafts(ctx, DraftsQuery{
		UserRole: u.Role, UserID: u.ID, Page: page, Limit: limit,
	})
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch drafts")
	}
	return &listPostsOutput{Body: postsResponse(posts, total, page, limit)}, nil
}

func (h *Handler) GetUserPosts(ctx context.Context, in *getUserPostsInput) (*listPostsOutput, error) {
	page, limit := paginationFromIn(in.Page, in.Limit)
	u, _ := auth.UserFromContext(ctx)
	posts, total, err := h.svc.UserPosts(ctx, UserPostsQuery{
		UserIDStr: in.ID, CurrentUserRole: u.Role, CurrentUserID: u.ID,
		Page: page, Limit: limit,
	})
	if err != nil {
		if errors.Is(err, ErrBadRequest) {
			return nil, huma.NewError(400, "Invalid user ID")
		}
		return nil, huma.NewError(500, "Failed to fetch posts")
	}
	return &listPostsOutput{Body: postsResponse(posts, total, page, limit)}, nil
}

func (h *Handler) GetPopularPosts(ctx context.Context, in *popularPostsInput) (*popularPostsOutput, error) {
	_, limit := paginationFromIn(1, in.Limit)
	u, _ := auth.UserFromContext(ctx)
	posts, err := h.svc.Popular(ctx, u.Role, limit)
	if err != nil {
		return nil, mapGuestPostListError(err, "Failed to fetch popular posts")
	}
	return &popularPostsOutput{Body: api.PostsListResponse{Posts: posts}}, nil
}

func (h *Handler) GetLatestPosts(ctx context.Context, in *latestPostsInput) (*latestPostsOutput, error) {
	_, limit := paginationFromIn(1, in.Limit)
	u, _ := auth.UserFromContext(ctx)
	posts, err := h.svc.Latest(ctx, u.Role, limit)
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch latest posts")
	}
	return &latestPostsOutput{Body: api.PostsListResponse{Posts: posts}}, nil
}

func (h *Handler) GetCategories(ctx context.Context, _ *createPostEmptyInput) (*categoriesOutput, error) {
	u, _ := auth.UserFromContext(ctx)
	cats, err := h.svc.Categories(ctx, u.Role)
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch categories")
	}
	return &categoriesOutput{Body: api.CategoriesResponse{Categories: cats}}, nil
}

func (h *Handler) GetTags(ctx context.Context, _ *createPostEmptyInput) (*tagsOutput, error) {
	u, _ := auth.UserFromContext(ctx)
	tags, err := h.svc.Tags(ctx, u.Role)
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch tags")
	}
	return &tagsOutput{Body: api.TagsResponse{Tags: tags}}, nil
}

func (h *Handler) CreateCategory(ctx context.Context, in *createCategoryInput) (*createCategoryOutput, error) {
	if l := len(in.Body.Name); l < 1 || l > 100 {
		return nil, huma.NewError(400, "Invalid request payload")
	}
	if l := len(in.Body.Description); l > 500 {
		return nil, huma.NewError(400, "Invalid request payload")
	}
	u, _ := auth.UserFromContext(ctx)
	category, err := h.svc.CreateCategory(ctx, u.Role, in.Body.Name, in.Body.Description)
	if err != nil {
		return nil, mapCategoryMutationError(err, "Failed to create category")
	}
	return &createCategoryOutput{
		Status: http.StatusCreated,
		Body:   api.CreateCategoryResponse{Message: "Category created successfully", Category: category},
	}, nil
}

func (h *Handler) DeleteCategory(ctx context.Context, in *categoryIDPathInput) (*messageOutput, error) {
	id, ok := parseIDParamString(in.ID)
	if !ok {
		return nil, huma.NewError(404, "Category does not exist")
	}
	u, _ := auth.UserFromContext(ctx)
	if err := h.svc.DeleteCategory(ctx, u.Role, id); err != nil {
		return nil, mapCategoryDeleteError(err, "Failed to delete category")
	}
	return &messageOutput{Body: api.MessageResponse{Message: "Category deleted successfully"}}, nil
}

func (h *Handler) CreateTag(ctx context.Context, in *createTagInput) (*createTagOutput, error) {
	if l := len(in.Body.Name); l < 1 || l > 100 {
		return nil, huma.NewError(400, "Invalid request payload")
	}
	u, _ := auth.UserFromContext(ctx)
	tag, err := h.svc.CreateTag(ctx, u.Role, in.Body.Name)
	if err != nil {
		return nil, mapTagMutationError(err, "Failed to create tag")
	}
	return &createTagOutput{
		Status: http.StatusCreated,
		Body:   api.CreateTagResponse{Message: "Tag created successfully", Tag: tag},
	}, nil
}

func (h *Handler) DeleteTag(ctx context.Context, in *tagIDPathInput) (*messageOutput, error) {
	id, ok := parseIDParamString(in.ID)
	if !ok {
		return nil, huma.NewError(404, "Tag does not exist")
	}
	u, _ := auth.UserFromContext(ctx)
	if err := h.svc.DeleteTag(ctx, u.Role, id); err != nil {
		return nil, mapTagDeleteError(err, "Failed to delete tag")
	}
	return &messageOutput{Body: api.MessageResponse{Message: "Tag deleted successfully"}}, nil
}

func (h *Handler) GetPendingPosts(ctx context.Context, in *moderationListInput) (*moderationListOutput, error) {
	return h.listModeration(ctx, model.PostStatusPending, in.Page, in.Limit, in.Search)
}

func (h *Handler) GetApprovedPosts(ctx context.Context, in *moderationListInput) (*moderationListOutput, error) {
	return h.listModeration(ctx, model.PostStatusPublished, in.Page, in.Limit, in.Search)
}

func (h *Handler) GetRejectedPosts(ctx context.Context, in *moderationListInput) (*moderationListOutput, error) {
	return h.listModeration(ctx, model.PostStatusRejected, in.Page, in.Limit, in.Search)
}

func (h *Handler) listModeration(ctx context.Context, status model.PostStatus, page, limit int, search string) (*moderationListOutput, error) {
	page, limit = paginationFromIn(page, limit)
	posts, total, err := h.svc.ListModeration(ctx, ListModerationQuery{
		Status: status, Page: page, Limit: limit, Search: search,
	})
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch moderation posts")
	}
	return &moderationListOutput{Body: postsResponse(posts, total, page, limit)}, nil
}

func (h *Handler) ApprovePost(ctx context.Context, in *postIDPathInput) (*approvePostOutput, error) {
	post, err := h.svc.Approve(ctx, in.ID)
	if err != nil {
		return nil, mapModerateError(err, "Failed to approve post")
	}
	return &approvePostOutput{Body: api.PostMutationResponse{Message: "Post approved", Post: post}}, nil
}

func (h *Handler) RejectPost(ctx context.Context, in *rejectPostInput) (*rejectPostOutput, error) {
	post, err := h.svc.Reject(ctx, in.ID, in.Body.RejectionReason)
	if err != nil {
		return nil, mapModerateError(err, "Failed to reject post")
	}
	return &rejectPostOutput{Body: api.PostMutationResponse{Message: "Post has been rejected", Post: post}}, nil
}

func (h *Handler) ResubmitPost(ctx context.Context, in *postIDPathInput) (*resubmitPostOutput, error) {
	post, err := h.svc.Resubmit(ctx, in.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrPostNotFound):
			return nil, huma.NewError(404, "Post does not exist")
		case errors.Is(err, ErrBadRequest):
			return nil, huma.NewError(400, "Only rejected posts can be resubmitted for moderation")
		default:
			return nil, huma.NewError(500, "Failed to resubmit post")
		}
	}
	return &resubmitPostOutput{Body: api.PostMutationResponse{Message: "Post resubmitted for moderation", Post: post}}, nil
}

func (h *Handler) ToggleLike(ctx context.Context, in *likeStatusInput) (*likeToggleOutput, error) {
	postID := parseIDParamUint(in.PostID)
	userID := auth.UserIDFromContext(ctx)
	isLiked, count, err := h.svc.ToggleLike(ctx, postID, userID)
	if err != nil {
		return nil, huma.NewError(500, "Failed to remove like")
	}
	message := "Liked successfully"
	if !isLiked {
		message = "Like removed"
	}
	return &likeToggleOutput{Body: api.LikeToggleResponse{
		Message: message, PostID: postID, IsLiked: isLiked, LikesCount: int(count),
	}}, nil
}

func (h *Handler) GetLikeStatus(ctx context.Context, in *likeStatusInput) (*likeStatusOutput, error) {
	postID := parseIDParamUint(in.PostID)
	userID := auth.UserIDFromContext(ctx)
	isLiked, count := h.svc.LikeStatus(ctx, postID, userID)
	return &likeStatusOutput{Body: api.LikeStatusResponse{
		PostID: postID, IsLiked: isLiked, LikesCount: int(count),
	}}, nil
}

// ---------- helpers ----------

// paginationFromIn clamps the page/limit inputs to safe values.
func paginationFromIn(page, limit int) (int, int) {
	return middleware.ParsePaginationValues(strconv.Itoa(page), strconv.Itoa(limit), middleware.DefaultPaginationLimit)
}

func postsResponse(posts []model.Post, total int64, page, limit int) api.PostsResponse {
	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}
	return api.PostsResponse{
		Posts: posts,
		Pagination: api.Pagination{
			Total: total, Page: page, Limit: limit, TotalPages: totalPages,
		},
	}
}

func parseIDParamString(id string) (uint, bool) {
	id64, err := strconv.ParseUint(id, 10, strconv.IntSize)
	if err != nil || id64 == 0 {
		return 0, false
	}
	return uint(id64), true
}

func parseIDParamUint(id string) uint {
	id64, _ := strconv.ParseUint(id, 10, 64)
	return uint(id64)
}

func inUseMessage(kind string, count int64) string {
	noun := "posts"
	if count == 1 {
		noun = "post"
	}
	return fmt.Sprintf("%s is used by %d %s", kind, count, noun)
}

func mapPostListError(err error, def string) error {
	if errors.Is(err, ErrGuestViewDenied) {
		return huma.NewError(403, "You must be logged in to view posts")
	}
	return huma.NewError(500, def)
}

func mapGuestPostListError(err error, def string) error {
	if errors.Is(err, ErrGuestViewDenied) {
		return huma.NewError(403, "You must be logged in to view popular posts")
	}
	return huma.NewError(500, def)
}

func mapPostGetError(err error, id, slug string) error {
	switch {
	case errors.Is(err, ErrGuestViewDenied):
		return huma.NewError(403, "You must be logged in to view this post")
	case errors.Is(err, ErrPostNotFound):
		return huma.NewError(404, "Post does not exist")
	default:
		return huma.NewError(500, "Failed to load post")
	}
}

func mapCreatePostError(err error) error {
	switch {
	case errors.Is(err, ErrForbidden):
		return huma.NewError(403, "Insufficient permissions to create a post")
	case errors.Is(err, model.ErrSlugTaken):
		return huma.NewError(409, "Slug is already taken by another post")
	case errors.Is(err, model.ErrEmptySlug), errors.Is(err, model.ErrInvalidSlug), errors.Is(err, model.ErrSlugTooLong):
		return huma.NewError(400, err.Error())
	default:
		return huma.NewError(500, "Failed to create post")
	}
}

func mapUpdatePostError(err error) error {
	switch {
	case errors.Is(err, ErrPostNotFound):
		return huma.NewError(404, "Post does not exist")
	case errors.Is(err, ErrForbidden):
		return huma.NewError(403, "Not authorized to modify this post")
	case errors.Is(err, model.ErrSlugTaken):
		return huma.NewError(409, "Slug is already taken by another post")
	case errors.Is(err, model.ErrEmptySlug), errors.Is(err, model.ErrInvalidSlug), errors.Is(err, model.ErrSlugTooLong):
		return huma.NewError(400, err.Error())
	default:
		return huma.NewError(500, "Failed to update post")
	}
}

func mapDeletePostError(err error) error {
	switch {
	case errors.Is(err, ErrPostNotFound):
		return huma.NewError(404, "Post does not exist")
	case errors.Is(err, ErrForbidden):
		return huma.NewError(403, "Not authorized to delete this post")
	default:
		return huma.NewError(500, "Failed to delete post")
	}
}

func mapCategoryMutationError(err error, def string) error {
	switch {
	case errors.Is(err, ErrBadRequest):
		return huma.NewError(400, "Category name must not be blank")
	case errors.Is(err, ErrForbidden):
		return huma.NewError(403, "Insufficient permissions to create a category")
	case errors.Is(err, ErrDuplicateName):
		return huma.NewError(409, "A category with this name already exists")
	default:
		return huma.NewError(500, def)
	}
}

func mapCategoryDeleteError(err error, def string) error {
	var inUse *InUseError
	switch {
	case errors.Is(err, ErrCategoryNotFound):
		return huma.NewError(404, "Category does not exist")
	case errors.As(err, &inUse):
		return huma.NewError(400, inUseMessage("Category", inUse.Count))
	case errors.Is(err, ErrForbidden):
		return huma.NewError(403, "Insufficient permissions to delete a category")
	default:
		return huma.NewError(500, def)
	}
}

func mapTagMutationError(err error, def string) error {
	switch {
	case errors.Is(err, ErrBadRequest):
		return huma.NewError(400, "Tag name must not be blank")
	case errors.Is(err, ErrForbidden):
		return huma.NewError(403, "Insufficient permissions to create a tag")
	case errors.Is(err, ErrDuplicateName):
		return huma.NewError(409, "A tag with this name already exists")
	default:
		return huma.NewError(500, def)
	}
}

func mapTagDeleteError(err error, def string) error {
	var inUse *InUseError
	switch {
	case errors.Is(err, ErrTagNotFound):
		return huma.NewError(404, "Tag does not exist")
	case errors.As(err, &inUse):
		return huma.NewError(400, inUseMessage("Tag", inUse.Count))
	case errors.Is(err, ErrForbidden):
		return huma.NewError(403, "Insufficient permissions to delete a tag")
	default:
		return huma.NewError(500, def)
	}
}

func mapModerateError(err error, def string) error {
	if errors.Is(err, ErrPostNotFound) {
		return huma.NewError(404, "Post does not exist")
	}
	return huma.NewError(500, def)
}

// Reset slog to avoid unused import warnings when only used in
// future refactors.
var _ = slog.Default
