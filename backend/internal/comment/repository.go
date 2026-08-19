package comment

import (
	"context"

	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the comment domain.
type Repository interface {
	ListByPostID(ctx context.Context, postID string) ([]model.Comment, error)
	Create(ctx context.Context, comment *model.Comment) error
	Save(ctx context.Context, comment *model.Comment) error
	Delete(ctx context.Context, comment *model.Comment) error
	CountByPostID(ctx context.Context, postID uint) (int64, error)
	FindByID(ctx context.Context, id string) (*model.Comment, error)
	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	FindPostByID(ctx context.Context, id uint) (*model.Post, error)
	GetModerationConfig(ctx context.Context) (model.CommentModerationConfig, error)
	SaveModerationConfig(ctx context.Context, config *model.CommentModerationConfig) error
	ListModeration(ctx context.Context, status model.CommentStatus, offset, limit int) ([]model.Comment, int64, error)
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListByPostID(ctx context.Context, postID string) ([]model.Comment, error) {
	var comments []model.Comment
	err := r.db.WithContext(ctx).Where("post_id = ? AND status = ?", postID, model.CommentStatusPublished).
		Preload("User").Order("created_at ASC").Find(&comments).Error
	return comments, err
}

func (r *gormRepository) Create(ctx context.Context, comment *model.Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *gormRepository) Save(ctx context.Context, comment *model.Comment) error {
	return r.db.WithContext(ctx).Save(comment).Error
}

func (r *gormRepository) Delete(ctx context.Context, comment *model.Comment) error {
	return r.db.WithContext(ctx).Delete(comment).Error
}

func (r *gormRepository) CountByPostID(ctx context.Context, postID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Comment{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *gormRepository) FindByID(ctx context.Context, id string) (*model.Comment, error) {
	var comment model.Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *gormRepository) FindUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) FindPostByID(ctx context.Context, id uint) (*model.Post, error) {
	var post model.Post
	if err := r.db.WithContext(ctx).First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) GetModerationConfig(ctx context.Context) (model.CommentModerationConfig, error) {
	var config model.CommentModerationConfig
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return config, err
	}
	return config, nil
}

func (r *gormRepository) SaveModerationConfig(ctx context.Context, config *model.CommentModerationConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *gormRepository) ListModeration(ctx context.Context, status model.CommentStatus, offset, limit int) ([]model.Comment, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Comment{}).
		Preload("User").Preload("Post").Where("status = ?", status)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var comments []model.Comment
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&comments).Error; err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}
