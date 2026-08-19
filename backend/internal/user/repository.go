package user

import (
	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the user domain.
type Repository interface {
	// Users
	FindByID(id uint) (*model.User, error)
	UpdateUser(user *model.User) error
	UpdateUserRole(user *model.User) error
	ListUsers(search string, offset, limit int) ([]model.User, int64, error)
	DeleteUser(user *model.User) error

	// Transaction-scoped cascade deletes
	DeleteCommentsByUserIDTx(tx *gorm.DB, userID uint) error
	DeleteLikesByUserIDTx(tx *gorm.DB, userID uint) error
	FindMediaByUserIDTx(tx *gorm.DB, userID uint) ([]model.MediaFile, error)
	DeleteMediaFilesByUserIDTx(tx *gorm.DB, userID uint) error
	FindPostsByAuthorIDTx(tx *gorm.DB, authorID uint) ([]model.Post, error)
	DeleteCommentsByPostIDTx(tx *gorm.DB, postID uint) error
	DeleteLikesByPostIDTx(tx *gorm.DB, postID uint) error
	DeletePostsByAuthorIDTx(tx *gorm.DB, authorID uint) error
	DeleteUserTx(tx *gorm.DB, user *model.User) error

	// Creator applications
	FindPendingApplication(userID uint) (*model.CreatorApplication, error)
	CreateApplication(app *model.CreatorApplication) error
	FindApplicationByID(id uint) (*model.CreatorApplication, error)
	SaveApplication(app *model.CreatorApplication) error
	ListApplications(status model.CreatorApplicationStatus, offset, limit int) ([]model.CreatorApplication, int64, error)

	// Admins (for notifications)
	FindAdmins() ([]model.User, error)

	// Transaction support
	Begin() *gorm.DB
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed user repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) UpdateUser(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *gormRepository) UpdateUserRole(user *model.User) error {
	return r.db.Model(user).Select("Role").Updates(user).Error
}

func (r *gormRepository) ListUsers(search string, offset, limit int) ([]model.User, int64, error) {
	query := r.db.Model(&model.User{})
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

func (r *gormRepository) DeleteUser(user *model.User) error {
	return r.db.Delete(user).Error
}

func (r *gormRepository) DeleteCommentsByUserIDTx(tx *gorm.DB, userID uint) error {
	return tx.Where("user_id = ?", userID).Delete(&model.Comment{}).Error
}

func (r *gormRepository) DeleteLikesByUserIDTx(tx *gorm.DB, userID uint) error {
	return tx.Where("user_id = ?", userID).Delete(&model.Like{}).Error
}

func (r *gormRepository) FindMediaByUserIDTx(tx *gorm.DB, userID uint) ([]model.MediaFile, error) {
	var mediaFiles []model.MediaFile
	if err := tx.Where("user_id = ?", userID).Find(&mediaFiles).Error; err != nil {
		return nil, err
	}
	return mediaFiles, nil
}

func (r *gormRepository) DeleteMediaFilesByUserIDTx(tx *gorm.DB, userID uint) error {
	return tx.Where("user_id = ?", userID).Delete(&model.MediaFile{}).Error
}

func (r *gormRepository) FindPostsByAuthorIDTx(tx *gorm.DB, authorID uint) ([]model.Post, error) {
	var posts []model.Post
	if err := tx.Where("author_id = ?", authorID).Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *gormRepository) DeleteCommentsByPostIDTx(tx *gorm.DB, postID uint) error {
	return tx.Where("post_id = ?", postID).Delete(&model.Comment{}).Error
}

func (r *gormRepository) DeleteLikesByPostIDTx(tx *gorm.DB, postID uint) error {
	return tx.Where("post_id = ?", postID).Delete(&model.Like{}).Error
}

func (r *gormRepository) DeletePostsByAuthorIDTx(tx *gorm.DB, authorID uint) error {
	return tx.Where("author_id = ?", authorID).Delete(&model.Post{}).Error
}

func (r *gormRepository) DeleteUserTx(tx *gorm.DB, user *model.User) error {
	return tx.Delete(user).Error
}

func (r *gormRepository) FindPendingApplication(userID uint) (*model.CreatorApplication, error) {
	var app model.CreatorApplication
	if err := r.db.Where("user_id = ? AND status = ?", userID, model.CreatorApplicationStatusPending).
		First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *gormRepository) CreateApplication(app *model.CreatorApplication) error {
	return r.db.Create(app).Error
}

func (r *gormRepository) FindApplicationByID(id uint) (*model.CreatorApplication, error) {
	var app model.CreatorApplication
	if err := r.db.Preload("User").First(&app, id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *gormRepository) SaveApplication(app *model.CreatorApplication) error {
	return r.db.Save(app).Error
}

func (r *gormRepository) ListApplications(status model.CreatorApplicationStatus, offset, limit int) ([]model.CreatorApplication, int64, error) {
	query := r.db.Model(&model.CreatorApplication{}).Preload("User")
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

func (r *gormRepository) FindAdmins() ([]model.User, error) {
	var admins []model.User
	if err := r.db.Where("role IN ?", []string{model.RoleAdmin, model.RoleSuperAdmin}).Find(&admins).Error; err != nil {
		return nil, err
	}
	return admins, nil
}

func (r *gormRepository) Begin() *gorm.DB {
	return r.db.Begin()
}
