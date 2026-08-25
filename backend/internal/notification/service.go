// Package notification implements the notification domain.
package notification

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Deps holds the dependencies required by the notification domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
}

// Service contains the business logic of the notification domain.
type Service struct {
	repo Repository
}

// NewService creates a notification service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB)}
}

// newServiceWithRepo creates a notification service with an explicit repository.
func newServiceWithRepo(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListQuery carries the pagination and filter parameters for List.
type ListQuery struct {
	UserID           uint
	Page             int
	Limit            int
	NotificationType model.NotificationType
	IsRead           string
}

// List returns the paginated notifications of a user, optionally filtered by
// type and read status.
func (s *Service) List(ctx context.Context, q ListQuery) ([]model.Notification, int64, error) {
	offset := (q.Page - 1) * q.Limit
	return s.repo.List(ctx, q.UserID, offset, q.Limit, q.NotificationType, q.IsRead)
}

// MarkAsRead marks a single notification as read. It returns the number of
// rows affected (0 when the notification does not belong to the user).
func (s *Service) MarkAsRead(ctx context.Context, userID uint, id int) (int64, error) {
	return s.repo.MarkAsRead(ctx, userID, id)
}

// MarkAllAsRead marks all notifications of a user as read.
func (s *Service) MarkAllAsRead(ctx context.Context, userID uint) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}

// Delete removes a notification owned by the user. It returns the number of
// rows affected (0 when the notification does not belong to the user).
func (s *Service) Delete(ctx context.Context, userID uint, id int) (int64, error) {
	return s.repo.Delete(ctx, userID, id)
}

// UnreadCount returns the number of unread notifications of a user.
func (s *Service) UnreadCount(ctx context.Context, userID uint) (int64, error) {
	return s.repo.UnreadCount(ctx, userID)
}

// CreateNotification creates a notification for a user. It is called by other
// domains (post, comment, user) when an event of interest occurs.
func (s *Service) CreateNotification(ctx context.Context, input model.NotificationInput) error {
	n := &model.Notification{
		UserID:        input.UserID,
		Type:          input.Type,
		Title:         input.Title,
		Content:       input.Content,
		RelatedID:     input.RelatedID,
		RelatedType:   input.RelatedType,
		RelatedPostID: input.RelatedPostID,
		IsRead:        false,
	}
	return s.repo.Create(ctx, n)
}
