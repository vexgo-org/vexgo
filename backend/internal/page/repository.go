package page

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the page domain.
type Repository interface {
	FindByID(ctx context.Context, id string) (*model.Page, error)
	FindBySlug(ctx context.Context, slug string) (*model.Page, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	SlugExistsExcludeID(ctx context.Context, slug string, excludeID uint) (bool, error)
	Create(ctx context.Context, page *model.Page) error
	Save(ctx context.Context, page *model.Page) error
	Delete(ctx context.Context, page *model.Page) error
	List(ctx context.Context, status, search string, page, limit int) ([]model.Page, int64, error)
	ListNav(ctx context.Context) ([]model.Page, error)
	FindUserByID(ctx context.Context, id uint) (*model.User, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed page repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindByID(ctx context.Context, id string) (*model.Page, error) {
	var page model.Page
	if err := r.db.WithContext(ctx).Preload("Author").First(&page, id).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *gormRepository) FindBySlug(ctx context.Context, slug string) (*model.Page, error) {
	var page model.Page
	if err := r.db.WithContext(ctx).Preload("Author").Where("slug = ?", slug).First(&page).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *gormRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Page{}).Where("slug = ?", slug).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *gormRepository) SlugExistsExcludeID(ctx context.Context, slug string, excludeID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Page{}).Where("slug = ? AND id != ?", slug, excludeID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *gormRepository) Create(ctx context.Context, page *model.Page) error {
	return r.db.WithContext(ctx).Create(page).Error
}

func (r *gormRepository) Save(ctx context.Context, page *model.Page) error {
	return r.db.WithContext(ctx).Save(page).Error
}

func (r *gormRepository) Delete(ctx context.Context, page *model.Page) error {
	return r.db.WithContext(ctx).Delete(page).Error
}

func (r *gormRepository) List(ctx context.Context, status, search string, page, limit int) ([]model.Page, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Page{}).Preload("Author")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		query = query.Where("title LIKE ? OR slug LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pages []model.Page
	if err := query.Order("sort_order ASC, id ASC").Offset((page - 1) * limit).Limit(limit).Find(&pages).Error; err != nil {
		return nil, 0, err
	}
	return pages, total, nil
}

func (r *gormRepository) ListNav(ctx context.Context) ([]model.Page, error) {
	var pages []model.Page
	if err := r.db.WithContext(ctx).
		Where("status = ? AND show_in_nav = ?", model.PageStatusPublished, true).
		Order("sort_order ASC, id ASC").
		Find(&pages).Error; err != nil {
		return nil, err
	}
	return pages, nil
}

func (r *gormRepository) FindUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
