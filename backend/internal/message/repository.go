package message

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the message domain.
type Repository interface {
	List(ctx context.Context, userID uint, offset, limit int, messageType, isRead string) ([]model.Notification, int64, error)
	MarkAsRead(ctx context.Context, userID uint, id int) (int64, error)
	MarkAllAsRead(ctx context.Context, userID uint) error
	Delete(ctx context.Context, userID uint, id int) (int64, error)
	UnreadCount(ctx context.Context, userID uint) (int64, error)
	Create(ctx context.Context, n *model.Notification) error
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed notification repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) List(ctx context.Context, userID uint, offset, limit int, messageType, isRead string) ([]model.Notification, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ?", userID)

	if messageType != "" {
		query = query.Where("type = ?", messageType)
	}
	if isRead != "" {
		query = query.Where("is_read = ?", isRead == "true")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var notifications []model.Notification
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&notifications).Error; err != nil {
		return nil, 0, err
	}

	return notifications, total, nil
}

func (r *gormRepository) MarkAsRead(ctx context.Context, userID uint, id int) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true)
	return result.RowsAffected, result.Error
}

func (r *gormRepository) MarkAllAsRead(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ?", userID).
		Update("is_read", true).Error
}

func (r *gormRepository) Delete(ctx context.Context, userID uint, id int) (int64, error) {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Notification{})
	return result.RowsAffected, result.Error
}

func (r *gormRepository) UnreadCount(ctx context.Context, userID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}

func (r *gormRepository) Create(ctx context.Context, n *model.Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}
