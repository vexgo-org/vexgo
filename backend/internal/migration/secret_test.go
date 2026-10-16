package migration

import (
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// openTestDB opens an in-memory SQLite database with a single connection so
// the database is not lost between statements.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db
}

// newEncryptionTestDB adds the three tables the secrets migration touches.
func newEncryptionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openTestDB(t)
	if err := db.AutoMigrate(&model.SMTPConfig{}, &model.AIConfig{}, &model.CommentModerationConfig{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func seedEncryptionFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	fixtures := []any{
		&model.SMTPConfig{Enabled: true, Host: "smtp.example.com", Password: "smtp-plain"},
		&model.SMTPConfig{Enabled: false, Host: "no-password.example.com", Password: ""},
		&model.AIConfig{Enabled: true, ApiKey: "ai-plain"},
		&model.AIConfig{Enabled: false, ApiKey: ""},
		&model.CommentModerationConfig{LLMReviewEnabled: true, ApiKey: "mod-plain"},
		&model.CommentModerationConfig{LLMReviewEnabled: false, ApiKey: ""},
	}
	for _, f := range fixtures {
		if err := db.Create(f).Error; err != nil {
			t.Fatalf("failed to seed fixture: %v", err)
		}
	}
}

// TC-ENC-021: a startup migration with a configured key encrypts all three
// plaintext secrets in place, and the values decrypt correctly afterwards.
func TestMigrateSecretsAtRest_EncryptsPlaintextSecrets(t *testing.T) {
	db := newEncryptionTestDB(t)
	seedEncryptionFixtures(t, db)

	cipher, err := secrets.New("test-encryption-key")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	migrated, err := migrateSecretsAtRest(db, cipher)
	if err != nil {
		t.Fatalf("MigrateSecretsAtRest error: %v", err)
	}
	if migrated != 3 {
		t.Errorf("expected 3 secrets migrated, got %d", migrated)
	}

	var smtp model.SMTPConfig
	if err := db.First(&smtp, "host = ?", "smtp.example.com").Error; err != nil {
		t.Fatalf("failed to load smtp config: %v", err)
	}
	if !secrets.IsEncrypted(smtp.Password) {
		t.Errorf("expected smtp password encrypted, got %q", smtp.Password)
	}
	if got, err := cipher.Decrypt(smtp.Password); err != nil || got != "smtp-plain" {
		t.Errorf("smtp password round trip failed: got %q, err %v", got, err)
	}

	var ai model.AIConfig
	if err := db.First(&ai, "enabled = ?", true).Error; err != nil {
		t.Fatalf("failed to load ai config: %v", err)
	}
	if got, err := cipher.Decrypt(ai.ApiKey); err != nil || got != "ai-plain" {
		t.Errorf("ai api key round trip failed: got %q, err %v", got, err)
	}

	var mod model.CommentModerationConfig
	if err := db.First(&mod, "llm_review_enabled = ?", true).Error; err != nil {
		t.Fatalf("failed to load moderation config: %v", err)
	}
	if got, err := cipher.Decrypt(mod.ApiKey); err != nil || got != "mod-plain" {
		t.Errorf("moderation api key round trip failed: got %q, err %v", got, err)
	}
}

// TC-ENC-022: running the migration twice is a no-op; values migrated on the
// first run still decrypt after the second run.
func TestMigrateSecretsAtRest_Idempotent(t *testing.T) {
	db := newEncryptionTestDB(t)
	seedEncryptionFixtures(t, db)

	cipher, err := secrets.New("test-encryption-key")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	if _, err := migrateSecretsAtRest(db, cipher); err != nil {
		t.Fatalf("first MigrateSecretsAtRest error: %v", err)
	}

	var smtpBefore model.SMTPConfig
	if err := db.First(&smtpBefore, "host = ?", "smtp.example.com").Error; err != nil {
		t.Fatalf("failed to load smtp config: %v", err)
	}

	migrated, err := migrateSecretsAtRest(db, cipher)
	if err != nil {
		t.Fatalf("second MigrateSecretsAtRest error: %v", err)
	}
	if migrated != 0 {
		t.Errorf("expected second run to be a no-op, migrated %d", migrated)
	}

	var smtpAfter model.SMTPConfig
	if err := db.First(&smtpAfter, "host = ?", "smtp.example.com").Error; err != nil {
		t.Fatalf("failed to load smtp config: %v", err)
	}
	if smtpAfter.Password != smtpBefore.Password {
		t.Errorf("expected stored ciphertext unchanged on second run")
	}
	if got, err := cipher.Decrypt(smtpAfter.Password); err != nil || got != "smtp-plain" {
		t.Errorf("smtp password still decryptable after second run: got %q, err %v", got, err)
	}
}

// TC-ENC-023: values already carrying the encrypted marker are left untouched
// by the migration.
func TestMigrateSecretsAtRest_SkipsAlreadyEncrypted(t *testing.T) {
	db := newEncryptionTestDB(t)
	seedEncryptionFixtures(t, db)

	cipher, err := secrets.New("test-encryption-key")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	// Pre-encrypt the AI key so the migration must skip it.
	aiCfg := model.AIConfig{}
	if err := db.First(&aiCfg, "enabled = ?", true).Error; err != nil {
		t.Fatalf("failed to load ai config: %v", err)
	}
	encrypted, err := cipher.Encrypt("ai-plain")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if err := db.Model(&model.AIConfig{}).Where("id = ?", aiCfg.ID).UpdateColumn("api_key", encrypted).Error; err != nil {
		t.Fatalf("failed to pre-encrypt ai key: %v", err)
	}

	migrated, err := migrateSecretsAtRest(db, cipher)
	if err != nil {
		t.Fatalf("MigrateSecretsAtRest error: %v", err)
	}
	if migrated != 2 {
		t.Errorf("expected 2 secrets migrated (ai key skipped), got %d", migrated)
	}

	var aiAfter model.AIConfig
	if err := db.First(&aiAfter, "id = ?", aiCfg.ID).Error; err != nil {
		t.Fatalf("failed to load ai config: %v", err)
	}
	if aiAfter.ApiKey != encrypted {
		t.Errorf("expected already-encrypted value untouched")
	}
}

// TC-ENC-024: with no configured key the service must be a no-op. This is the
// branch that keeps the no-key plaintext fallback working: encrypting here
// would make the values unreadable, because there is no cipher to open them.
func TestMigrateSecrets_NilCipherSkips(t *testing.T) {
	db := newEncryptionTestDB(t)
	seedEncryptionFixtures(t, db)

	// Deps.Cipher is left nil, which is what initCipher returns when
	// settings_encryption_key is unset.
	if err := NewService(Deps{DB: db}).MigrateSecrets(); err != nil {
		t.Fatalf("MigrateSecrets: %v", err)
	}

	var smtp model.SMTPConfig
	if err := db.First(&smtp, "host = ?", "smtp.example.com").Error; err != nil {
		t.Fatalf("failed to load smtp config: %v", err)
	}
	if smtp.Password != "smtp-plain" {
		t.Errorf("expected smtp password left plaintext, got %q", smtp.Password)
	}

	var ai model.AIConfig
	if err := db.First(&ai, "enabled = ?", true).Error; err != nil {
		t.Fatalf("failed to load ai config: %v", err)
	}
	if ai.ApiKey != "ai-plain" {
		t.Errorf("expected ai api key left plaintext, got %q", ai.ApiKey)
	}

	var mod model.CommentModerationConfig
	if err := db.First(&mod, "llm_review_enabled = ?", true).Error; err != nil {
		t.Fatalf("failed to load moderation config: %v", err)
	}
	if mod.ApiKey != "mod-plain" {
		t.Errorf("expected moderation api key left plaintext, got %q", mod.ApiKey)
	}
}

// TC-ENC-025: the same migration through the exported entry point app.go
// calls, so the Deps wiring and the nil guard stay covered.
func TestMigrateSecrets_EncryptsViaService(t *testing.T) {
	db := newEncryptionTestDB(t)
	seedEncryptionFixtures(t, db)

	cipher, err := secrets.New("test-encryption-key")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	if err := NewService(Deps{DB: db, Cipher: cipher}).MigrateSecrets(); err != nil {
		t.Fatalf("MigrateSecrets: %v", err)
	}

	var smtp model.SMTPConfig
	if err := db.First(&smtp, "host = ?", "smtp.example.com").Error; err != nil {
		t.Fatalf("failed to load smtp config: %v", err)
	}
	if !secrets.IsEncrypted(smtp.Password) {
		t.Errorf("expected smtp password encrypted, got %q", smtp.Password)
	}
	if got, err := cipher.Decrypt(smtp.Password); err != nil || got != "smtp-plain" {
		t.Errorf("smtp password round trip failed: got %q, err %v", got, err)
	}

	// A second run through the service must stay a no-op.
	if err := NewService(Deps{DB: db, Cipher: cipher}).MigrateSecrets(); err != nil {
		t.Fatalf("second MigrateSecrets: %v", err)
	}
	var smtpAfter model.SMTPConfig
	if err := db.First(&smtpAfter, "host = ?", "smtp.example.com").Error; err != nil {
		t.Fatalf("failed to reload smtp config: %v", err)
	}
	if got, err := cipher.Decrypt(smtpAfter.Password); err != nil || got != "smtp-plain" {
		t.Errorf("smtp password still decryptable after second run: got %q, err %v", got, err)
	}
}

// TC-ENC-024: a failure inside the migration is propagated, not swallowed.
// Swallowing it would let the server boot with plaintext secrets still in the
// database while logging nothing.
func TestMigrateSecrets_SurfacesError(t *testing.T) {
	// No AutoMigrate: every table the migration reads is missing.
	db := openTestDB(t)

	cipher, err := secrets.New("test-encryption-key")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	if err := NewService(Deps{DB: db, Cipher: cipher}).MigrateSecrets(); err == nil {
		t.Fatal("expected error when the secret tables are missing, got nil")
	}
}
