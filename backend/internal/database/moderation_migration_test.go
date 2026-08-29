package database

import (
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newLegacyModerationDB opens an in-memory SQLite database whose
// comment_moderation_configs table still has the pre-3-switch columns, as an
// upgraded installation would have before the migration runs.
func newLegacyModerationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	create := `CREATE TABLE comment_moderation_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		enabled BOOLEAN,
		auto_approve_enabled BOOLEAN,
		min_score_threshold REAL,
		model_provider TEXT,
		api_key TEXT,
		api_endpoint TEXT,
		model_name TEXT,
		moderation_prompt TEXT,
		block_keywords TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`
	if err := db.Exec(create).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	return db
}

// seedLegacyConfig inserts one legacy moderation config row with the given
// old switch values and returns its id.
func seedLegacyConfig(t *testing.T, db *gorm.DB, enabled, autoApprove bool) uint {
	t.Helper()
	res := db.Exec(
		`INSERT INTO comment_moderation_configs
			(enabled, auto_approve_enabled, min_score_threshold, api_key)
			VALUES (?, ?, ?, ?)`,
		enabled, autoApprove, 0.5, "stored-key",
	)
	if res.Error != nil {
		t.Fatalf("seed legacy config: %v", res.Error)
	}
	var id uint
	if err := db.Raw("SELECT id FROM comment_moderation_configs ORDER BY id DESC LIMIT 1").Scan(&id).Error; err != nil {
		t.Fatalf("read seeded id: %v", err)
	}
	return id
}

// switchValues reads the three new switch columns for one row.
func switchValues(t *testing.T, db *gorm.DB, id uint) (manual, keyword, llm bool) {
	t.Helper()
	row := db.Raw(
		`SELECT manual_review_enabled, keyword_filter_enabled, llm_review_enabled
			FROM comment_moderation_configs WHERE id = ?`, id,
	).Row()
	if err := row.Scan(&manual, &keyword, &llm); err != nil {
		t.Fatalf("read switches: %v", err)
	}
	return manual, keyword, llm
}

// TC-CMOD-021: legacy Enabled/AutoApproveEnabled combos map to the new
// switches per the documented mapping, and the legacy columns are dropped so
// re-running the migration is a no-op.
func TestMigrateLegacyModerationConfig_Mapping(t *testing.T) {
	db := newLegacyModerationDB(t)
	if err := db.AutoMigrate(&model.CommentModerationConfig{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	oldDefault := seedLegacyConfig(t, db, false, true)  // was: publish immediately
	manualHold := seedLegacyConfig(t, db, false, false) // was: pending
	aiEnabled := seedLegacyConfig(t, db, true, true)    // was: AI moderation on

	migrated, err := MigrateLegacyModerationConfig(db)
	if err != nil {
		t.Fatalf("MigrateLegacyModerationConfig: %v", err)
	}
	if migrated != 3 {
		t.Errorf("expected 3 migrated rows, got %d", migrated)
	}

	if manual, keyword, llm := switchValues(t, db, oldDefault); manual || keyword || llm {
		t.Errorf("old default (enabled=false, auto=true) should map to all off, got manual=%v keyword=%v llm=%v",
			manual, keyword, llm)
	}
	if manual, keyword, llm := switchValues(t, db, manualHold); !manual || keyword || llm {
		t.Errorf("enabled=false, auto=false should map to manual only, got manual=%v keyword=%v llm=%v", manual, keyword, llm)
	}
	if manual, keyword, llm := switchValues(t, db, aiEnabled); manual || !keyword || !llm {
		t.Errorf("enabled=true should map to keyword+llm on, got manual=%v keyword=%v llm=%v", manual, keyword, llm)
	}

	if db.Migrator().HasColumn(&model.CommentModerationConfig{}, "enabled") {
		t.Errorf("legacy column enabled should be dropped after migration")
	}
	if db.Migrator().HasColumn(&model.CommentModerationConfig{}, "auto_approve_enabled") {
		t.Errorf("legacy column auto_approve_enabled should be dropped after migration")
	}
	if db.Migrator().HasColumn(&model.CommentModerationConfig{}, "min_score_threshold") {
		t.Errorf("legacy column min_score_threshold should be dropped after migration")
	}

	// Data other than the switches must survive the migration untouched.
	var apiKey string
	readKey := db.Raw("SELECT api_key FROM comment_moderation_configs WHERE id = ?", aiEnabled).Scan(&apiKey)
	if err := readKey.Error; err != nil {
		t.Fatalf("read api key: %v", err)
	}
	if apiKey != "stored-key" {
		t.Errorf("expected api key preserved, got %q", apiKey)
	}
}

// TC-CMOD-021: the migration is idempotent — a second run must not touch the
// switches, so admin changes made after the one-time migration survive
// restarts.
func TestMigrateLegacyModerationConfig_Idempotent(t *testing.T) {
	db := newLegacyModerationDB(t)
	if err := db.AutoMigrate(&model.CommentModerationConfig{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	id := seedLegacyConfig(t, db, true, false)

	if _, err := MigrateLegacyModerationConfig(db); err != nil {
		t.Fatalf("first migration: %v", err)
	}

	// Simulate an admin turning the LLM switch back off after the migration.
	adminUpdate := db.Exec("UPDATE comment_moderation_configs SET llm_review_enabled = ? WHERE id = ?", false, id)
	if err := adminUpdate.Error; err != nil {
		t.Fatalf("admin update: %v", err)
	}

	migrated, err := MigrateLegacyModerationConfig(db)
	if err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if migrated != 0 {
		t.Errorf("expected second run to be a no-op, got %d migrated rows", migrated)
	}
	if _, keyword, llm := switchValues(t, db, id); !keyword || llm {
		t.Errorf("second run must not rewrite the switches, got keyword=%v llm=%v", keyword, llm)
	}
}

// A fresh database (no legacy columns) and a database without any config row
// must both be untouched no-ops.
func TestMigrateLegacyModerationConfig_NoopCases(t *testing.T) {
	t.Run("fresh table without legacy columns", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		if err != nil {
			t.Fatalf("open test db: %v", err)
		}
		if err := db.AutoMigrate(&model.CommentModerationConfig{}); err != nil {
			t.Fatalf("automigrate: %v", err)
		}
		migrated, err := MigrateLegacyModerationConfig(db)
		if err != nil {
			t.Fatalf("MigrateLegacyModerationConfig: %v", err)
		}
		if migrated != 0 {
			t.Errorf("expected no-op on fresh schema, got %d", migrated)
		}
	})

	t.Run("missing table", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		if err != nil {
			t.Fatalf("open test db: %v", err)
		}
		if _, err := MigrateLegacyModerationConfig(db); err != nil {
			t.Fatalf("MigrateLegacyModerationConfig: %v", err)
		}
	})
}
