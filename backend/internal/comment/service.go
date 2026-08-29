// Package comment implements the comment and comment-moderation domain.
package comment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	repo     Repository
	notifier Notifier
	cipher   SecretCipher
}

// NewService creates a comment service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), notifier: deps.Notifier, cipher: deps.Cipher}
}

// defaultModerationPrompt is used by the moderation checker when no custom
// prompt is configured.
const defaultModerationPrompt = "Please review the following comment for compliance. " +
	"If the comment contains illegal content, personal attacks, or inappropriate material, " +
	"return 'REJECT'; if the comment is compliant, return 'APPROVE'. " +
	"Only return the result, no explanation.\n\nComment content:\n{{content}}"

// defaultModerationConfig returns the configuration used when no row exists.
func defaultModerationConfig() model.CommentModerationConfig {
	return model.CommentModerationConfig{
		Enabled:            false,
		ModelName:          "gpt-3.5-turbo",
		ModerationPrompt:   defaultModerationPrompt,
		AutoApproveEnabled: true,
		MinScoreThreshold:  0.5,
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

// Create creates a comment, applies moderation, and notifies the post author
// and (for replies) the parent comment author. It returns the created comment
// (with author preloaded) and the published comment count for the post.
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

	// Set comment status
	if config.Enabled {
		// If AI moderation enabled, set to pending status first
		comment.Status = model.CommentStatusPending
	} else if config.AutoApproveEnabled {
		// If AI moderation not enabled, decide whether to auto-approve based on config
		comment.Status = model.CommentStatusPublished
	} else {
		// Still requires manual moderation
		comment.Status = model.CommentStatusPending
	}

	if err := s.repo.Create(ctx, &comment); err != nil {
		return nil, 0, err
	}

	// If AI moderation enabled, perform moderation
	if config.Enabled {
		approved, _, err := moderateCommentAI(req.Content, config)
		if err != nil {
			// If AI moderation fails, log error but don't affect comment creation
			slog.Warn("AI moderation failed, defaulting to published", "err", err)
			comment.Status = model.CommentStatusPublished
		} else if approved {
			comment.Status = model.CommentStatusPublished
		} else {
			comment.Status = model.CommentStatusRejected
		}

		// Update comment status
		if err := s.repo.Save(ctx, &comment); err != nil {
			return nil, 0, err
		}
	}

	// Return created comment and updated comment count
	count, _ := s.repo.CountByPostID(ctx, req.PostID)

	// Create notifications
	s.notifyPostAuthor(ctx, req.PostID, req.UserID, req.Content)
	if req.ParentID != nil {
		s.notifyParentAuthor(ctx, *req.ParentID, req.UserID, req.Content)
	}

	return &comment, count, nil
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
	Enabled            bool
	ModelProvider      string
	ApiKey             string
	ApiEndpoint        string
	ModelName          string
	ModerationPrompt   string
	BlockKeywords      string
	AutoApproveEnabled bool
	MinScoreThreshold  float64
}

// UpdateModerationConfig creates or updates the comment moderation
// configuration. The API key is only overwritten when a non-empty value is
// provided. The returned configuration has the API key masked.
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
		Enabled:            req.Enabled,
		ModelProvider:      req.ModelProvider,
		ApiEndpoint:        req.ApiEndpoint,
		ModelName:          req.ModelName,
		ModerationPrompt:   req.ModerationPrompt,
		BlockKeywords:      req.BlockKeywords,
		AutoApproveEnabled: req.AutoApproveEnabled,
		MinScoreThreshold:  req.MinScoreThreshold,
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

// moderateCommentAI performs keyword-based moderation. It is intentionally
// simple; a real AI API call can replace it later.
func moderateCommentAI(content string, config model.CommentModerationConfig) (bool, string, error) {
	if !config.Enabled {
		return true, "", nil // if not enabled, auto approve
	}

	// Check blocked keywords
	if config.BlockKeywords != "" {
		keywords := strings.SplitSeq(config.BlockKeywords, ",")
		for keyword := range keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" && strings.Contains(strings.ToLower(content), strings.ToLower(keyword)) {
				return false, "Contains blocked keyword: " + keyword, nil
			}
		}
	}

	// Simulate AI moderation logic (should be replaced with real AI API call in production)
	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, "垃圾") || strings.Contains(lowerContent, "spam") ||
		strings.Contains(lowerContent, "广告") || strings.Contains(lowerContent, "ad") {
		return false, "AI detected non-compliant content", nil
	}

	return true, "", nil
}
