package public

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the public domain. Every query
// the server-side rendered pages need lives here, including the batch counts
// that keep a list page from running one query per post.
type Repository interface {
	// Settings and theme configuration.
	GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error)
	GetThemeConfig(ctx context.Context) (model.ThemeConfig, error)

	// Posts.
	ListPublishedPosts(ctx context.Context, q PostListQuery) ([]model.Post, int64, error)
	FindPublishedPostBySlug(ctx context.Context, slug string) (*model.Post, error)
	ListPublishedCategories(ctx context.Context) ([]string, error)
	ListRecentPublishedPosts(ctx context.Context, limit int) ([]model.Post, error)
	ListPopularTags(ctx context.Context, limit int) ([]PopularTag, error)
	IncrementPostViews(ctx context.Context, id uint) error
	CountLikesByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error)
	CountCommentsByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error)

	// Users and custom pages.
	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	ListNavPages(ctx context.Context) ([]model.Page, error)
	FindPageBySlug(ctx context.Context, slug string, publishedOnly bool) (*model.Page, error)
	FindFirstUserByRole(ctx context.Context, role string) (*model.User, error)
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed public repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	var settings model.GeneralSettings
	if err := r.db.WithContext(ctx).
		Preload("SiteIcon").
		First(&settings).
		Error; err != nil {
		return settings, err
	}
	return settings, nil
}

func (r *gormRepository) GetThemeConfig(ctx context.Context) (model.ThemeConfig, error) {
	var config model.ThemeConfig
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return config, err
	}
	return config, nil
}

func (r *gormRepository) ListPublishedPosts(ctx context.Context, q PostListQuery) ([]model.Post, int64, error) {
	query := r.postBaseQuery(ctx).
		Where("status = ?", model.PostStatusPublished)
	if q.AuthorID != 0 {
		query = query.Where("author_id = ?", q.AuthorID)
	}
	if q.Search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+q.Search+"%", "%"+q.Search+"%")
	}
	if q.Category != "" {
		query = query.Where("category = ?", q.Category)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var posts []model.Post
	if err := query.Order("created_at DESC").
		Offset((q.Page - 1) * q.Limit).
		Limit(q.Limit).
		Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

func (r *gormRepository) FindPublishedPostBySlug(ctx context.Context, slug string) (*model.Post, error) {
	var post model.Post
	if err := r.postBaseQuery(ctx).
		Where("slug = ? AND status = ?", slug, model.PostStatusPublished).
		First(&post).
		Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *gormRepository) ListPublishedCategories(ctx context.Context) ([]string, error) {
	var categories []string
	if err := r.db.WithContext(ctx).
		Model(&model.Post{}).
		Where("status = ? AND category != ''", model.PostStatusPublished).
		Distinct().
		Pluck("category", &categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *gormRepository) ListRecentPublishedPosts(ctx context.Context, limit int) ([]model.Post, error) {
	var posts []model.Post
	if err := r.postBaseQuery(ctx).
		Where("status = ?", model.PostStatusPublished).
		Order("created_at DESC").
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *gormRepository) ListPopularTags(ctx context.Context, limit int) ([]PopularTag, error) {
	var rows []PopularTag
	if err := r.db.WithContext(ctx).Table("tags").
		Select("tags.name, COUNT(*) AS count").
		Joins("JOIN post_tags ON post_tags.tag_id = tags.id").
		Joins("JOIN posts ON posts.id = post_tags.post_id").
		Where("posts.status = ?", model.PostStatusPublished).
		Group("tags.id").
		Order("count DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *gormRepository) IncrementPostViews(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&model.Post{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error
}

func (r *gormRepository) CountLikesByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(postIDs))
	if len(postIDs) == 0 {
		return counts, nil
	}
	type result struct {
		PostID uint
		Count  int64
	}
	var rows []result
	if err := r.db.WithContext(ctx).Model(&model.Like{}).
		Select("post_id, COUNT(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Find(&rows).Error; err != nil {
		return counts, err
	}
	for _, row := range rows {
		counts[row.PostID] = row.Count
	}
	return counts, nil
}

func (r *gormRepository) CountCommentsByPostIDs(ctx context.Context, postIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(postIDs))
	if len(postIDs) == 0 {
		return counts, nil
	}
	type result struct {
		PostID uint
		Count  int64
	}
	var rows []result
	if err := r.db.WithContext(ctx).Model(&model.Comment{}).
		Select("post_id, COUNT(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Find(&rows).Error; err != nil {
		return counts, err
	}
	for _, row := range rows {
		counts[row.PostID] = row.Count
	}
	return counts, nil
}

func (r *gormRepository) FindUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.userBaseQuery(ctx).
		First(&user, id).
		Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) ListNavPages(ctx context.Context) ([]model.Page, error) {
	var pages []model.Page
	if err := r.db.WithContext(ctx).
		Where("status = ? AND show_in_nav = ?", model.PageStatusPublished, true).
		Order("sort_order ASC, id ASC").
		Find(&pages).Error; err != nil {
		return nil, err
	}
	return pages, nil
}

func (r *gormRepository) FindPageBySlug(ctx context.Context, slug string, publishedOnly bool) (*model.Page, error) {
	query := r.db.WithContext(ctx).Where("slug = ?", slug)
	if publishedOnly {
		query = query.Where("status = ?", model.PageStatusPublished)
	}
	var page model.Page
	if err := query.First(&page).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *gormRepository) FindFirstUserByRole(ctx context.Context, role string) (*model.User, error) {
	var user model.User
	if err := r.userBaseQuery(ctx).
		Where("role = ?", role).
		Order("id ASC").
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) postBaseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&model.Post{}).
		Preload("Author").
		Preload("Author.Avatar").
		Preload("CoverImage").
		Preload("Tags")
}

func (r *gormRepository) userBaseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Preload("Avatar")
}
