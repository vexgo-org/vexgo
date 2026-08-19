// Package comment implements the comment and comment-moderation domain.
package comment

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"vexgo/backend/internal/auth"
	"vexgo/backend/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	ErrCommentNotFound = errors.New("comment not found")
	ErrUserNotFound    = errors.New("user not found")
	ErrForbidden       = errors.New("forbidden")
)

type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Notifier  Notifier
}

type Notifier = model.Notifier

type Service struct {
	repo     Repository
	notifier Notifier
}

func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), notifier: deps.Notifier}
}

func defaultModerationConfig() model.CommentModerationConfig {
	return model.CommentModerationConfig{
		Enabled:            false,
		ModelName:          "gpt-3.5-turbo",
		ModerationPrompt:   "Please review the following comment for compliance. If the comment contains illegal content, personal attacks, or inappropriate material, return 'REJECT'; if the comment is compliant, return 'APPROVE'. Only return the result, no explanation.\n\nComment content:\n{{content}}",
		AutoApproveEnabled: true,
		MinScoreThreshold:  0.5,
	}
}

func (s *Service) moderationConfig(ctx context.Context) (model.CommentModerationConfig, error) {
	config, err := s.repo.GetModerationConfig(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultModerationConfig(), nil
		}
		return config, err
	}
	return config, nil
}

func (s *Service) ListByPost(ctx context.Context, postID string, currentUserID uint, currentUserRole string) ([]model.Comment, error) {
	comments, err := s.repo.ListByPostID(ctx, postID)
	if err != nil {
		return nil, err
	}

	for i := range comments {
		author := &comments[i].User
		if currentUserRole != model.RoleAdmin && currentUserRole != model.RoleSuperAdmin && author.ID != currentUserID {
			auth.FilterUserByPrivacy(author, currentUserID, currentUserRole)
		}
	}

	return comments, nil
}

func (s *Service) Create(ctx context.Context, postID, userID uint, content string, parentID *uint) (*model.Comment, int64, error) {
	config, err := s.moderationConfig(ctx)
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

	if config.Enabled {
		comment.Status = model.CommentStatusPending
	} else if config.AutoApproveEnabled {
		comment.Status = model.CommentStatusPublished
	} else {
		comment.Status = model.CommentStatusPending
	}

	if err := s.repo.Create(ctx, &comment); err != nil {
		return nil, 0, err
	}

	if config.Enabled {
		approved, _, err := moderateCommentAI(content, config)
		if err != nil {
			logrus.WithError(err).Warn("AI moderation failed, defaulting to published")
			comment.Status = model.CommentStatusPublished
		} else if approved {
			comment.Status = model.CommentStatusPublished
		} else {
			comment.Status = model.CommentStatusRejected
		}

		if err := s.repo.Save(ctx, &comment); err != nil {
			return nil, 0, err
		}
	}

	count, _ := s.repo.CountByPostID(ctx, postID)
	s.repo.FindByID(ctx, fmt.Sprintf("%d", comment.ID))

	s.notifyPostAuthor(ctx, postID, userID, content)
	if parentID != nil {
		s.notifyParentAuthor(ctx, *parentID, userID, content)
	}

	return &comment, count, nil
}

func (s *Service) notifyPostAuthor(ctx context.Context, postID, userID uint, content string) {
	post, err := s.repo.FindPostByID(ctx, postID)
	if err != nil {
		return
	}
	if post.AuthorID == userID {
		return
	}
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return
	}
	commentContent := content
	if len(commentContent) > 50 {
		commentContent = commentContent[:50] + "..."
	}
	if err := s.notifier.CreateNotification(ctx,
		post.AuthorID, "comment", "Post Commented",
		fmt.Sprintf("User \"%s\" commented on your post \"%s\": %s", user.Username, post.Title, commentContent),
		strconv.FormatUint(uint64(postID), 10), "post",
	); err != nil {
		logrus.WithError(err).Warn("failed to create comment notification")
	}
}

func (s *Service) notifyParentAuthor(ctx context.Context, parentID, userID uint, content string) {
	parentComment, err := s.repo.FindByID(ctx, fmt.Sprintf("%d", parentID))
	if err != nil {
		return
	}
	if parentComment.UserID == userID {
		return
	}
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return
	}
	replyContent := content
	if len(replyContent) > 50 {
		replyContent = replyContent[:50] + "..."
	}
	if err := s.notifier.CreateNotification(ctx,
		parentComment.UserID, "reply", "Comment Replied",
		fmt.Sprintf("User \"%s\" replied to your comment: %s", user.Username, replyContent),
		strconv.FormatUint(uint64(parentID), 10), "comment",
	); err != nil {
		logrus.WithError(err).Warn("failed to create reply notification")
	}
}

func (s *Service) Delete(ctx context.Context, commentID string, userID uint) (int64, error) {
	comment, err := s.repo.FindByID(ctx, commentID)
	if err != nil {
		return 0, ErrCommentNotFound
	}

	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return 0, ErrUserNotFound
	}

	if !model.IsAdmin(*user) && comment.UserID != userID {
		return 0, ErrForbidden
	}

	if err := s.repo.Delete(ctx, comment); err != nil {
		return 0, err
	}

	count, _ := s.repo.CountByPostID(ctx, comment.PostID)
	return count, nil
}

func (s *Service) GetModerationConfig(ctx context.Context) (model.CommentModerationConfig, error) {
	config, err := s.moderationConfig(ctx)
	if err != nil {
		return config, err
	}
	config.ApiKey = ""
	return config, nil
}

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

func (s *Service) UpdateModerationConfig(ctx context.Context, req UpdateModerationConfigRequest) (model.CommentModerationConfig, error) {
	config, err := s.moderationConfig(ctx)
	if err != nil {
		return config, err
	}

	_, getErr := s.repo.GetModerationConfig(ctx)
	isCreate := errors.Is(getErr, gorm.ErrRecordNotFound)

	if isCreate {
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
	} else {
		config.Enabled = req.Enabled
		config.ModelProvider = req.ModelProvider
		config.ApiEndpoint = req.ApiEndpoint
		config.ModelName = req.ModelName
		config.ModerationPrompt = req.ModerationPrompt
		config.BlockKeywords = req.BlockKeywords
		config.AutoApproveEnabled = req.AutoApproveEnabled
		config.MinScoreThreshold = req.MinScoreThreshold
		if req.ApiKey != "" {
			config.ApiKey = req.ApiKey
		}
	}

	if err := s.repo.SaveModerationConfig(ctx, &config); err != nil {
		return config, err
	}

	config.ApiKey = ""
	return config, nil
}

func (s *Service) ListModeration(ctx context.Context, status model.CommentStatus, page, limit int) ([]model.Comment, int64, error) {
	offset := (page - 1) * limit
	return s.repo.ListModeration(ctx, status, offset, limit)
}

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

func moderateCommentAI(content string, config model.CommentModerationConfig) (bool, string, error) {
	if !config.Enabled {
		return true, "", nil
	}

	if config.BlockKeywords != "" {
		keywords := strings.SplitSeq(config.BlockKeywords, ",")
		for keyword := range keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" && strings.Contains(strings.ToLower(content), strings.ToLower(keyword)) {
				return false, "Contains blocked keyword: " + keyword, nil
			}
		}
	}

	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, "垃圾") || strings.Contains(lowerContent, "spam") ||
		strings.Contains(lowerContent, "广告") || strings.Contains(lowerContent, "ad") {
		return false, "AI detected non-compliant content", nil
	}

	return true, "", nil
}
