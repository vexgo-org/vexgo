package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

func TestChangePassword(t *testing.T) {
	svc, _, db := newTestService(t)
	u := seedUser(t, db, "alice@example.com", "oldpass", model.RoleGuest, true)

	// wrong old password
	if err := svc.ChangePassword(context.Background(), u.ID, "nope", "newpass123"); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("expected ErrWrongPassword, got %v", err)
	}

	// success: version incremented and new password works
	if err := svc.ChangePassword(context.Background(), u.ID, "oldpass", "newpass123"); err != nil {
		t.Fatalf("ChangePassword error: %v", err)
	}
	var stored model.User
	if err := db.First(&stored, u.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if stored.PasswordVersion != u.PasswordVersion+1 {
		t.Errorf("expected password version incremented")
	}
	if _, _, err := svc.Login(context.Background(), LoginRequest{Email: "alice@example.com", Password: "newpass123"}); err != nil {
		t.Errorf("expected new password to work, got %v", err)
	}
	if _, _, err := svc.Login(context.Background(), LoginRequest{Email: "alice@example.com", Password: "oldpass"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected old password to fail, got %v", err)
	}
}

func TestResetPassword(t *testing.T) {
	svc, _, db := newTestService(t)
	expiresAt := time.Now().Add(5 * time.Minute)
	u := model.User{
		Username:          "alice",
		Email:             "alice@example.com",
		Password:          "hash",
		Role:              model.RoleGuest,
		VerificationToken: tokenStorageForm("reset-token"),
		TokenExpiresAt:    &expiresAt,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	// invalid token
	if err := svc.ResetPassword(context.Background(), "nope", "newpass123"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("expected ErrInvalidResetToken, got %v", err)
	}

	// empty token must not resolve to an account whose token was cleared
	if err := svc.ResetPassword(context.Background(), "", "newpass123"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("expected ErrInvalidResetToken for empty token, got %v", err)
	}
	if _, err := svc.repo.FindUserByToken(context.Background(), ""); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected ErrRecordNotFound for empty token lookup, got %v", err)
	}

	// expired token (with the correct reset- prefix, so it reaches the
	// expiry check instead of being rejected by the prefix check)
	expiredAt := time.Now().Add(-1 * time.Minute)
	u2 := model.User{
		Username:          "bob",
		Email:             "bob@example.com",
		Password:          "hash",
		Role:              model.RoleGuest,
		VerificationToken: tokenStorageForm("reset-expired"),
		TokenExpiresAt:    &expiredAt,
	}
	if err := db.Create(&u2).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := svc.ResetPassword(context.Background(), "reset-expired", "newpass123"); !errors.Is(err, ErrResetTokenExpired) {
		t.Errorf("expected ErrResetTokenExpired, got %v", err)
	}

	// success: token cleared and password version bumped (invalidates sessions)
	if err := svc.ResetPassword(context.Background(), "reset-token", "newpass123"); err != nil {
		t.Fatalf("ResetPassword error: %v", err)
	}
	var stored model.User
	if err := db.First(&stored, u.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if stored.VerificationToken != "" {
		t.Errorf("expected token cleared")
	}
	if stored.PasswordVersion != u.PasswordVersion+1 {
		t.Errorf("expected password version bumped from %d to %d, got %d",
			u.PasswordVersion, u.PasswordVersion+1, stored.PasswordVersion)
	}
}

func TestResetPassword_RejectsNonResetTokens(t *testing.T) {
	svc, _, db := newTestService(t)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Email verification token (valid, unexpired) must NOT reset a password.
	u1 := model.User{
		Username:          "alice",
		Email:             "alice@example.com",
		Password:          "hash",
		Role:              model.RoleGuest,
		VerificationToken: tokenStorageForm("verify-abc"),
		TokenExpiresAt:    &expiresAt,
	}
	if err := db.Create(&u1).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := svc.ResetPassword(context.Background(), "verify-abc", "newpass123"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("expected ErrInvalidResetToken for verification token, got %v", err)
	}

	// Email-change token must NOT reset a password either.
	u2 := model.User{
		Username:          "bob",
		Email:             "bob@example.com",
		Password:          "hash",
		Role:              model.RoleGuest,
		VerificationToken: tokenStorageForm("email-change-abc"),
		TokenExpiresAt:    &expiresAt,
	}
	if err := db.Create(&u2).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := svc.ResetPassword(context.Background(), "email-change-abc", "newpass123"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("expected ErrInvalidResetToken for email-change token, got %v", err)
	}

	// The rejected tokens are left untouched (not consumed).
	for _, token := range []string{"verify-abc", "email-change-abc"} {
		var stored model.User
		if err := db.Where("verification_token = ?", tokenStorageForm(token)).First(&stored).Error; err != nil {
			t.Fatalf("token %q unexpectedly cleared: %v", token, err)
		}
	}
}

func TestRequestPasswordReset_SendsResetEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	enableSMTP(t, db)
	u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
	captureEmails(t)

	if err := svc.RequestPasswordReset(context.Background(), "alice@example.com", "https", "example.com"); err != nil {
		t.Fatalf("RequestPasswordReset error: %v", err)
	}

	if len(capturedEmails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(capturedEmails))
	}
	email := capturedEmails[0]
	if email.To != "alice@example.com" {
		t.Errorf("expected To alice@example.com, got %q", email.To)
	}
	if email.Subject != "Password Reset Request" {
		t.Errorf("unexpected subject %q", email.Subject)
	}

	tok := emailedToken(t, email)
	if !strings.HasPrefix(tok, model.TokenPrefixReset) {
		t.Fatalf("expected reset token, got %q", tok)
	}

	var stored model.User
	if err := db.First(&stored, u.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.VerificationToken != tokenStorageForm(tok) {
		t.Errorf("expected only the token hash at rest, got %q", stored.VerificationToken)
	}

	// The emailed link actually resets the password.
	if err := svc.ResetPassword(context.Background(), tok, "brandnew123"); err != nil {
		t.Fatalf("ResetPassword error: %v", err)
	}
	if _, _, err := svc.Login(context.Background(), LoginRequest{Email: "alice@example.com", Password: "password123"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected old password invalid after reset")
	}
}

func TestRequestPasswordReset_NoEmailForUnknownUser(t *testing.T) {
	svc, _, db := newTestService(t)
	enableSMTP(t, db)
	captureEmails(t)

	if err := svc.RequestPasswordReset(context.Background(), "ghost@example.com", "https", "example.com"); err != nil {
		t.Fatalf("RequestPasswordReset error: %v", err)
	}
	if len(capturedEmails) != 0 {
		t.Errorf("expected no email for unknown user, got %d", len(capturedEmails))
	}
}
