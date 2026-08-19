// Package message implements the notification/message domain.
package message

import (
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

// newServiceWithRepo creates a message service with an explicit repository
// (useful for tests that want to inject a mock).
func newServiceWithRepo(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns the paginated notifications of a user, optionally filtered by
// type and read status.
func (s *Service) List(userID uint, page, limit int, messageType, isRead string) ([]model.Notification, int64, error) {
	offset := (page - 1) * limit
	return s.repo.List(userID, offset, limit, messageType, isRead)
}

// MarkAsRead marks a single notification as read. It returns the number of
// rows affected (0 when the notification does not belong to the user).
func (s *Service) MarkAsRead(userID uint, id int) (int64, error) {
	return s.repo.MarkAsRead(userID, id)
}

// MarkAllAsRead marks all notifications of a user as read.
func (s *Service) MarkAllAsRead(userID uint) error {
	return s.repo.MarkAllAsRead(userID)
}

// Delete removes a notification owned by the user. It returns the number of
// rows affected (0 when the notification does not belong to the user).
func (s *Service) Delete(userID uint, id int) (int64, error) {
	return s.repo.Delete(userID, id)
}

// UnreadCount returns the number of unread notifications of a user.
func (s *Service) UnreadCount(userID uint) (int64, error) {
	return s.repo.UnreadCount(userID)
}

// CreateNotification creates a notification for a user. It is called by other
// domains (post, comment, user) when an event of interest occurs.
func (s *Service) CreateNotification(userID uint, notificationType, title, content, relatedID, relatedType string) error {
	n := &model.Notification{
		UserID:      userID,
		Type:        notificationType,
		Title:       title,
		Content:     content,
		RelatedID:   relatedID,
		RelatedType: relatedType,
		IsRead:      false,
	}
	return s.repo.Create(n)
}
