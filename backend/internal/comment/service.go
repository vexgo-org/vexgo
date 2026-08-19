// Package comment implements the comment and comment-moderation domain.
package comment

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"vexgo/backend/internal/auth"
	"vexgo/backend/internal/model"

	"github.com/sirupsen/logrus"
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

// Deps holds the dependencies required by the comment domain.
type Deps struct {
	DB       *gorm.DB
	Notifier Notifier
}

// Notifier is the seam for creating notifications. It is implemented by the
// message domain and injected so it can be faked in tests.
type Notifier interface {
	CreateNotification(userID uint, notificationType, title, content, relatedID, relatedType string) error
}

// Service contains the business logic of the comment domain.
type Service struct {
	db       *gorm.DB
	notifier Notifier
}

// NewService creates a comment service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{db: deps.DB, notifier: deps.Notifier}
}

// defaultModerationConfig returns the configuration used when no row exists.
func defaultModerationConfig() model.CommentModerationConfig {
	return model.CommentModerationConfig{
		Enabled:            false,
		ModelProvider:      "",
		ApiKey:             "",
		ApiEndpoint:        "",
		ModelName:          "gpt-3.5-turbo",
		ModerationPrompt:   "Please review the following comment for compliance. If the comment contains illegal content, personal attacks, or inappropriate material, return 'REJECT'; if the comment is compliant, return 'APPROVE'. Only return the result, no explanation.\n\nComment content:\n{{content}}",
		BlockKeywords:      "",
		AutoApproveEnabled: true,
		MinScoreThreshold:  0.5,
	}
}

// moderationConfig loads the comment moderation configuration, falling back to
// the default values when no row exists.
func (s *Service) moderationConfig() (model.CommentModerationConfig, error) {
	var config model.CommentModerationConfig
	if err := s.db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return defaultModerationConfig(), nil
		}
		return config, err
	}
	return config, nil
}

// ListByPost returns the published comments of a post, applying author privacy
// filtering for viewers that are neither the author nor an admin.
func (s *Service) ListByPost(postID string, currentUserID uint, currentUserRole string) ([]model.Comment, error) {
	var comments []model.Comment
	if err := s.db.Where("post_id = ? AND status = ?", postID, model.CommentStatusPublished).
		Preload("User").
		Order("created_at ASC").
		Find(&comments).Error; err != nil {
		return nil, err
	}

	// Apply privacy filtering to comment authors
	for i := range comments {
		author := &comments[i].User
		// If not admin and not viewing own comment, apply privacy filtering
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && author.ID != currentUserID {
			auth.FilterUserByPrivacy(author, currentUserID, currentUserRole)
		}
	}

	return comments, nil
}

// Create creates a comment, applies moderation, and notifies the post author
// and (for replies) the parent comment author. It returns the created comment
// (with author preloaded) and the published comment count for the post.
func (s *Service) Create(postID, userID uint, content string, parentID *uint) (*model.Comment, int64, error) {
	config, err := s.moderationConfig()
	if err != nil {
		return nil, 0, err
	}

	comment := model.Comment{
		PostID:  postID,
		Content: content,
		UserID:  userID,
	}
	if parentID != nil {
		comment.ParentID = parentID
	}

	// Set comment status
	if config.Enabled {
		// If AI moderation enabled, set to pending status first
		comment.Status = model.CommentStatusPending
	} else {
		// If AI moderation not enabled, decide whether to auto-approve based on config
		if config.AutoApproveEnabled {
			comment.Status = model.CommentStatusPublished
		} else {
			comment.Status = model.CommentStatusPending // still requires manual moderation
		}
	}

	if err := s.db.Create(&comment).Error; err != nil {
		return nil, 0, err
	}

	// If AI moderation enabled, perform moderation
	if config.Enabled {
		approved, _, err := moderateCommentAI(content, config)
		if err != nil {
			// If AI moderation fails, log error but don't affect comment creation
			logrus.WithError(err).Warn("AI moderation failed, defaulting to published")
			comment.Status = model.CommentStatusPublished // default to published on failure
		} else {
			if approved {
				comment.Status = model.CommentStatusPublished
			} else {
				comment.Status = model.CommentStatusRejected
			}
		}

		// Update comment status
		if err := s.db.Save(&comment).Error; err != nil {
			return nil, 0, err
		}
	}

	// Return created comment and updated comment count
	var count int64
	s.db.Model(&model.Comment{}).Where("post_id = ? AND status = ?", postID, model.CommentStatusPublished).Count(&count)

	// Preload author information
	s.db.Preload("User").First(&comment, comment.ID)

	// Create notifications
	s.notifyPostAuthor(postID, userID, content)
	if parentID != nil {
		s.notifyParentAuthor(*parentID, userID, content)
	}

	return &comment, count, nil
}

// notifyPostAuthor notifies the post author unless they wrote the comment.
func (s *Service) notifyPostAuthor(postID, userID uint, content string) {
	var post model.Post
	if err := s.db.First(&post, postID).Error; err != nil {
		return
	}
	if post.AuthorID == userID {
		// Don't notify the commenter if they are the post author
		return
	}
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return
	}
	// Truncate comment content to first 50 characters
	commentContent := content
	if len(commentContent) > 50 {
		commentContent = commentContent[:50] + "..."
	}
	if err := s.notifier.CreateNotification(
		post.AuthorID,
		"comment",
		"Post Commented",
		fmt.Sprintf("User \"%s\" commented on your post \"%s\": %s", user.Username, post.Title, commentContent),
		strconv.FormatUint(uint64(postID), 10),
		"post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create comment notification")
	}
}

// notifyParentAuthor notifies the parent comment author unless they wrote the reply.
func (s *Service) notifyParentAuthor(parentID, userID uint, content string) {
	var parentComment model.Comment
	if err := s.db.First(&parentComment, parentID).Error; err != nil {
		return
	}
	if parentComment.UserID == userID {
		// Don't notify the commenter if they are the parent comment author
		return
	}
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return
	}
	// Truncate reply content to first 50 characters
	replyContent := content
	if len(replyContent) > 50 {
		replyContent = replyContent[:50] + "..."
	}
	if err := s.notifier.CreateNotification(
		parentComment.UserID,
		"reply",
		"Comment Replied",
		fmt.Sprintf("User \"%s\" replied to your comment: %s", user.Username, replyContent),
		strconv.FormatUint(uint64(parentID), 10),
		"comment",
	); err != nil {
		logrus.WithError(err).Warn("failed to create reply notification")
	}
}

// Delete removes a comment when the acting user is its author or an admin.
// It returns the remaining comment count for the post.
func (s *Service) Delete(commentID string, userID uint) (int64, error) {
	var comment model.Comment
	if err := s.db.First(&comment, commentID).Error; err != nil {
		return 0, ErrCommentNotFound
	}

	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return 0, ErrUserNotFound
	}

	// Admins or super admins can delete any comment, authors can delete their own comments
	if !model.IsAdmin(user) && comment.UserID != userID {
		return 0, ErrForbidden
	}

	if err := s.db.Delete(&comment).Error; err != nil {
		return 0, err
	}

	// Return comment count after deletion for frontend sync
	var count int64
	s.db.Model(&model.Comment{}).Where("post_id = ?", comment.PostID).Count(&count)

	return count, nil
}

// GetModerationConfig returns the moderation configuration with the API key masked.
func (s *Service) GetModerationConfig() (model.CommentModerationConfig, error) {
	config, err := s.moderationConfig()
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
func (s *Service) UpdateModerationConfig(req UpdateModerationConfigRequest) (model.CommentModerationConfig, error) {
	var config model.CommentModerationConfig
	if err := s.db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new configuration
			config = model.CommentModerationConfig{
				Enabled:            req.Enabled,
				ModelProvider:      req.ModelProvider,
				ApiKey:             req.ApiKey,
				ApiEndpoint:        req.ApiEndpoint,
				ModelName:          req.ModelName,
				ModerationPrompt:   req.ModerationPrompt,
				BlockKeywords:      req.BlockKeywords,
				AutoApproveEnabled: req.AutoApproveEnabled,
				MinScoreThreshold:  req.MinScoreThreshold,
			}
			if err := s.db.Create(&config).Error; err != nil {
				return config, err
			}
		} else {
			return config, err
		}
	} else {
		// Update existing configuration
		config.Enabled = req.Enabled
		config.ModelProvider = req.ModelProvider
		config.ApiEndpoint = req.ApiEndpoint
		config.ModelName = req.ModelName
		config.ModerationPrompt = req.ModerationPrompt
		config.BlockKeywords = req.BlockKeywords
		config.AutoApproveEnabled = req.AutoApproveEnabled
		config.MinScoreThreshold = req.MinScoreThreshold

		// Only update if new API key is provided
		if req.ApiKey != "" {
			config.ApiKey = req.ApiKey
		}

		if err := s.db.Save(&config).Error; err != nil {
			return config, err
		}
	}

	// Don't return sensitive information
	config.ApiKey = ""
	return config, nil
}

// ListModeration returns the paginated comments with the given status
// (pending/published/rejected) for the moderation queue.
func (s *Service) ListModeration(status model.CommentStatus, page, limit int) ([]model.Comment, int64, error) {
	query := s.db.Model(&model.Comment{}).
		Preload("User").
		Preload("Post").
		Where("status = ?", status)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var comments []model.Comment
	if err := query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&comments).Error; err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

// SetStatus approves or rejects a comment.
func (s *Service) SetStatus(id string, status model.CommentStatus) (*model.Comment, error) {
	var comment model.Comment
	if err := s.db.First(&comment, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrCommentNotFound
		}
		return nil, err
	}

	comment.Status = status
	if err := s.db.Save(&comment).Error; err != nil {
		return nil, err
	}

	return &comment, nil
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
	// This is just a simple keyword check as an example
	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, "垃圾") || strings.Contains(lowerContent, "spam") ||
		strings.Contains(lowerContent, "广告") || strings.Contains(lowerContent, "ad") {
		return false, "AI detected non-compliant content", nil
	}

	// Simulate AI moderation approval
	return true, "", nil
}
