package post

import (
	"context"

	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the post domain.
type Repository interface {
	// Posts
	FindByID(ctx context.Context, id string) (*model.Post, error)
	FindByIDPreloadTags(ctx context.Context, id string) (*model.Post, error)
	Create(ctx context.Context, post *model.Post) error
	Save(ctx context.Context, post *model.Post) error
	Delete(ctx context.Context, post *model.Post) error
	IncrementViewCount(ctx context.Context, postID uint) error

	BaseQuery(ctx context.Context) *gorm.DB

	// Likes
	CountLikes(ctx context.Context, postID uint) (int64, error)
	FindLike(ctx context.Context, postID, userID uint) (*model.Like, error)
	CreateLike(ctx context.Context, like *model.Like) error
	DeleteLike(ctx context.Context, like *model.Like) error

	// Comments
	CountComments(ctx context.Context, postID uint) (int64, error)
	DeleteCommentsByPostID(ctx context.Context, postID uint) error
	DeleteLikesByPostID(ctx context.Context, postID uint) error

	FindOrCreateTag(ctx context.Context, name string) (*model.Tag, error)
	ReplaceTagsAssociation(ctx context.Context, post *model.Post, tags []model.Tag) error
	ClearTagsAssociation(ctx context.Context, post *model.Post) error
	FindAllTags(ctx context.Context) ([]model.Tag, error)

	FindAllCategories(ctx context.Context) ([]model.Category, error)
	CreateCategory(ctx context.Context, category *model.Category) error

	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	GetGuestViewSetting(ctx context.Context) bool

	ListModeration(ctx context.Context, status model.PostStatus, offset, limit int, search string) ([]model.Post, int64, error)

	// Batch queries to avoid N+1
	BatchCountLikesByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error)
	BatchCountCommentsByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error)
	BatchFindLikedPostIDs(ctx context.Context, postIDs []uint, userID uint) (map[uint]bool, error)
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindByID(ctx context.Context, id string) (*model.Post, error) {
	var post model.Post
	if err := r.db.WithContext(ctx).Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) FindByIDPreloadTags(ctx context.Context, id string) (*model.Post, error) {
	var post model.Post
	if err := r.db.WithContext(ctx).Preload("Tags").First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) Create(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *gormRepository) Save(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Save(post).Error
}

func (r *gormRepository) Delete(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Delete(post).Error
}

func (r *gormRepository) IncrementViewCount(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Model(&model.Post{}).Where("id = ?", postID).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error
}

func (r *gormRepository) BaseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Post{}).Preload("Author").Preload("Tags")
}

func (r *gormRepository) CountLikes(ctx context.Context, postID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Like{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *gormRepository) FindLike(ctx context.Context, postID, userID uint) (*model.Like, error) {
	var like model.Like
	if err := r.db.WithContext(ctx).Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error; err != nil {
		return nil, err
	}
	return &like, nil
}

func (r *gormRepository) CreateLike(ctx context.Context, like *model.Like) error {
	return r.db.WithContext(ctx).Create(like).Error
}

func (r *gormRepository) DeleteLike(ctx context.Context, like *model.Like) error {
	return r.db.WithContext(ctx).Delete(like).Error
}

func (r *gormRepository) CountComments(ctx context.Context, postID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Comment{}).Where("post_id = ?", postID).Count(&count).Error
	return count, err
}

func (r *gormRepository) DeleteCommentsByPostID(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Where("post_id = ?", postID).Delete(&model.Comment{}).Error
}

func (r *gormRepository) DeleteLikesByPostID(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Where("post_id = ?", postID).Delete(&model.Like{}).Error
}

func (r *gormRepository) FindOrCreateTag(ctx context.Context, name string) (*model.Tag, error) {
	var tag model.Tag
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			tag = model.Tag{Name: name}
			if err := r.db.WithContext(ctx).Create(&tag).Error; err != nil {
				return nil, err
			}
			return &tag, nil
		}
		return nil, err
	}
	return &tag, nil
}

func (r *gormRepository) ReplaceTagsAssociation(ctx context.Context, post *model.Post, tags []model.Tag) error {
	return r.db.WithContext(ctx).Model(post).Association("Tags").Replace(tags)
}

func (r *gormRepository) ClearTagsAssociation(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Model(post).Association("Tags").Clear()
}

func (r *gormRepository) FindAllTags(ctx context.Context) ([]model.Tag, error) {
	var tags []model.Tag
	if err := r.db.WithContext(ctx).Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *gormRepository) FindAllCategories(ctx context.Context) ([]model.Category, error) {
	var categories []model.Category
	if err := r.db.WithContext(ctx).Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *gormRepository) CreateCategory(ctx context.Context, category *model.Category) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *gormRepository) FindUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) GetGuestViewSetting(ctx context.Context) bool {
	var config model.GeneralSettings
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return true
	}
	return config.AllowGuestViewPosts
}

func (r *gormRepository) ListModeration(ctx context.Context, status model.PostStatus, offset, limit int, search string) ([]model.Post, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Post{}).
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

func (r *gormRepository) BatchCountLikesByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error) {
	if len(postIDs) == 0 {
		return make(map[uint]int64), nil
	}
	type result struct {
		PostID uint
		Count  int64
	}
	var results []result
	err := r.db.WithContext(ctx).Model(&model.Like{}).
		Select("post_id, COUNT(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[uint]int64, len(postIDs))
	for _, r := range results {
		counts[r.PostID] = r.Count
	}
	return counts, nil
}

func (r *gormRepository) BatchCountCommentsByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error) {
	if len(postIDs) == 0 {
		return make(map[uint]int64), nil
	}
	type result struct {
		PostID uint
		Count  int64
	}
	var results []result
	err := r.db.WithContext(ctx).Model(&model.Comment{}).
		Select("post_id, COUNT(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[uint]int64, len(postIDs))
	for _, r := range results {
		counts[r.PostID] = r.Count
	}
	return counts, nil
}

func (r *gormRepository) BatchFindLikedPostIDs(ctx context.Context, postIDs []uint, userID uint) (map[uint]bool, error) {
	if len(postIDs) == 0 || userID == 0 {
		return make(map[uint]bool), nil
	}
	var likedPostIDs []uint
	err := r.db.WithContext(ctx).Model(&model.Like{}).
		Where("post_id IN ? AND user_id = ?", postIDs, userID).
		Pluck("post_id", &likedPostIDs).Error
	if err != nil {
		return nil, err
	}
	liked := make(map[uint]bool, len(likedPostIDs))
	for _, id := range likedPostIDs {
		liked[id] = true
	}
	return liked, nil
}
