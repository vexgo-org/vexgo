package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// CommentsResponse is the body of GET /api/comments/post/:id and the paged
// comment moderation queues. Moderation lists append a pagination object.
type CommentsResponse struct {
	Comments   []model.Comment `json:"comments"`
	Pagination *Pagination     `json:"pagination,omitempty"`
}

// CommentCreateResponse is the body of POST /api/comments.
type CommentCreateResponse struct {
	Message            string        `json:"message"`
	Comment            model.Comment `json:"comment"`
	CommentsCount      int64         `json:"commentsCount"`
	RequiresModeration bool          `json:"requiresModeration"`
}

// CommentMutationResponse is the body of the comment moderation approve and
// reject actions.
type CommentMutationResponse struct {
	Message string        `json:"message"`
	Comment model.Comment `json:"comment"`
}

// CommentDeleteResponse is the body of DELETE /api/comments/:id.
type CommentDeleteResponse struct {
	Message       string `json:"message"`
	CommentsCount int64  `json:"commentsCount"`
}

// CommentModerationUpdateResponse is the body of PUT
// /api/moderation/comments/config.
type CommentModerationUpdateResponse struct {
	Message string                        `json:"message"`
	Config  model.CommentModerationConfig `json:"config"`
}

// LLMTestResponse is the body of the comment moderation and AI connection
// test endpoints.
type LLMTestResponse struct {
	Message  string `json:"message"`
	Response string `json:"response"`
}

// CreateCommentRequest is the body of POST /api/comments. PostID accepts
// either a numeric post ID or its string form.
type CreateCommentRequest struct {
	PostID   any    `json:"postId" binding:"required" tstype:"number | string"`
	Content  string `json:"content" binding:"required"`
	ParentID *uint  `json:"parentId,omitempty"`
}

// UpdateCommentModerationConfigRequest is the body of PUT
// /api/moderation/comments/config. An empty API key keeps the stored value.
type UpdateCommentModerationConfigRequest struct {
	ManualReviewEnabled  bool   `json:"manualReviewEnabled"`
	KeywordFilterEnabled bool   `json:"keywordFilterEnabled"`
	LLMReviewEnabled     bool   `json:"llmReviewEnabled"`
	ModelProvider        string `json:"modelProvider"`
	ApiKey               string `json:"apiKey"` // if empty, don't update API key
	ApiEndpoint          string `json:"apiEndpoint"`
	ModelName            string `json:"modelName"`
	ModerationPrompt     string `json:"moderationPrompt"`
	BlockKeywords        string `json:"blockKeywords"`
}
