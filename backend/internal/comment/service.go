// Package comment implements the comment and comment-moderation domain.
package comment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler.
var (
	// ErrCommentNotFound means the comment does not exist.
	ErrCommentNotFound = errors.New("comment not found")
	// ErrUserNotFound means the acting user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrForbidden means the acting user may not modify this comment.
	ErrForbidden = errors.New("forbidden")
	// ErrLLMConfigIncomplete means LLM review is enabled (or tested) without
	// a stored API key and endpoint.
	ErrLLMConfigIncomplete = errors.New("LLM review requires an API key and endpoint")
)

// SecretCipher is the seam for encrypting the moderation API key at rest. It
// is implemented by the secrets package and injected so it can be faked
// in tests. A nil cipher means no key is configured: the key is stored as
// plaintext.
type SecretCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(stored string) (string, error)
}

// Deps holds the dependencies required by the comment domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Notifier  Notifier
	Cipher    SecretCipher
}

// Notifier is the seam for creating notifications. It is implemented by the
// notification domain and injected so it can be faked in tests.
type Notifier = model.Notifier

// Service contains the business logic of the comment domain.
type Service struct {
	repo      Repository
	notifier  Notifier
	cipher    SecretCipher
	llmClient *http.Client
}

// NewService creates a comment service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{
		repo:      NewRepository(deps.DB),
		notifier:  deps.Notifier,
		cipher:    deps.Cipher,
		llmClient: &http.Client{Timeout: llmTimeout},
	}
}

// defaultModerationPrompt asks the model for a strict JSON verdict so the
// reply can be parsed mechanically; any other reply counts as a moderation
// failure and holds the comment for manual review.
const defaultModerationPrompt = `You are a comment moderation assistant. Review the comment below ` +
	`for illegal content, personal attacks, spam, or inappropriate material.
Respond with strict JSON only and no other text: {"approved": true, "reason": "short explanation"}.
Set "approved" to false when the comment must be rejected.

Comment content:
{{content}}`

// defaultModerationConfig returns the configuration used when no row exists.
// With every switch off, new comments are published immediately.
func defaultModerationConfig() model.CommentModerationConfig {
	return model.CommentModerationConfig{
		ModelName:        "gpt-3.5-turbo",
		ModerationPrompt: defaultModerationPrompt,
	}
}

// moderationConfig loads the comment moderation configuration, falling back to
// the default values when no row exists. The stored API key is decrypted for
// internal use; an undecryptable key (e.g. after a key rotation) is treated as
// unset with an error naming the setting.
func (s *Service) moderationConfig(ctx context.Context) (model.CommentModerationConfig, error) {
	config, err := s.repo.GetModerationConfig(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultModerationConfig(), nil
		}
		return config, err
	}
	if config.ApiKey != "" && s.cipher != nil {
		decrypted, decErr := s.cipher.Decrypt(config.ApiKey)
		if decErr != nil {
			slog.Error("failed to decrypt stored secret, treating it as unset; please re-save it",
				"setting", "comment_moderation.api_key", "err", decErr)
			config.ApiKey = ""
		} else {
			config.ApiKey = decrypted
		}
	}
	return config, nil
}

// ListByPost returns the published comments of a post, applying author privacy
// filtering for viewers that are neither the author nor an admin.
func (s *Service) ListByPost(
	ctx context.Context,
	postID string,
	currentUserID uint,
	currentUserRole string,
) ([]model.Comment, error) {
	comments, err := s.repo.ListByPostID(ctx, postID)
	if err != nil {
		return nil, err
	}

	// Apply privacy filtering to comment authors
	for i := range comments {
		author := &comments[i].User
		// If not admin and not viewing own comment, apply privacy filtering
		if !model.IsAdmin(currentUserRole) && author.ID != currentUserID {
			auth.FilterUserByPrivacy(author, currentUserID, currentUserRole)
		}
	}

	return comments, nil
}

// CreateRequest carries the fields for creating a comment.
type CreateRequest struct {
	PostID   uint
	UserID   uint
	Content  string
	ParentID *uint
}

// Create creates a comment with its moderation status decided before
// persistence (a single write carries the final status), and notifies the
// post author and (for replies) the parent comment author only when the
// comment is published. It returns the created comment (with author
// preloaded on read paths) and the comment count for the post.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.Comment, int64, error) {
	config, err := s.moderationConfig(ctx)
	if err != nil {
		return nil, 0, err
	}

	comment := model.Comment{
		PostID:  req.PostID,
		Content: req.Content,
		UserID:  req.UserID,
	}
	if req.ParentID != nil {
		comment.ParentID = req.ParentID
	}
	comment.Status, comment.ModerationReason = s.moderationDecision(ctx, req.Content, config)

	if err := s.repo.Create(ctx, &comment); err != nil {
		return nil, 0, err
	}

	// Pending and rejected comments are invisible to their recipients, so
	// they must not generate notifications.
	if comment.Status == model.CommentStatusPublished {
		s.notifyPostAuthor(ctx, req.PostID, req.UserID, req.Content)
		if req.ParentID != nil {
			s.notifyParentAuthor(ctx, *req.ParentID, req.UserID, req.Content)
		}
	}

	count, _ := s.repo.CountByPostID(ctx, req.PostID)
	return &comment, count, nil
}

// moderationDecision runs the moderation pipeline for one new comment and
// returns the final status and reason, short-circuiting on the first
// decision:
//
//  1. keyword filter (if on): a hit rejects the comment, the LLM is not called;
//  2. LLM review (if on): a reject verdict rejects; an approve verdict is
//     held for manual review when that switch is on, else published; any
//     LLM failure fails closed to pending, even with manual review off;
//  3. manual review (if on): the comment is held as pending;
//  4. otherwise the comment is published.
func (s *Service) moderationDecision(
	ctx context.Context,
	content string,
	config model.CommentModerationConfig,
) (model.CommentStatus, string) {
	if config.KeywordFilterEnabled {
		if keyword, hit := matchBlockedKeyword(content, config.BlockKeywords); hit {
			return model.CommentStatusRejected, truncateReason("Contains blocked keyword: " + keyword)
		}
	}

	if config.LLMReviewEnabled {
		verdict, err := s.reviewWithLLM(ctx, content, config)
		if err != nil {
			slog.Warn("LLM moderation failed; holding comment for manual review", "err", err)
			return model.CommentStatusPending, "LLM review failed; held for manual review"
		}
		if !verdict.Approved {
			return model.CommentStatusRejected, truncateReason(verdict.Reason)
		}
		if config.ManualReviewEnabled {
			return model.CommentStatusPending, ""
		}
		return model.CommentStatusPublished, ""
	}

	if config.ManualReviewEnabled {
		return model.CommentStatusPending, ""
	}
	return model.CommentStatusPublished, ""
}

// matchBlockedKeyword reports whether content contains any comma-separated
// blocked keyword, case-insensitively, returning the matched keyword.
func matchBlockedKeyword(content, blockKeywords string) (string, bool) {
	for keyword := range strings.SplitSeq(blockKeywords, ",") {
		keyword = strings.TrimSpace(keyword)
		if keyword != "" && strings.Contains(strings.ToLower(content), strings.ToLower(keyword)) {
			return keyword, true
		}
	}
	return "", false
}

// maxModerationReason is the storage limit of the moderation_reason column
// (model.Comment.ModerationReason, gorm size:500).
const maxModerationReason = 500

// truncateReason caps a moderation reason at the column limit, counting
// runes, so an oversized model reply or keyword cannot break comment
// persistence on strict databases (MySQL/PostgreSQL).
func truncateReason(reason string) string {
	runes := []rune(reason)
	if len(runes) <= maxModerationReason {
		return reason
	}
	return string(runes[:maxModerationReason])
}

// notifyPostAuthor notifies the post author unless they wrote the comment.
func (s *Service) notifyPostAuthor(ctx context.Context, postID, userID uint, content string) {
	post, err := s.repo.FindPostByID(ctx, postID)
	if err != nil {
		return
	}
	// Don't notify the commenter if they are the post author
	if post.AuthorID == userID {
		return
	}
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return
	}
	// Truncate comment content to first 50 characters
	commentContent := content
	if len(commentContent) > 50 {
		commentContent = commentContent[:50] + "..."
	}
	if err := s.notifier.CreateNotification(ctx, model.NotificationInput{
		UserID:        post.AuthorID,
		Type:          model.NotificationTypeComment,
		Title:         "Post Commented",
		Content:       fmt.Sprintf("User \"%s\" commented on your post \"%s\": %s", user.Username, post.Title, commentContent),
		RelatedID:     strconv.FormatUint(uint64(postID), 10),
		RelatedType:   model.NotificationRelatedTypePost,
		RelatedPostID: &postID,
	}); err != nil {
		slog.Warn("failed to create comment notification", "err", err)
	}
}

// notifyParentAuthor notifies the parent comment author unless they wrote the reply.
func (s *Service) notifyParentAuthor(ctx context.Context, parentID, userID uint, content string) {
	parentComment, err := s.repo.FindByID(ctx, fmt.Sprintf("%d", parentID))
	if err != nil {
		return
	}
	// Don't notify the commenter if they are the parent comment author
	if parentComment.UserID == userID {
		return
	}
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return
	}
	// Truncate reply content to first 50 characters
	replyContent := content
	if len(replyContent) > 50 {
		replyContent = replyContent[:50] + "..."
	}
	if err := s.notifier.CreateNotification(ctx, model.NotificationInput{
		UserID:        parentComment.UserID,
		Type:          model.NotificationTypeReply,
		Title:         "Comment Replied",
		Content:       fmt.Sprintf("User \"%s\" replied to your comment: %s", user.Username, replyContent),
		RelatedID:     strconv.FormatUint(uint64(parentID), 10),
		RelatedType:   model.NotificationRelatedTypeComment,
		RelatedPostID: &parentComment.PostID,
	}); err != nil {
		slog.Warn("failed to create reply notification", "err", err)
	}
}

// Delete removes a comment when the acting user is its author or an admin.
// It returns the remaining comment count for the post.
func (s *Service) Delete(ctx context.Context, commentID string, userID uint) (int64, error) {
	comment, err := s.repo.FindByID(ctx, commentID)
	if err != nil {
		return 0, ErrCommentNotFound
	}

	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return 0, ErrUserNotFound
	}

	// Admins or super admins can delete any comment, authors can delete their own comments
	if !model.IsAdmin(user.Role) && comment.UserID != userID {
		return 0, ErrForbidden
	}

	if err := s.repo.Delete(ctx, comment); err != nil {
		return 0, err
	}

	// Return comment count after deletion for frontend sync
	count, _ := s.repo.CountByPostID(ctx, comment.PostID)
	return count, nil
}

// GetModerationConfig returns the moderation configuration with the API key masked.
func (s *Service) GetModerationConfig(ctx context.Context) (model.CommentModerationConfig, error) {
	config, err := s.moderationConfig(ctx)
	if err != nil {
		return config, err
	}

	// Don't return sensitive information like API key
	config.ApiKey = ""
	return config, nil
}

// UpdateModerationConfigRequest carries the fields accepted by the admin API.
type UpdateModerationConfigRequest struct {
	ManualReviewEnabled  bool
	KeywordFilterEnabled bool
	LLMReviewEnabled     bool
	ModelProvider        string
	ApiKey               string
	ApiEndpoint          string
	ModelName            string
	ModerationPrompt     string
	BlockKeywords        string
}

// UpdateModerationConfig creates or updates the comment moderation
// configuration. The API key is only overwritten when a non-empty value is
// provided. Enabling LLM review without a stored or provided API key and
// endpoint is rejected with ErrLLMConfigIncomplete. The returned
// configuration has the API key masked.
func (s *Service) UpdateModerationConfig(
	ctx context.Context,
	req UpdateModerationConfigRequest,
) (model.CommentModerationConfig, error) {
	// Single fetch decides create vs update; any persistence error other than
	// "no row yet" aborts the update so a stored key can never be wiped by a
	// transient read failure.
	raw, getErr := s.repo.GetModerationConfig(ctx)
	isCreate := errors.Is(getErr, gorm.ErrRecordNotFound)
	if getErr != nil && !isCreate {
		return model.CommentModerationConfig{}, fmt.Errorf("load comment moderation config: %w", getErr)
	}

	config := model.CommentModerationConfig{
		ManualReviewEnabled:  req.ManualReviewEnabled,
		KeywordFilterEnabled: req.KeywordFilterEnabled,
		LLMReviewEnabled:     req.LLMReviewEnabled,
		ModelProvider:        req.ModelProvider,
		ApiEndpoint:          req.ApiEndpoint,
		ModelName:            req.ModelName,
		ModerationPrompt:     req.ModerationPrompt,
		BlockKeywords:        req.BlockKeywords,
	}

	// The API key is only overwritten when a non-empty value is provided;
	// otherwise the stored (possibly encrypted) value is kept exactly as read.
	switch {
	case req.ApiKey != "":
		apiKey, encErr := s.encryptSecret(req.ApiKey, "comment_moderation.api_key")
		if encErr != nil {
			return model.CommentModerationConfig{}, encErr
		}
		config.ApiKey = apiKey
	case !isCreate:
		config.ApiKey = raw.ApiKey
	}

	// LLM review needs working credentials: a provided key, or — on update —
	// a previously stored one (a non-empty stored value, encrypted or not).
	if req.LLMReviewEnabled {
		apiKey := req.ApiKey
		if apiKey == "" && !isCreate {
			apiKey = raw.ApiKey
		}
		if apiKey == "" || req.ApiEndpoint == "" {
			return model.CommentModerationConfig{}, ErrLLMConfigIncomplete
		}
	}

	if err := s.repo.SaveModerationConfig(ctx, &config); err != nil {
		return model.CommentModerationConfig{}, fmt.Errorf("save comment moderation config: %w", err)
	}

	// Don't return sensitive information
	config.ApiKey = ""
	return config, nil
}

// encryptSecret encrypts a secret before it is stored. Empty values are
// passed through, and without a configured cipher the plaintext is stored
// as-is (no-key fallback). Errors are wrapped with the setting name so the
// admin can tell which secret failed to save.
func (s *Service) encryptSecret(value, setting string) (string, error) {
	if value == "" || s.cipher == nil {
		return value, nil
	}
	encrypted, err := s.cipher.Encrypt(value)
	if err != nil {
		return "", fmt.Errorf("encrypt %s: %w", setting, err)
	}
	return encrypted, nil
}

// ListModeration returns the paginated comments with the given status
// (pending/published/rejected) for the moderation queue.
func (s *Service) ListModeration(
	ctx context.Context,
	status model.CommentStatus,
	page, limit int,
) ([]model.Comment, int64, error) {
	offset := (page - 1) * limit
	return s.repo.ListModeration(ctx, status, offset, limit)
}

// SetStatus approves or rejects a comment.
func (s *Service) SetStatus(ctx context.Context, id string, status model.CommentStatus) (*model.Comment, error) {
	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCommentNotFound
		}
		return nil, err
	}

	comment.Status = status
	if err := s.repo.Save(ctx, comment); err != nil {
		return nil, err
	}

	return comment, nil
}
