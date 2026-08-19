package post

import (
	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the post domain.
type Repository interface {
	// Posts
	FindByID(id string) (*model.Post, error)
	FindByIDPreloadTags(id string) (*model.Post, error)
	Create(post *model.Post) error
	Save(post *model.Post) error
	Delete(post *model.Post) error
	IncrementViewCount(postID uint) error

	// Query helpers used by the service for complex queries
	BaseQuery() *gorm.DB

	// Likes
	CountLikes(postID uint) (int64, error)
	FindLike(postID, userID uint) (*model.Like, error)
	CreateLike(like *model.Like) error
	DeleteLike(like *model.Like) error

	// Comments
	CountComments(postID uint) (int64, error)
	DeleteCommentsByPostID(postID uint) error

	// Likes cascade
	DeleteLikesByPostID(postID uint) error

	// Tags
	FindOrCreateTag(name string) (*model.Tag, error)
	ReplaceTagsAssociation(post *model.Post, tags []model.Tag) error
	ClearTagsAssociation(post *model.Post) error
	FindAllTags() ([]model.Tag, error)

	// Categories
	FindAllCategories() ([]model.Category, error)
	CreateCategory(category *model.Category) error

	// Users (for permission checks)
	FindUserByID(id uint) (*model.User, error)

	// Settings
	GetGuestViewSetting() bool

	// Moderation
	ListModeration(status model.PostStatus, offset, limit int, search string) ([]model.Post, int64, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed post repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindByID(id string) (*model.Post, error) {
	var post model.Post
	if err := r.db.Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) FindByIDPreloadTags(id string) (*model.Post, error) {
	var post model.Post
	if err := r.db.Preload("Tags").First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) Create(post *model.Post) error {
	return r.db.Create(post).Error
}

func (r *gormRepository) Save(post *model.Post) error {
	return r.db.Save(post).Error
}

func (r *gormRepository) Delete(post *model.Post) error {
	return r.db.Delete(post).Error
}

func (r *gormRepository) IncrementViewCount(postID uint) error {
	return r.db.Model(&model.Post{}).Where("id = ?", postID).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error
}

func (r *gormRepository) BaseQuery() *gorm.DB {
	return r.db.Model(&model.Post{}).Preload("Author").Preload("Tags")
}

func (r *gormRepository) CountLikes(postID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Like{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *gormRepository) FindLike(postID, userID uint) (*model.Like, error) {
	var like model.Like
	if err := r.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error; err != nil {
		return nil, err
	}
	return &like, nil
}

func (r *gormRepository) CreateLike(like *model.Like) error {
	return r.db.Create(like).Error
}

func (r *gormRepository) DeleteLike(like *model.Like) error {
	return r.db.Delete(like).Error
}

func (r *gormRepository) CountComments(postID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Comment{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *gormRepository) DeleteCommentsByPostID(postID uint) error {
	return r.db.Where("post_id = ?", postID).Delete(&model.Comment{}).Error
}

func (r *gormRepository) DeleteLikesByPostID(postID uint) error {
	return r.db.Where("post_id = ?", postID).Delete(&model.Like{}).Error
}

func (r *gormRepository) FindOrCreateTag(name string) (*model.Tag, error) {
	var tag model.Tag
	if err := r.db.Where("name = ?", name).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			tag = model.Tag{Name: name}
			if err := r.db.Create(&tag).Error; err != nil {
				return nil, err
			}
			return &tag, nil
		}
		return nil, err
	}
	return &tag, nil
}

func (r *gormRepository) ReplaceTagsAssociation(post *model.Post, tags []model.Tag) error {
	return r.db.Model(post).Association("Tags").Replace(tags)
}

func (r *gormRepository) ClearTagsAssociation(post *model.Post) error {
	return r.db.Model(post).Association("Tags").Clear()
}

func (r *gormRepository) FindAllTags() ([]model.Tag, error) {
	var tags []model.Tag
	if err := r.db.Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *gormRepository) FindAllCategories() ([]model.Category, error) {
	var categories []model.Category
	if err := r.db.Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *gormRepository) CreateCategory(category *model.Category) error {
	return r.db.Create(category).Error
}

func (r *gormRepository) FindUserByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) GetGuestViewSetting() bool {
	var config model.GeneralSettings
	if err := r.db.First(&config).Error; err != nil {
		return true
	}
	return config.AllowGuestViewPosts
}

func (r *gormRepository) ListModeration(status model.PostStatus, offset, limit int, search string) ([]model.Post, int64, error) {
	query := r.db.Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", status)

	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Joins("LEFT JOIN users ON posts.author_id = users.id").
			Where("posts.title LIKE ? OR posts.content LIKE ? OR users.username LIKE ?", searchTerm, searchTerm, searchTerm)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var posts []model.Post
	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, 0, err
	}

	return posts, total, nil
}
