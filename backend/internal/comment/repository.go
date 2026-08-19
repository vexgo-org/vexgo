package comment

import (
	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the comment domain.
type Repository interface {
	// Comment CRUD
	ListByPostID(postID string) ([]model.Comment, error)
	Create(comment *model.Comment) error
	Save(comment *model.Comment) error
	Delete(comment *model.Comment) error
	CountByPostID(postID uint) (int64, error)
	FindByID(id string) (*model.Comment, error)

	// User lookups (for notification helpers)
	FindUserByID(id uint) (*model.User, error)
	FindPostByID(id uint) (*model.Post, error)

	// Moderation config
	GetModerationConfig() (model.CommentModerationConfig, error)
	SaveModerationConfig(config *model.CommentModerationConfig) error

	// Moderation queue
	ListModeration(status model.CommentStatus, offset, limit int) ([]model.Comment, int64, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed comment repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListByPostID(postID string) ([]model.Comment, error) {
	var comments []model.Comment
	err := r.db.Where("post_id = ? AND status = ?", postID, model.CommentStatusPublished).
		Preload("User").
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}

func (r *gormRepository) Create(comment *model.Comment) error {
	return r.db.Create(comment).Error
}

func (r *gormRepository) Save(comment *model.Comment) error {
	return r.db.Save(comment).Error
}

func (r *gormRepository) Delete(comment *model.Comment) error {
	return r.db.Delete(comment).Error
}

func (r *gormRepository) CountByPostID(postID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Comment{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *gormRepository) FindByID(id string) (*model.Comment, error) {
	var comment model.Comment
	if err := r.db.First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *gormRepository) FindUserByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) FindPostByID(id uint) (*model.Post, error) {
	var post model.Post
	if err := r.db.First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) GetModerationConfig() (model.CommentModerationConfig, error) {
	var config model.CommentModerationConfig
	if err := r.db.First(&config).Error; err != nil {
		return config, err
	}
	return config, nil
}

func (r *gormRepository) SaveModerationConfig(config *model.CommentModerationConfig) error {
	return r.db.Save(config).Error
}

func (r *gormRepository) ListModeration(status model.CommentStatus, offset, limit int) ([]model.Comment, int64, error) {
	query := r.db.Model(&model.Comment{}).
		Preload("User").
		Preload("Post").
		Where("status = ?", status)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var comments []model.Comment
	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&comments).Error; err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}
