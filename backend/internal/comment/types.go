// Wire types for the comment domain. Swag reads the JSON
// tags to populate the OpenAPI spec; orval turns the
// generated schemas into TypeScript interfaces.
package comment

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// CommentListResponse is the body of GET /api/comments/post/{id}.
type CommentListResponse struct {
	Comments []model.Comment `json:"comments"`
}

// CreateCommentRequest is the body of POST /api/comments.
// The postId field accepts either a number or a string; the
// service layer normalises the type before persisting.
type CreateCommentRequest struct {
	// TODO: Consider change the type of `PostID` to `uint`.
	PostID   any    `json:"postId" binding:"required" swaggertype:"primitive,integer" example:"42"`
	Content  string `json:"content" binding:"required" maxLength:"100" example:"Great post!"`
	ParentID *uint  `json:"parentId" swaggertype:"primitive,integer" example:"7"`
}

// CreateCommentResponse is the body of POST /api/comments on
// success. `requiresModeration` is true when the comment is
// held in the pending queue (manual review or keyword/LLM
// filter triggered).
type CreateCommentResponse struct {
	Message            string         `json:"message" example:"Comment created successfully"`
	Comment            *model.Comment `json:"comment"`
	CommentsCount      int64          `json:"commentsCount" example:"3"`
	RequiresModeration bool           `json:"requiresModeration" example:"false"`
}

// DeleteCommentResponse is the body of DELETE /api/comments/{id}.
type DeleteCommentResponse struct {
	Message       string `json:"message" example:"Comment deleted"`
	CommentsCount int64  `json:"commentsCount" example:"3"`
}

// UpdateModerationConfigBody is the body of PUT
// /api/moderation/comments/config. Any field can be omitted
// to leave it unchanged; the apiKey field is the only one
// that's read-only-on-return (never echoed back).
type UpdateModerationConfigBody struct {
	ManualReviewEnabled  bool   `json:"manualReviewEnabled" example:"true"`
	KeywordFilterEnabled bool   `json:"keywordFilterEnabled" example:"true"`
	LLMReviewEnabled     bool   `json:"llmReviewEnabled" example:"false"`
	ModelProvider        string `json:"modelProvider" example:"openai"`
	ApiKey               string `json:"apiKey" example:"sk-..."`
	ApiEndpoint          string `json:"apiEndpoint" example:"https://api.openai.com/v1"`
	ModelName            string `json:"modelName" example:"gpt-4o-mini"`
	ModerationPrompt     string `json:"moderationPrompt"`
	BlockKeywords        string `json:"blockKeywords" example:"spam,badword"`
}

// UpdateModerationConfigResponse is the body of PUT
// /api/moderation/comments/config on success. The `config`
// field is the same shape as the GET response; the apiKey
// is masked in the response.
type UpdateModerationConfigResponse struct {
	Message string                        `json:"message" example:"Comment moderation configuration updated successfully"`
	Config  model.CommentModerationConfig `json:"config"`
}

// TestModerationResponse is the body of POST
// /api/moderation/comments/config/test. The `response` field is the
// raw text the configured LLM returned for the test prompt.
type TestModerationResponse struct {
	Message  string `json:"message"`
	Response string `json:"response"`
}

// CommentModerationListResponse is the body of GET
// /api/moderation/comments/{pending,approved,rejected}.
type CommentModerationListResponse struct {
	Comments   []model.Comment `json:"comments"`
	Pagination Pagination      `json:"pagination"`
}

// CommentMessageResponse is the body of PUT
// /api/moderation/comments/{id}/(approve|reject). The
// `comment` field is the updated row.
type CommentMessageResponse struct {
	Message string         `json:"message" example:"Comment approved"`
	Comment *model.Comment `json:"comment"`
}

// Pagination describes a paged result. It is the same shape
// used by the rest of the REST surface.
type Pagination struct {
	Total      int64 `json:"total" example:"42"`
	Page       int   `json:"page" example:"1"`
	Limit      int   `json:"limit" example:"10"`
	TotalPages int   `json:"totalPages" example:"5"`
}
