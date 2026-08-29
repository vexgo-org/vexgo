package database

import (
	"fmt"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// legacyModerationColumns are the pre-3-switch configuration columns of the
// comment_moderation_configs table.
var legacyModerationColumns = []string{"enabled", "auto_approve_enabled", "min_score_threshold"}

// MigrateLegacyModerationConfig maps the single legacy comment moderation
// config row onto the new explicit switches, one time at startup:
//
//	enabled=false, auto_approve_enabled=true  (old default) → all switches off
//	enabled=false, auto_approve_enabled=false               → manual review on
//	enabled=true, any auto_approve_enabled                  → keyword + LLM review on
//
// The legacy columns are dropped once their values are mapped. That both
// removes dead schema and makes the migration one-shot: on later startups the
// columns no longer exist and this function is a no-op, so switch changes an
// admin makes after the migration are never rewritten.
func MigrateLegacyModerationConfig(db *gorm.DB) (int, error) {
	table := &model.CommentModerationConfig{}
	if !db.Migrator().HasTable(table) || !db.Migrator().HasColumn(table, legacyModerationColumns[0]) {
		return 0, nil
	}

	type legacyRow struct {
		ID                 uint
		Enabled            bool
		AutoApproveEnabled bool
	}
	var rows []legacyRow
	if err := db.Table("comment_moderation_configs").
		Select("id, enabled, auto_approve_enabled").
		Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("read legacy comment moderation config: %w", err)
	}

	migrated := 0
	for _, row := range rows {
		updates := legacySwitchUpdates(row.Enabled, row.AutoApproveEnabled)
		if len(updates) > 0 {
			if err := db.Table("comment_moderation_configs").
				Where("id = ?", row.ID).
				Updates(updates).Error; err != nil {
				return migrated, fmt.Errorf("migrate comment moderation config (id=%d): %w", row.ID, err)
			}
		}
		migrated++
	}

	for _, column := range legacyModerationColumns {
		if err := dropLegacyModerationColumn(db, column); err != nil {
			return migrated, fmt.Errorf("drop legacy moderation column %s: %w", column, err)
		}
	}

	if migrated > 0 {
		slog.Info("migrated legacy comment moderation config to explicit switches", "rows", migrated)
	}
	return migrated, nil
}

// legacySwitchUpdates translates one legacy switch combination into the new
// column updates; an empty map means "all off" and needs no write.
func legacySwitchUpdates(enabled, autoApprove bool) map[string]any {
	switch {
	case enabled:
		return map[string]any{"keyword_filter_enabled": true, "llm_review_enabled": true}
	case !autoApprove:
		return map[string]any{"manual_review_enabled": true}
	default:
		return nil
	}
}

// dropLegacyModerationColumn drops one legacy column if it still exists.
// Migrator.DropColumn cannot be used here: it rebuilds the table from the
// current model schema, which no longer contains the legacy fields. The
// identifiers are package constants, never user input.
func dropLegacyModerationColumn(db *gorm.DB, column string) error {
	if !db.Migrator().HasTable(&model.CommentModerationConfig{}) ||
		!db.Migrator().HasColumn(&model.CommentModerationConfig{}, column) {
		return nil
	}
	var query string
	switch db.Name() {
	case "mysql":
		query = fmt.Sprintf("ALTER TABLE `comment_moderation_configs` DROP COLUMN `%s`", column)
	default: // sqlite and postgres accept double-quoted identifiers
		query = fmt.Sprintf(`ALTER TABLE "comment_moderation_configs" DROP COLUMN "%s"`, column)
	}
	return db.Exec(query).Error
}
