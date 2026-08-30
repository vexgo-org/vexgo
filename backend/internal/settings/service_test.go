package settings

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/public"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&model.SMTPConfig{}, &model.GeneralSettings{}, &model.AIConfig{}, &model.ThemeConfig{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	renderer := public.NewRenderer(db, "http://localhost", t.TempDir())
	return NewService(Deps{DB: db, Themes: renderer, Mailer: mailer.NewService(mailer.Deps{DB: db})}), db
}

func TestGetSMTPConfig_Default(t *testing.T) {
	svc, _ := newTestService(t)
	config, err := svc.GetSMTPConfig(context.Background())
	if err != nil {
		t.Fatalf("GetSMTPConfig error: %v", err)
	}
	if config.Enabled || config.Port != 587 || config.FromName != "VexGo" {
		t.Errorf("expected default config, got %+v", config)
	}
}

func TestUpdateSMTPConfig_CreateUpdateAndMaskPassword(t *testing.T) {
	svc, db := newTestService(t)

	// create
	config, err := svc.UpdateSMTPConfig(context.Background(), SMTPConfigRequest{
		Enabled:   true,
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "user",
		Password:  "secret",
		FromEmail: "a@example.com",
		FromName:  "VexGo",
	})
	if err != nil {
		t.Fatalf("UpdateSMTPConfig error: %v", err)
	}
	if config.Password != "" {
		t.Errorf("expected password masked in response")
	}

	// stored password persists
	var stored model.SMTPConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if stored.Password != "secret" {
		t.Errorf("expected stored password, got %q", stored.Password)
	}

	// update without password preserves it
	config, err = svc.UpdateSMTPConfig(context.Background(), SMTPConfigRequest{
		Enabled:   false,
		Host:      "smtp2.example.com",
		Port:      465,
		Username:  "user",
		Password:  "",
		FromEmail: "a@example.com",
		FromName:  "VexGo",
	})
	if err != nil {
		t.Fatalf("UpdateSMTPConfig error: %v", err)
	}
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if stored.Password != "secret" {
		t.Errorf("expected password preserved, got %q", stored.Password)
	}
	if stored.Host != "smtp2.example.com" {
		t.Errorf("expected host updated")
	}
}

func TestTestSMTP_ValidationErrors(t *testing.T) {
	svc, db := newTestService(t)

	// not configured
	if _, err := svc.TestSMTP(context.Background(), "admin@example.com"); !errors.Is(err, ErrSMTPNotConfigured) {
		t.Errorf("expected ErrSMTPNotConfigured, got %v", err)
	}

	// disabled
	if err := db.Create(&model.SMTPConfig{Enabled: false}).Error; err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}
	if _, err := svc.TestSMTP(context.Background(), "admin@example.com"); !errors.Is(err, ErrSMTPDisabled) {
		t.Errorf("expected ErrSMTPDisabled, got %v", err)
	}

	// incomplete fields
	if err := db.Model(&model.SMTPConfig{}).Where("1 = 1").Updates(map[string]any{"enabled": true, "host": "localhost", "port": 25, "username": "u", "password": "p", "from_email": "a@b.c"}).Error; err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// no recipient (empty test email and empty admin email) → rejected before sending
	if err := db.Model(&model.SMTPConfig{}).Where("1 = 1").Update("test_email", "").Error; err != nil {
		t.Fatalf("failed to clear test email: %v", err)
	}
	if _, err := svc.TestSMTP(context.Background(), ""); !errors.Is(err, ErrSMTPNoRecipient) {
		t.Errorf("expected ErrSMTPNoRecipient, got %v", err)
	}

	// with a recipient the send is attempted (connection refused here is fine)
	if err := db.Model(&model.SMTPConfig{}).Where("1 = 1").Update("test_email", "t@example.com").Error; err != nil {
		t.Fatalf("failed to set test email: %v", err)
	}
	if _, err := svc.TestSMTP(context.Background(), ""); err == nil || errors.Is(err, ErrSMTPNoRecipient) || errors.Is(err, ErrSMTPIncomplete) {
		t.Errorf("expected a send attempt error, got %v", err)
	}
}

func TestGetGeneralSettings_Default(t *testing.T) {
	svc, _ := newTestService(t)
	config, err := svc.GetGeneralSettings(context.Background())
	if err != nil {
		t.Fatalf("GetGeneralSettings error: %v", err)
	}
	if !config.RegistrationEnabled || !config.AllowGuestViewPosts || config.SiteName != "VexGo" || config.ItemsPerPage != 20 {
		t.Errorf("expected default settings, got %+v", config)
	}
}

func TestUpdateGeneralSettings(t *testing.T) {
	svc, _ := newTestService(t)
	config, err := svc.UpdateGeneralSettings(context.Background(), GeneralSettingsRequest{
		CaptchaEnabled:      true,
		RegistrationEnabled: false,
		SiteName:            "My Blog",
		ItemsPerPage:        10,
	})
	if err != nil {
		t.Fatalf("UpdateGeneralSettings error: %v", err)
	}
	if !config.CaptchaEnabled || config.SiteName != "My Blog" {
		t.Errorf("expected updated settings, got %+v", config)
	}

	// read back — RegistrationEnabled: false must survive the create path.
	// It used to be stored as true because the model carried gorm:"default:true"
	// and GORM omitted zero-value bools on Create.
	got, err := svc.GetGeneralSettings(context.Background())
	if err != nil {
		t.Fatalf("GetGeneralSettings error: %v", err)
	}
	if !got.CaptchaEnabled || got.SiteName != "My Blog" {
		t.Errorf("expected persisted settings, got %+v", got)
	}
	if got.RegistrationEnabled {
		t.Errorf("expected RegistrationEnabled=false persisted, got %+v", got)
	}
}

func TestGetAIConfig_Default(t *testing.T) {
	svc, _ := newTestService(t)
	config, err := svc.GetAIConfig(context.Background())
	if err != nil {
		t.Fatalf("GetAIConfig error: %v", err)
	}
	if config.Provider != "openai" || config.ModelName != "gpt-3.5-turbo" {
		t.Errorf("expected default AI config, got %+v", config)
	}
}

func TestUpdateAIConfig_KeyPreserved(t *testing.T) {
	svc, db := newTestService(t)

	config, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     true,
		Provider:    "openai",
		ApiEndpoint: "https://api.openai.com",
		ApiKey:      "sk-secret",
		ModelName:   "gpt-4",
	})
	if err != nil {
		t.Fatalf("UpdateAIConfig error: %v", err)
	}
	if config.ApiKey != "" {
		t.Errorf("expected api key masked in response")
	}

	// update without key preserves it
	if _, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     false,
		Provider:    "openai",
		ApiEndpoint: "https://api.openai.com",
		ApiKey:      "",
		ModelName:   "gpt-4",
	}); err != nil {
		t.Fatalf("UpdateAIConfig error: %v", err)
	}
	var stored model.AIConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if stored.ApiKey != "sk-secret" {
		t.Errorf("expected api key preserved, got %q", stored.ApiKey)
	}
}

func TestGetThemeConfig_Default(t *testing.T) {
	svc, _ := newTestService(t)
	theme, err := svc.GetThemeConfig(context.Background())
	if err != nil {
		t.Fatalf("GetThemeConfig error: %v", err)
	}
	if theme != "default" {
		t.Errorf("expected default theme, got %q", theme)
	}
}

func TestUpdateThemeConfig(t *testing.T) {
	svc, _ := newTestService(t)

	// unknown theme rejected
	if _, err := svc.UpdateThemeConfig(context.Background(), "no-such-theme"); !errors.Is(err, ErrThemeNotFound) {
		t.Errorf("expected ErrThemeNotFound, got %v", err)
	}

	// default theme is always valid
	theme, err := svc.UpdateThemeConfig(context.Background(), "default")
	if err != nil {
		t.Fatalf("UpdateThemeConfig error: %v", err)
	}
	if theme != "default" {
		t.Errorf("expected default theme, got %q", theme)
	}

	got, err := svc.GetThemeConfig(context.Background())
	if err != nil {
		t.Fatalf("GetThemeConfig error: %v", err)
	}
	if got != "default" {
		t.Errorf("expected persisted default theme, got %q", got)
	}
}

func TestThemePreview_UnknownTheme(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.ThemePreview("no-such-theme"); !errors.Is(err, ErrThemeNotFound) {
		t.Errorf("expected ErrThemeNotFound, got %v", err)
	}
}

// newTestServiceWithCipher builds a settings service wired with a real cipher
// (test passphrase) plus the underlying DB, for encryption-at-rest tests.
func newTestServiceWithCipher(t *testing.T) (*Service, *gorm.DB, *secrets.Cipher) {
	t.Helper()
	db := newTestDB(t)
	cipher, err := secrets.New("test-encryption-key")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}
	renderer := public.NewRenderer(db, "http://localhost", t.TempDir())
	svc := NewService(Deps{DB: db, Themes: renderer, Mailer: mailer.NewService(mailer.Deps{DB: db}), Cipher: cipher})
	return svc, db, cipher
}

// TC-ENC-009: with a cipher wired, the SMTP password is stored as
// enc:v1: ciphertext, never as plaintext, while API responses stay masked.
func TestUpdateSMTPConfig_EncryptsPasswordWithCipher(t *testing.T) {
	svc, db, cipher := newTestServiceWithCipher(t)

	resp, err := svc.UpdateSMTPConfig(context.Background(), SMTPConfigRequest{
		Enabled:   true,
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "user",
		Password:  "plain-secret",
		FromEmail: "a@example.com",
	})
	if err != nil {
		t.Fatalf("UpdateSMTPConfig error: %v", err)
	}
	if resp.Password != "" {
		t.Errorf("expected password masked in response, got %q", resp.Password)
	}

	var stored model.SMTPConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	if !secrets.IsEncrypted(stored.Password) {
		t.Errorf("expected stored password to carry the encrypted marker, got %q", stored.Password)
	}
	if strings.Contains(stored.Password, "plain-secret") {
		t.Errorf("stored password must not contain the plaintext, got %q", stored.Password)
	}

	decrypted, err := cipher.Decrypt(stored.Password)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != "plain-secret" {
		t.Errorf("expected stored ciphertext to decrypt to the original, got %q", decrypted)
	}

	got, err := svc.GetSMTPConfig(context.Background())
	if err != nil {
		t.Fatalf("GetSMTPConfig error: %v", err)
	}
	if got.Password != "" {
		t.Errorf("expected masked password from GET, got %q", got.Password)
	}
}

// TC-ENC-010: without a configured cipher, secrets are stored as plaintext
// exactly as before the feature (no-key fallback).
func TestUpdateSMTPConfig_PlaintextWithoutCipher(t *testing.T) {
	svc, db, _ := newTestServiceWithCipher(t)
	svc.cipher = nil

	if _, err := svc.UpdateSMTPConfig(context.Background(), SMTPConfigRequest{
		Enabled:   true,
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "user",
		Password:  "plain-secret",
		FromEmail: "a@example.com",
	}); err != nil {
		t.Fatalf("UpdateSMTPConfig error: %v", err)
	}

	var stored model.SMTPConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	if stored.Password != "plain-secret" {
		t.Errorf("expected plaintext fallback storage, got %q", stored.Password)
	}
}

// TC-ENC-011: an update without a password keeps the stored ciphertext, and
// the preserved value still decrypts.
func TestUpdateSMTPConfig_EmptyPasswordKeepsStoredValueDecryptable(t *testing.T) {
	svc, db, cipher := newTestServiceWithCipher(t)

	if _, err := svc.UpdateSMTPConfig(context.Background(), SMTPConfigRequest{
		Enabled:   true,
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "user",
		Password:  "plain-secret",
		FromEmail: "a@example.com",
	}); err != nil {
		t.Fatalf("UpdateSMTPConfig error: %v", err)
	}

	if _, err := svc.UpdateSMTPConfig(context.Background(), SMTPConfigRequest{
		Enabled:   false,
		Host:      "smtp2.example.com",
		Port:      465,
		Username:  "user",
		Password:  "",
		FromEmail: "a@example.com",
	}); err != nil {
		t.Fatalf("UpdateSMTPConfig (no password) error: %v", err)
	}

	var stored model.SMTPConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	decrypted, err := cipher.Decrypt(stored.Password)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != "plain-secret" {
		t.Errorf("expected preserved password to stay decryptable, got %q", decrypted)
	}
}

// TC-ENC-012: with a cipher wired, the AI API key is stored as enc:v1:
// ciphertext while responses stay masked.
func TestUpdateAIConfig_EncryptsApiKeyWithCipher(t *testing.T) {
	svc, db, cipher := newTestServiceWithCipher(t)

	resp, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     true,
		Provider:    "openai",
		ApiEndpoint: "https://api.example.com/v1",
		ApiKey:      "sk-test-key",
		ModelName:   "gpt-3.5-turbo",
	})
	if err != nil {
		t.Fatalf("UpdateAIConfig error: %v", err)
	}
	if resp.ApiKey != "" {
		t.Errorf("expected API key masked in response, got %q", resp.ApiKey)
	}

	var stored model.AIConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	if !secrets.IsEncrypted(stored.ApiKey) {
		t.Errorf("expected stored API key to carry the encrypted marker, got %q", stored.ApiKey)
	}

	decrypted, err := cipher.Decrypt(stored.ApiKey)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != "sk-test-key" {
		t.Errorf("expected stored ciphertext to decrypt to the original, got %q", decrypted)
	}
}

// TC-ENC-013: an update without an API key keeps the stored ciphertext, and
// the preserved value still decrypts.
func TestUpdateAIConfig_EmptyApiKeyKeepsStoredValueDecryptable(t *testing.T) {
	svc, db, cipher := newTestServiceWithCipher(t)

	if _, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     true,
		Provider:    "openai",
		ApiEndpoint: "https://api.example.com/v1",
		ApiKey:      "sk-test-key",
		ModelName:   "gpt-3.5-turbo",
	}); err != nil {
		t.Fatalf("UpdateAIConfig error: %v", err)
	}

	if _, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     false,
		Provider:    "openai",
		ApiEndpoint: "https://api2.example.com/v1",
		ApiKey:      "",
		ModelName:   "gpt-4",
	}); err != nil {
		t.Fatalf("UpdateAIConfig (no key) error: %v", err)
	}

	var stored model.AIConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	decrypted, err := cipher.Decrypt(stored.ApiKey)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != "sk-test-key" {
		t.Errorf("expected preserved API key to stay decryptable, got %q", decrypted)
	}
}

// TC-ENC-014: an undecryptable stored AI key (wrong/rotated key) must not
// crash the server; the key is treated as unset and the test call reports the
// incomplete-configuration error.
func TestTestAI_UndecryptableKeyTreatedAsUnset(t *testing.T) {
	svc, db, _ := newTestServiceWithCipher(t)

	// Store a value encrypted under a different passphrase.
	other, err := secrets.New("other-key")
	if err != nil {
		t.Fatalf("failed to create other cipher: %v", err)
	}
	encrypted, err := other.Encrypt("sk-rotated")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if err := db.Create(&model.AIConfig{
		Enabled:     true,
		Provider:    "openai",
		ApiEndpoint: "https://api.example.com/v1",
		ApiKey:      encrypted,
		ModelName:   "gpt-3.5-turbo",
	}).Error; err != nil {
		t.Fatalf("failed to seed AI config: %v", err)
	}

	if _, err := svc.TestAI(context.Background()); !errors.Is(err, ErrAIIncomplete) {
		t.Errorf("expected ErrAIIncomplete for undecryptable key, got %v", err)
	}
}

// TestThemePreview_RejectsEscapingPreviewPath ensures a malicious theme
// metadata cannot make the public preview endpoint serve files outside the
// theme directory (arbitrary file read).
func TestThemePreview_RejectsEscapingPreviewPath(t *testing.T) {
	svc, _ := newTestService(t)
	dataDir := svc.themes.DataDir()

	themeID := "evil"
	themeDir := filepath.Join(dataDir, public.ThemesDir, themeID)
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	meta := `{"id": "evil", "preview": "../../secret.txt"}`
	if err := os.WriteFile(filepath.Join(themeDir, public.ThemeMetaFile), []byte(meta), 0o600); err != nil {
		t.Fatalf("write meta error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "secret.txt"), []byte("top secret"), 0o600); err != nil {
		t.Fatalf("write secret error: %v", err)
	}

	if _, err := svc.ThemePreview(themeID); !errors.Is(err, ErrPreviewNotFound) {
		t.Errorf("expected ErrPreviewNotFound for escaping preview path, got %v", err)
	}
}
