package verification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"

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
	if err := db.AutoMigrate(&model.User{}, &model.Captcha{}, &model.GeneralSettings{}, &model.SMTPConfig{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	return NewService(Deps{DB: db, Mailer: mailer.NewMailer(db)}), db
}

// capturedEmail holds the rendered parts of an outgoing email captured by the
// mailer test seam.
type capturedEmail struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// capturedEmails records emails captured by the mailer test seam.
var capturedEmails []capturedEmail

// captureEmails installs the mailer capture hook and resets it after the test.
func captureEmails(t *testing.T) {
	t.Helper()
	capturedEmails = nil
	mailer.SetMailCaptureHook(func(to, subject, textBody, htmlBody string) {
		capturedEmails = append(capturedEmails, capturedEmail{to, subject, textBody, htmlBody})
	})
	t.Cleanup(func() { mailer.SetMailCaptureHook(nil) })
}

func TestIsCaptchaEnabled_DefaultDisabled(t *testing.T) {
	svc, _ := newTestService(t)
	enabled, err := svc.IsCaptchaEnabled(context.Background())
	if err != nil {
		t.Fatalf("IsCaptchaEnabled error: %v", err)
	}
	if enabled {
		t.Errorf("expected captcha disabled by default")
	}
}

func TestIsCaptchaEnabled_WhenEnabled(t *testing.T) {
	svc, db := newTestService(t)
	settings := model.GeneralSettings{CaptchaEnabled: true}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	enabled, err := svc.IsCaptchaEnabled(context.Background())
	if err != nil {
		t.Fatalf("IsCaptchaEnabled error: %v", err)
	}
	if !enabled {
		t.Errorf("expected captcha enabled")
	}
}

func TestVerificationStatus(t *testing.T) {
	svc, db := newTestService(t)
	u := model.User{Username: "alice", Email: "alice@example.com", EmailVerified: true}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	verified, email, err := svc.VerificationStatus(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("VerificationStatus error: %v", err)
	}
	if !verified || email != "alice@example.com" {
		t.Errorf("expected verified true + email, got verified=%v email=%q", verified, email)
	}

	if _, _, err := svc.VerificationStatus(context.Background(), 99999); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGenerateCaptcha(t *testing.T) {
	svc, db := newTestService(t)
	captcha, err := svc.GenerateCaptcha(context.Background())
	if err != nil {
		t.Fatalf("GenerateCaptcha error: %v", err)
	}
	if captcha.ID == "" || captcha.Token == "" {
		t.Errorf("expected id and token, got %+v", captcha)
	}
	if captcha.BgImage == "" || captcha.PuzzleImg == "" {
		t.Errorf("expected base64 images")
	}
	if captcha.Y < 0 {
		t.Errorf("expected positive y coordinate, got %d", captcha.Y)
	}

	// persisted for later verification
	var stored model.Captcha
	if err := db.First(&stored, "id = ?", captcha.ID).Error; err != nil {
		t.Fatalf("failed to load captcha: %v", err)
	}
	if stored.X != captcha.X {
		t.Errorf("expected stored X %d, got %d", captcha.X, stored.X)
	}
}

func TestVerifyCaptcha(t *testing.T) {
	svc, db := newTestService(t)
	captcha, err := svc.GenerateCaptcha(context.Background())
	if err != nil {
		t.Fatalf("GenerateCaptcha error: %v", err)
	}

	// correct position passes and marks as used
	if err := svc.VerifyCaptcha(context.Background(), captcha.ID, captcha.Token, captcha.X); err != nil {
		t.Fatalf("VerifyCaptcha error: %v", err)
	}
	if err := svc.VerifyCaptcha(context.Background(), captcha.ID, captcha.Token, captcha.X); !errors.Is(err, ErrCaptchaUsed) {
		t.Errorf("expected ErrCaptchaUsed on second use, got %v", err)
	}

	// wrong position
	captcha2, err := svc.GenerateCaptcha(context.Background())
	if err != nil {
		t.Fatalf("GenerateCaptcha error: %v", err)
	}
	if err := svc.VerifyCaptcha(context.Background(), captcha2.ID, captcha2.Token, captcha2.X+100); !errors.Is(err, ErrCaptchaMismatch) {
		t.Errorf("expected ErrCaptchaMismatch, got %v", err)
	}

	// unknown captcha
	if err := svc.VerifyCaptcha(context.Background(), "nope", "nope", 10); !errors.Is(err, ErrCaptchaNotFound) {
		t.Errorf("expected ErrCaptchaNotFound, got %v", err)
	}

	// expired captcha
	expired := model.Captcha{
		ID:        "expired-id",
		Token:     "expired-token",
		X:         50,
		Y:         20,
		Width:     60,
		Height:    60,
		ExpiresAt: time.Now().Add(-1 * time.Minute),
		Used:      false,
	}
	if err := db.Create(&expired).Error; err != nil {
		t.Fatalf("failed to seed expired captcha: %v", err)
	}
	if err := svc.VerifyCaptcha(context.Background(), "expired-id", "expired-token", 50); !errors.Is(err, ErrCaptchaExpired) {
		t.Errorf("expected ErrCaptchaExpired, got %v", err)
	}
}

func TestResendVerificationEmail_AlreadyVerified(t *testing.T) {
	svc, db := newTestService(t)
	u := model.User{Username: "alice", Email: "alice@example.com", EmailVerified: true}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	if err := svc.ResendVerificationEmail(context.Background(), u.ID, "localhost:8080"); !errors.Is(err, ErrEmailAlreadyVerified) {
		t.Errorf("expected ErrEmailAlreadyVerified, got %v", err)
	}
}

func TestResendVerificationEmail_MissingUser(t *testing.T) {
	svc, _ := newTestService(t)
	if err := svc.ResendVerificationEmail(context.Background(), 99999, "localhost:8080"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestResendVerificationEmail_NoEmailWhenSMTPDisabled(t *testing.T) {
	svc, db := newTestService(t)
	captureEmails(t)
	u := model.User{Username: "alice", Email: "alice@example.com", EmailVerified: false}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if err := svc.ResendVerificationEmail(context.Background(), u.ID, "localhost:8080"); !errors.Is(err, ErrEmailServiceDisabled) {
		t.Errorf("expected ErrEmailServiceDisabled, got %v", err)
	}
	if len(capturedEmails) != 0 {
		t.Errorf("expected no email when SMTP disabled, got %d", len(capturedEmails))
	}
}
