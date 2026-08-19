package settings

import (
	"context"
	"errors"
	"testing"

	"vexgo/backend/internal/model"
	"vexgo/backend/internal/public"

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
	return NewService(Deps{DB: db, Themes: renderer}), db
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
