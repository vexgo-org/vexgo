package settings

import (
	"context"
	"errors"
	"fmt"
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
	if err := db.AutoMigrate(
		&model.SMTPConfig{},
		&model.Blob{},
		&model.Asset{},
		&model.GeneralSettings{},
		&model.AIConfig{},
		&model.ThemeConfig{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	renderer := public.NewService(public.Deps{DB: db, BaseURL: "http://localhost", DataDir: t.TempDir()})
	return NewService(Deps{DB: db, Themes: renderer, Mailer: mailer.NewService(mailer.Deps{DB: db})}), db
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
	renderer := public.NewService(public.Deps{DB: db, BaseURL: "http://localhost", DataDir: t.TempDir()})
	svc := NewService(Deps{DB: db, Themes: renderer, Mailer: mailer.NewService(mailer.Deps{DB: db}), Cipher: cipher})
	return svc, db, cipher
}

// wrappedNotFoundRepository returns the not-found sentinel wrapped with context,
// the way a repository that adds context to its errors does. Embedding
// Repository leaves the lookups these tests do not exercise unimplemented.
type wrappedNotFoundRepository struct {
	Repository
}

func (wrappedNotFoundRepository) GetAIConfig(context.Context) (model.AIConfig, error) {
	return model.AIConfig{}, fmt.Errorf("load ai config: %w", gorm.ErrRecordNotFound)
}

func (wrappedNotFoundRepository) GetSMTPConfig(context.Context) (model.SMTPConfig, error) {
	return model.SMTPConfig{}, fmt.Errorf("load smtp config: %w", gorm.ErrRecordNotFound)
}

// A wrapped gorm.ErrRecordNotFound must still be read as "no row stored yet".
// A bare == comparison would not match, silently turning the documented
// first-run defaults into a 500 once a repository starts wrapping its errors.
func TestNotFoundRecognizedThroughWrapping(t *testing.T) {
	svc := &Service{repo: wrappedNotFoundRepository{}}

	if _, err := svc.AIModels(context.Background()); !errors.Is(err, ErrAINotConfigured) {
		t.Errorf("AIModels: expected ErrAINotConfigured through a wrapped not-found, got %v", err)
	}

	config, err := svc.GetSMTPConfig(context.Background())
	if err != nil {
		t.Fatalf("GetSMTPConfig: expected the default config, got %v", err)
	}
	if config.Port != 587 || config.FromName != "VexGo" {
		t.Errorf("GetSMTPConfig: expected first-run defaults, got %+v", config)
	}
}
