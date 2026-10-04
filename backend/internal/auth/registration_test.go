package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

func TestRegister_WithCaptcha(t *testing.T) {
	svc, _, db := newTestService(t)
	settings := model.GeneralSettings{CaptchaEnabled: true, RegistrationEnabled: true}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("failed to enable captcha: %v", err)
	}

	registerReq := func(captchaID, token string, x, y int) RegisterRequest {
		return RegisterRequest{
			Email:        "new@example.com",
			Password:     "password123",
			Username:     "newbie",
			CaptchaID:    captchaID,
			CaptchaToken: token,
			CaptchaX:     x,
			CaptchaY:     y,
			Protocol:     "http",
			Host:         "localhost",
		}
	}

	// missing captcha fields
	if _, err := svc.Register(context.Background(), registerReq("", "", 0, 0)); !errors.Is(err, ErrCaptchaRequired) {
		t.Errorf("expected ErrCaptchaRequired, got %v", err)
	}

	captcha := model.Captcha{ID: "c2", Token: "t2", X: 80, Y: 40, Width: 60, Height: 60, ExpiresAt: time.Now().Add(5 * time.Minute)}
	if err := db.Create(&captcha).Error; err != nil {
		t.Fatalf("failed to seed captcha: %v", err)
	}

	// wrong y position
	if _, err := svc.Register(context.Background(), registerReq("c2", "t2", 80, 20)); !errors.Is(err, ErrCaptchaMismatch) {
		t.Errorf("expected ErrCaptchaMismatch for wrong y, got %v", err)
	}

	// correct position passes (fresh challenge — the failed attempt consumed c2)
	seedCaptcha(t, db, "c2b", "t2b", 80, 40)
	result, err := svc.Register(context.Background(), RegisterRequest{
		Email: "new@example.com", Password: "password123", Username: "newbie",
		CaptchaID: "c2b", CaptchaToken: "t2b", CaptchaX: 80, CaptchaY: 40,
		Protocol: "http", Host: "localhost",
	})
	if err != nil {
		t.Fatalf("Register with captcha error: %v", err)
	}
	if result.User == nil {
		t.Errorf("expected user")
	}
}

func TestRegister_Success(t *testing.T) {
	svc, _, db := newTestService(t)

	result, err := svc.Register(context.Background(), RegisterRequest{Email: "new@example.com", Password: "password123", Username: "newbie", Protocol: "http", Host: "localhost:8080"})
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if result.RequiresVerification {
		t.Errorf("expected no verification requirement (SMTP disabled)")
	}
	if result.User.Role != model.RoleGuest {
		t.Errorf("expected guest role, got %s", result.User.Role)
	}
	var stored model.User
	if err := db.First(&stored, result.User.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if stored.Username != "newbie" {
		t.Errorf("expected username newbie, got %s", stored.Username)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	seedUser(t, db, "dup@example.com", "password123", model.RoleGuest, true)

	if _, err := svc.Register(context.Background(), RegisterRequest{Email: "dup@example.com", Password: "password123", Username: "other", Protocol: "http", Host: "localhost"}); !errors.Is(err, ErrUserExists) {
		t.Errorf("expected ErrUserExists, got %v", err)
	}
}

// A registration-time user lookup failure must fail closed with ErrQueryFailed
// instead of being treated as "user does not exist" and proceeding to create
// the account.
func TestRegister_DbErrorOnUserLookup(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.repo = &failingRepo{Repository: svc.repo, findUserByEmailErr: errors.New("database is unavailable")}

	_, err := svc.Register(context.Background(), RegisterRequest{
		Email: "boom@example.com", Password: "password123", Username: "boom",
		Protocol: "https", Host: "example.com",
	})
	if !errors.Is(err, ErrQueryFailed) {
		t.Errorf("expected ErrQueryFailed when user lookup fails, got %v", err)
	}
}

func TestRegister_Disabled(t *testing.T) {
	svc, _, db := newTestService(t)
	// Seed settings with registration disabled. The RegistrationEnabled field
	// used to carry gorm:"default:true", which made GORM omit the zero value
	// on Create and silently store true — that bug is now fixed.
	if err := db.Create(&model.GeneralSettings{RegistrationEnabled: false}).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	if _, err := svc.Register(context.Background(), RegisterRequest{Email: "new@example.com", Password: "password123", Username: "newbie", Protocol: "http", Host: "localhost"}); !errors.Is(err, ErrRegistrationDisabled) {
		t.Errorf("expected ErrRegistrationDisabled, got %v", err)
	}
}

func TestRegister_SendsVerificationEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	enableSMTP(t, db)
	captureEmails(t)

	result, err := svc.Register(context.Background(), RegisterRequest{
		Email: "new@example.com", Password: "password123", Username: "newbie",
		Protocol: "https", Host: "example.com",
	})
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if !result.RequiresVerification {
		t.Errorf("expected verification required")
	}

	if len(capturedEmails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(capturedEmails))
	}
	email := capturedEmails[0]
	if email.To != "new@example.com" {
		t.Errorf("expected To new@example.com, got %q", email.To)
	}
	if email.Subject != "Please Verify Your Email Address" {
		t.Errorf("unexpected subject %q", email.Subject)
	}
	if !strings.Contains(email.TextBody, "newbie") || !strings.Contains(email.HTMLBody, "newbie") {
		t.Errorf("expected recipient username in email body")
	}

	tok := emailedToken(t, email)

	var stored model.User
	if err := db.First(&stored, result.User.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.VerificationToken != tokenStorageForm(tok) {
		t.Errorf("expected only the token hash at rest, got %q", stored.VerificationToken)
	}

	// The emailed link actually verifies the address.
	emailChange, _, err := svc.VerifyEmail(context.Background(), tok)
	if err != nil {
		t.Fatalf("VerifyEmail via link error: %v", err)
	}
	if emailChange {
		t.Errorf("expected normal verification, got email change")
	}
}

// Re-registering an unverified address while its verification token is still
// live must not send another email: the live token is the per-account resend
// cooldown that keeps the endpoint from being used as a mail bomb.
func TestRegister_UnverifiedDuplicateNoSecondEmailInCooldown(t *testing.T) {
	svc, _, db := newTestService(t)
	enableSMTP(t, db)
	captureEmails(t)

	first, err := svc.Register(context.Background(), RegisterRequest{
		Email: "repeat@example.com", Password: "password123", Username: "first",
		Protocol: "https", Host: "example.com",
	})
	if err != nil || !first.RequiresVerification {
		t.Fatalf("initial registration failed: result=%+v err=%v", first, err)
	}

	second, err := svc.Register(context.Background(), RegisterRequest{
		Email: "repeat@example.com", Password: "password123", Username: "second",
		Protocol: "https", Host: "example.com",
	})
	if err != nil || !second.RequiresVerification {
		t.Fatalf("duplicate registration should still ask for verification: result=%+v err=%v", second, err)
	}
	var count int64
	if err := db.Model(&model.User{}).Where("email = ?", "repeat@example.com").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one user, got %d", count)
	}
	if len(capturedEmails) != 1 {
		t.Fatalf("expected no second email while the first token is live, got %d emails", len(capturedEmails))
	}
}

// When SMTP is disabled there is no verification flow at all: a duplicate
// registration for an unverified account must mirror fresh-registration
// semantics (immediately usable, requires_verification=false) instead of
// telling the user to wait for an email nobody can send.
func TestRegister_DuplicateUnverifiedSMTPDisabled(t *testing.T) {
	svc, _, _ := newTestService(t)
	captureEmails(t)

	first, err := svc.Register(context.Background(), RegisterRequest{
		Email: "dup@example.com", Password: "password123", Username: "first",
	})
	if err != nil || first.RequiresVerification {
		t.Fatalf("initial registration: result=%+v err=%v", first, err)
	}

	second, err := svc.Register(context.Background(), RegisterRequest{
		Email: "dup@example.com", Password: "password123", Username: "second",
	})
	if err != nil {
		t.Fatalf("duplicate registration error: %v", err)
	}
	if second.RequiresVerification {
		t.Error("requires_verification must be false when SMTP is disabled")
	}
	if second.User == nil || second.User.Email != "dup@example.com" {
		t.Errorf("expected the existing user returned, got %+v", second.User)
	}
	if len(capturedEmails) != 0 {
		t.Errorf("expected no emails without SMTP, got %d", len(capturedEmails))
	}
}

func TestRegister_NoEmailWhenSMTPDisabled(t *testing.T) {
	svc, _, _ := newTestService(t)
	captureEmails(t)

	result, err := svc.Register(context.Background(), RegisterRequest{
		Email: "x@example.com", Password: "password123", Username: "x",
		Protocol: "https", Host: "example.com",
	})
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if result.RequiresVerification {
		t.Errorf("expected no verification requirement when SMTP disabled")
	}
	if len(capturedEmails) != 0 {
		t.Errorf("expected no email sent, got %d", len(capturedEmails))
	}
}
