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
	db *gorm.DB
}

// NewService creates a message service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{db: deps.DB}
}

// List returns the paginated notifications of a user, optionally filtered by
// type and read status.
func (s *Service) List(userID uint, page, limit int, messageType, isRead string) ([]model.Notification, int64, error) {
	offset := (page - 1) * limit

	query := s.db.Model(&model.Notification{}).Where("user_id = ?", userID)

	// Type filter
	if messageType != "" {
		query = query.Where("type = ?", messageType)
	}

	// Read status filter
	if isRead != "" {
		readStatus := isRead == "true"
		query = query.Where("is_read = ?", readStatus)
	}

	// Calculate total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Query messages
	var notifications []model.Notification
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&notifications).Error; err != nil {
		return nil, 0, err
	}

	return notifications, total, nil
}

// MarkAsRead marks a single notification as read. It returns the number of
// rows affected (0 when the notification does not belong to the user).
func (s *Service) MarkAsRead(userID uint, id int) (int64, error) {
	result := s.db.Model(&model.Notification{}).Where("id = ? AND user_id = ?", id, userID).Update("is_read", true)
	return result.RowsAffected, result.Error
}

// MarkAllAsRead marks all notifications of a user as read.
func (s *Service) MarkAllAsRead(userID uint) error {
	return s.db.Model(&model.Notification{}).Where("user_id = ?", userID).Update("is_read", true).Error
}

// Delete removes a notification owned by the user. It returns the number of
// rows affected (0 when the notification does not belong to the user).
func (s *Service) Delete(userID uint, id int) (int64, error) {
	result := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Notification{})
	return result.RowsAffected, result.Error
}

// UnreadCount returns the number of unread notifications of a user.
func (s *Service) UnreadCount(userID uint) (int64, error) {
	var count int64
	err := s.db.Model(&model.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&count).Error
	return count, err
}

// CreateNotification creates a notification for a user. It is called by other
// domains (post, comment, user) when an event of interest occurs.
func (s *Service) CreateNotification(userID uint, notificationType, title, content, relatedID, relatedType string) error {
	notification := model.Notification{
		UserID:      userID,
		Type:        notificationType,
		Title:       title,
		Content:     content,
		RelatedID:   relatedID,
		RelatedType: relatedType,
		IsRead:      false,
	}

	return s.db.Create(&notification).Error
}
