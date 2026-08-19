// Package message implements the notification/message domain.
package message

import (
	"context"

	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Deps holds the dependencies required by the message domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
}

// Service contains the business logic of the message domain.
type Service struct {
	repo Repository
}

// NewService creates a message service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB)}
}

// newServiceWithRepo creates a message service with an explicit repository.
func newServiceWithRepo(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns the paginated notifications of a user, optionally filtered by
// type and read status.
func (s *Service) List(ctx context.Context, userID uint, page, limit int, messageType, isRead string) ([]model.Notification, int64, error) {
	offset := (page - 1) * limit
	return s.repo.List(ctx, userID, offset, limit, messageType, isRead)
}

// MarkAsRead marks a single notification as read.
func (s *Service) MarkAsRead(ctx context.Context, userID uint, id int) (int64, error) {
	return s.repo.MarkAsRead(ctx, userID, id)
}

// MarkAllAsRead marks all notifications of a user as read.
func (s *Service) MarkAllAsRead(ctx context.Context, userID uint) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}

// Delete removes a notification owned by the user.
func (s *Service) Delete(ctx context.Context, userID uint, id int) (int64, error) {
	return s.repo.Delete(ctx, userID, id)
}

// UnreadCount returns the number of unread notifications of a user.
func (s *Service) UnreadCount(ctx context.Context, userID uint) (int64, error) {
	return s.repo.UnreadCount(ctx, userID)
}

// CreateNotification creates a notification for a user.
func (s *Service) CreateNotification(ctx context.Context, userID uint, notificationType, title, content, relatedID, relatedType string) error {
	n := &model.Notification{
		UserID:      userID,
		Type:        notificationType,
		Title:       title,
		Content:     content,
		RelatedID:   relatedID,
		RelatedType: relatedType,
		IsRead:      false,
	}
	return s.repo.Create(ctx, n)
}
