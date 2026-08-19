package user

import (
	"context"
	"fmt"

	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the user domain.
type Repository interface {
	// Users
	FindByID(ctx context.Context, id uint) (*model.User, error)
	UpdateUser(ctx context.Context, user *model.User) error
	UpdateUserRole(ctx context.Context, user *model.User) error
	ListUsers(ctx context.Context, search string, offset, limit int) ([]model.User, int64, error)

	// DeleteUserCascade deletes the user and everything referencing them
	// (comments, likes, media rows, posts and their post_tags join rows) in
	// a single transaction. It returns the URLs of the referenced files so
	// the caller can delete them after the transaction commits.
	DeleteUserCascade(ctx context.Context, userID uint) ([]string, error)

	// Creator applications
	FindPendingApplication(ctx context.Context, userID uint) (*model.CreatorApplication, error)
	CreateApplication(ctx context.Context, app *model.CreatorApplication) error
	FindApplicationByID(ctx context.Context, id uint) (*model.CreatorApplication, error)
	SaveApplication(ctx context.Context, app *model.CreatorApplication) error
	ListApplications(ctx context.Context, status model.CreatorApplicationStatus, offset, limit int) ([]model.CreatorApplication, int64, error)

	// Admins (for notifications)
	FindAdmins(ctx context.Context) ([]model.User, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed user repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) UpdateUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *gormRepository) UpdateUserRole(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Model(user).Select("Role").Updates(user).Error
}

func (r *gormRepository) ListUsers(ctx context.Context, search string, offset, limit int) ([]model.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.User{})
	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("username LIKE ? OR email LIKE ?", searchTerm, searchTerm)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	if err := query.Offset(offset).Limit(limit).Order("id ASC").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// DeleteUserCascade removes the user and all dependent rows inside a single
// transaction, including the post_tags join rows that would otherwise survive
// as orphans. File URLs are collected (not deleted) so the caller can remove
// the files only after the transaction commits.
func (r *gormRepository) DeleteUserCascade(ctx context.Context, userID uint) ([]string, error) {
	var fileURLs []string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Comments and likes authored by the user.
		if err := tx.Where("user_id = ?", userID).Delete(&model.Comment{}).Error; err != nil {
			return fmt.Errorf("delete user comments: %w", err)
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Like{}).Error; err != nil {
			return fmt.Errorf("delete user likes: %w", err)
		}

		// Media files owned by the user.
		var mediaFiles []model.MediaFile
		if err := tx.Where("user_id = ?", userID).Find(&mediaFiles).Error; err != nil {
			return fmt.Errorf("query user media files: %w", err)
		}
		for _, media := range mediaFiles {
			fileURLs = append(fileURLs, media.URL)
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.MediaFile{}).Error; err != nil {
			return fmt.Errorf("delete user media files: %w", err)
		}

		// Posts authored by the user, with all dependents and join rows.
		var posts []model.Post
		if err := tx.Where("author_id = ?", userID).Find(&posts).Error; err != nil {
			return fmt.Errorf("query user posts: %w", err)
		}
		postIDs := make([]uint, 0, len(posts))
		for _, post := range posts {
			postIDs = append(postIDs, post.ID)
			if post.CoverImage != "" {
				fileURLs = append(fileURLs, post.CoverImage)
			}
		}
		if len(postIDs) > 0 {
			if err := tx.Where("post_id IN ?", postIDs).Delete(&model.Comment{}).Error; err != nil {
				return fmt.Errorf("delete post comments: %w", err)
			}
			if err := tx.Where("post_id IN ?", postIDs).Delete(&model.Like{}).Error; err != nil {
				return fmt.Errorf("delete post likes: %w", err)
			}
			if err := tx.Exec("DELETE FROM post_tags WHERE post_id IN ?", postIDs).Error; err != nil {
				return fmt.Errorf("delete post tags: %w", err)
			}
			if err := tx.Where("id IN ?", postIDs).Delete(&model.Post{}).Error; err != nil {
				return fmt.Errorf("delete user posts: %w", err)
			}
		}

		if err := tx.Delete(&model.User{}, userID).Error; err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fileURLs, nil
}

func (r *gormRepository) FindPendingApplication(ctx context.Context, userID uint) (*model.CreatorApplication, error) {
	var app model.CreatorApplication
	if err := r.db.WithContext(ctx).Where("user_id = ? AND status = ?", userID, model.CreatorApplicationStatusPending).
		First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *gormRepository) CreateApplication(ctx context.Context, app *model.CreatorApplication) error {
	return r.db.WithContext(ctx).Create(app).Error
}

func (r *gormRepository) FindApplicationByID(ctx context.Context, id uint) (*model.CreatorApplication, error) {
	var app model.CreatorApplication
	if err := r.db.WithContext(ctx).Preload("User").First(&app, id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *gormRepository) SaveApplication(ctx context.Context, app *model.CreatorApplication) error {
	return r.db.WithContext(ctx).Save(app).Error
}

func (r *gormRepository) ListApplications(ctx context.Context, status model.CreatorApplicationStatus, offset, limit int) ([]model.CreatorApplication, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CreatorApplication{}).Preload("User")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var apps []model.CreatorApplication
	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&apps).Error; err != nil {
		return nil, 0, err
	}

	return apps, total, nil
}

func (r *gormRepository) FindAdmins(ctx context.Context) ([]model.User, error) {
	var admins []model.User
	err := r.db.WithContext(ctx).
		Where("role IN ?", []string{model.RoleAdmin, model.RoleSuperAdmin}).
		Find(&admins).Error
	if err != nil {
		return nil, err
	}
	return admins, nil
}
