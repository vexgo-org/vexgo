package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

func TestUpdateEmail_SendsChangeEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	enableSMTP(t, db)
	u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
	captureEmails(t)

	pending, err := svc.UpdateEmail(context.Background(), UpdateEmailRequest{
		UserID: u.ID, NewEmail: "fresh@example.com", Protocol: "https", Host: "example.com",
	})
	if err != nil {
		t.Fatalf("UpdateEmail error: %v", err)
	}
	if !pending {
		t.Errorf("expected pending confirmation")
	}

	if len(capturedEmails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(capturedEmails))
	}
	email := capturedEmails[0]
	if email.To != "fresh@example.com" {
		t.Errorf("expected To fresh@example.com (the new email), got %q", email.To)
	}
	if email.Subject != "Confirm Email Change" {
		t.Errorf("unexpected subject %q", email.Subject)
	}
	if !strings.Contains(email.TextBody, "fresh@example.com") || !strings.Contains(email.HTMLBody, "fresh@example.com") {
		t.Errorf("expected new email in email body")
	}

	tok := emailedToken(t, email)
	if !strings.HasPrefix(tok, model.TokenPrefixEmailChange) {
		t.Fatalf("expected email-change token, got %q", tok)
	}

	var stored model.User
	if err := db.First(&stored, u.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.VerificationToken != tokenStorageForm(tok) {
		t.Errorf("expected only the token hash at rest, got %q", stored.VerificationToken)
	}

	// The emailed link actually confirms the email change.
	emailChange, newEmail, err := svc.VerifyEmail(context.Background(), tok)
	if err != nil {
		t.Fatalf("VerifyEmail via link error: %v", err)
	}
	if !emailChange {
		t.Errorf("expected email change")
	}
	if newEmail != "fresh@example.com" {
		t.Errorf("expected new email fresh@example.com, got %q", newEmail)
	}
}

func TestVerifyEmail_InvalidToken(t *testing.T) {
	svc, _, _ := newTestService(t)
	if _, _, err := svc.VerifyEmail(context.Background(), "no-such-token"); err == nil {
		t.Errorf("expected error for unknown token")
	}
}

func TestVerifyEmail_Success(t *testing.T) {
	svc, _, db := newTestService(t)
	expiresAt := time.Now().Add(5 * time.Minute)
	u := model.User{
		Username:          "alice",
		Email:             "alice@example.com",
		VerificationToken: tokenStorageForm("verify-abc"),
		TokenExpiresAt:    &expiresAt,
		EmailVerified:     false,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	emailChange, _, err := svc.VerifyEmail(context.Background(), "verify-abc")
	if err != nil {
		t.Fatalf("VerifyEmail error: %v", err)
	}
	if emailChange {
		t.Errorf("expected non-email-change verification")
	}
	var after model.User
	if err := db.First(&after, u.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if !after.EmailVerified {
		t.Errorf("expected email verified after VerifyEmail")
	}
}

func TestVerifyEmail_EmailChangeUnknownToken(t *testing.T) {
	svc, _, _ := newTestService(t)
	if _, _, err := svc.VerifyEmail(context.Background(), "email-change-nope"); err == nil {
		t.Errorf("expected error for unknown email-change token")
	}
}

func TestVerifyEmail_EmailChangeReturnsNewEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	expiresAt := time.Now().Add(5 * time.Minute)
	u := model.User{
		Username:          "alice",
		Email:             "old@example.com",
		VerificationToken: tokenStorageForm("email-change-abc"),
		TokenExpiresAt:    &expiresAt,
		PendingEmail:      "new@example.com",
		EmailVerified:     true,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	emailChange, newEmail, err := svc.VerifyEmail(context.Background(), "email-change-abc")
	if err != nil {
		t.Fatalf("VerifyEmail error: %v", err)
	}
	if !emailChange {
		t.Errorf("expected email-change verification")
	}
	if newEmail != "new@example.com" {
		t.Errorf("expected pending email returned, got %q", newEmail)
	}

	var after model.User
	if err := db.First(&after, u.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if after.Email != "new@example.com" {
		t.Errorf("expected email updated, got %q", after.Email)
	}
	if after.VerificationToken != "" || after.PendingEmail != "" {
		t.Errorf("expected token and pending email cleared")
	}
}

func TestVerifyEmail_RejectsResetToken(t *testing.T) {
	svc, _, db := newTestService(t)
	expiresAt := time.Now().Add(5 * time.Minute)
	u := model.User{
		Username:          "alice",
		Email:             "alice@example.com",
		VerificationToken: tokenStorageForm("reset-abc"),
		TokenExpiresAt:    &expiresAt,
		EmailVerified:     false,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	// A password-reset token must not be usable to verify an email.
	if _, _, err := svc.VerifyEmail(context.Background(), "reset-abc"); err == nil {
		t.Errorf("expected error for password-reset token")
	}
	var after model.User
	if err := db.First(&after, u.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if after.EmailVerified {
		t.Errorf("email must not be verified by a reset token")
	}
}

func TestVerificationStatus(t *testing.T) {
	svc, _, db := newTestService(t)
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
