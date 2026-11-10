package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"
)

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
