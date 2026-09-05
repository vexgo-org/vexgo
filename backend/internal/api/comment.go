package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// CommentsListResponse is the body of GET /api/comments/post/{id}.
type CommentsListResponse struct {
	Comments []model.Comment `json:"comments" doc:"Top-level comments for the post"`
}

// CreateCommentRequest is the body of POST /api/comments. PostID
// is intentionally a flexible number-or-string to keep the
// frontend contract (the comment composer posts the post slug in
// some flows and the numeric ID in others).
type CreateCommentRequest struct {
	PostID   any    `json:"postId" required:"" doc:"Numeric or string post ID"`
	Content  string `json:"content" required:"" maxLength:"100" doc:"Comment text (max 100 characters)"`
	ParentID *uint  `json:"parentId,omitempty" doc:"Optional parent comment ID for replies"`
}

// CreateCommentResponse is the body of POST /api/comments.
type CreateCommentResponse struct {
	Message            string         `json:"message" doc:"Human-readable outcome"`
	Comment            *model.Comment `json:"comment" doc:"The persisted comment"`
	CommentsCount      int64          `json:"commentsCount" doc:"Total non-deleted comments on the post after this one"`
	RequiresModeration bool           `json:"requiresModeration" doc:"True when the comment is held in the moderation queue"`
}

// CommentMutationResponse is the body of DELETE /api/comments/{id}
// and the comment status mutations.
type CommentMutationResponse struct {
	Message       string         `json:"message" doc:"Human-readable outcome"`
	Comment       *model.Comment `json:"comment,omitempty" doc:"The comment after the mutation, when applicable"`
	CommentsCount int64          `json:"commentsCount,omitempty" doc:"Total non-deleted comments on the post (delete only)"`
}

// ModerationCommentsResponse is the body of the comment-moderation
// queue list endpoints.
type ModerationCommentsResponse struct {
	Comments   []model.Comment `json:"comments" doc:"Comments in the requested moderation state"`
	Pagination Pagination      `json:"pagination" doc:"Paging metadata"`
}

// UpdateCommentModerationConfigRequest is the body of PUT /api/moderation/comments/config. All fields
// are optional pointers so a partial update can omit any of them.
type UpdateCommentModerationConfigRequest struct {
	ManualReviewEnabled  *bool  `json:"manualReviewEnabled,omitempty" doc:"Hold every new comment for manual approval"`
	KeywordFilterEnabled *bool  `json:"keywordFilterEnabled,omitempty" doc:"Reject comments that match a blocked keyword"`
	LLMReviewEnabled     *bool  `json:"llmReviewEnabled,omitempty" doc:"Use the configured LLM to flag comments"`
	ModelProvider        string `json:"modelProvider,omitempty" doc:"openai|azure|... — selects the LLM client"`
	ApiKey               string `json:"apiKey,omitempty" doc:"LLM API key (encrypted at rest); empty leaves the existing key in place"`
	ApiEndpoint          string `json:"apiEndpoint,omitempty" doc:"Custom LLM endpoint URL"`
	ModelName            string `json:"modelName,omitempty" doc:"Model name (e.g. gpt-3.5-turbo)"`
	ModerationPrompt     string `json:"moderationPrompt,omitempty" doc:"System prompt sent to the LLM"`
	BlockKeywords        string `json:"blockKeywords,omitempty" doc:"Comma-separated blocked keywords"`
}

// UpdateCommentModerationConfigResponse is the body of PUT /api/moderation/comments/config.
type UpdateCommentModerationConfigResponse struct {
	Message string                        `json:"message" doc:"Human-readable outcome"`
	Config  model.CommentModerationConfig `json:"config" doc:"The updated configuration"`
}

// TestModerationResponse is the body of POST /api/moderation/comments/config/test.
type TestModerationResponse struct {
	Message  string `json:"message" doc:"Human-readable outcome"`
	Response string `json:"response" doc:"Raw LLM response (or summary)"`
}
