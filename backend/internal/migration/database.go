package migration

import (
	"fmt"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// autoMigrate creates or updates the database schema for all models.
func autoMigrate(db *gorm.DB) error {
	// Remove duplicate likes (same post+user) before the composite unique
	// index is created, so pre-existing duplicate data cannot break the
	// migration. On a fresh database the table does not exist yet and this
	// is a no-op.
	if db.Migrator().HasTable(&model.Like{}) {
		if err := db.Exec(
			"DELETE FROM likes WHERE id NOT IN (SELECT MIN(id) FROM likes GROUP BY post_id, user_id)",
		).Error; err != nil {
			return fmt.Errorf("deduplicate likes: %w", err)
		}
	}

	return db.AutoMigrate(
		&model.Post{},
		&model.Page{},
		&model.User{},
		&model.Tag{},
		&model.Category{},
		&model.Comment{},
		&model.Like{},
		&model.MediaFile{},
		&model.SMTPConfig{},
		&model.Captcha{},
		&model.GeneralSettings{},
		&model.CommentModerationConfig{},
		&model.AIConfig{},
		&model.SSOBinding{},
		&model.ThemeConfig{},
		&model.Notification{},
		&model.CreatorApplication{},
		&model.Asset{},
		&model.Blob{},
	)
}
