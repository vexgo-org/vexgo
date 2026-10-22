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

// dropLegacyColumn drops one column that is no longer part of the model. The
// table name comes from the model, the column is looked up by its raw name
// because the current schema no longer contains it. Migrator.DropColumn
// cannot be used here: it rebuilds the table from the current model schema,
// which would silently leave the legacy column in place. The identifiers come
// from the model and package constants, never user input.
func dropLegacyColumn(db *gorm.DB, value any, column string) error {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(value); err != nil {
		return fmt.Errorf("parse %T schema: %w", value, err)
	}
	if !db.Migrator().HasTable(value) || !db.Migrator().HasColumn(value, column) {
		return nil
	}
	var query string
	switch db.Name() {
	case "mysql":
		query = fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", stmt.Schema.Table, column)
	default: // sqlite and postgres accept double-quoted identifiers
		query = fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN "%s"`, stmt.Schema.Table, column)
	}
	return db.Exec(query).Error
}
