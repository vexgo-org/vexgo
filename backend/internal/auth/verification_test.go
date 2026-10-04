package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Once the previous verification token has expired, a resend goes through
// again.
func TestResendVerification_AfterExpirySendsAgain(t *testing.T) {
	svc, _, db := newTestService(t)
	enableSMTP(t, db)
	captureEmails(t)

	if _, err := svc.Register(context.Background(), RegisterRequest{
		Email: "cool@example.com", Password: "password123", Username: "cool",
		Protocol: "https", Host: "example.com",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if len(capturedEmails) != 1 {
		t.Fatalf("expected initial email, got %d", len(capturedEmails))
	}

	req := ResendVerificationRequest{Email: "cool@example.com", Protocol: "https", Host: "example.com"}
	for range 3 {
		if err := svc.ResendVerification(context.Background(), req); err != nil {
			t.Fatalf("resend during cooldown: %v", err)
		}
	}
	if len(capturedEmails) != 1 {
		t.Fatalf("expected cooldown to suppress resends, got %d emails", len(capturedEmails))
	}

	// Backdate the stored token so the window lapses.
	var u model.User
	if err := db.Where("email = ?", "cool@example.com").First(&u).Error; err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-1 * time.Minute)
	if err := db.Model(&u).Update("token_expires_at", past).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.ResendVerification(context.Background(), req); err != nil {
		t.Fatalf("resend after expiry: %v", err)
	}
	if len(capturedEmails) != 2 {
		t.Fatalf("expected expired token to unlock one more email, got %d", len(capturedEmails))
	}
}

// A live password-reset (or email-change) token sharing the column must not
// block a verification resend.
func TestResendVerification_NonVerifyTokensDoNotCoolDown(t *testing.T) {
	svc, _, db := newTestService(t)
	enableSMTP(t, db)
	captureEmails(t)

	expiresAt := time.Now().Add(5 * time.Minute)
	u := model.User{
		Username:          "bob",
		Email:             "bob@example.com",
		Password:          "hash",
		Role:              model.RoleGuest,
		VerificationToken: tokenStorageForm(model.TokenPrefixReset + "abc"),
		TokenExpiresAt:    &expiresAt,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.ResendVerification(context.Background(), ResendVerificationRequest{
		Email: "bob@example.com", Protocol: "https", Host: "example.com",
	}); err != nil {
		t.Fatalf("resend with parked reset token: %v", err)
	}
	if len(capturedEmails) != 1 {
		t.Fatalf("expected reset token not to cool down resend, got %d emails", len(capturedEmails))
	}
}

func TestResendVerification_SilentForUnknownAndVerified(t *testing.T) {
	svc, _, db := newTestService(t)
	captureEmails(t)

	// Unknown address: silent success, no email sent.
	if err := svc.ResendVerification(context.Background(), ResendVerificationRequest{
		Email: "ghost@example.com", Protocol: "https", Host: "example.com",
	}); err != nil {
		t.Errorf("unknown address must be silent, got %v", err)
	}

	// Verified address: silent success, no email sent.
	seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
	if err := svc.ResendVerification(context.Background(), ResendVerificationRequest{
		Email: "alice@example.com", Protocol: "https", Host: "example.com",
	}); err != nil {
		t.Errorf("verified address must be silent, got %v", err)
	}

	if len(capturedEmails) != 0 {
		t.Errorf("expected no emails, got %d", len(capturedEmails))
	}
}

func TestResendVerification_DbErrorOnLookup(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.repo = &failingRepo{Repository: svc.repo, findUserByEmailErr: errors.New("database is unavailable")}

	if err := svc.ResendVerification(context.Background(), ResendVerificationRequest{Email: "x@example.com"}); !errors.Is(err, ErrQueryFailed) {
		t.Errorf("expected ErrQueryFailed, got %v", err)
	}
}

// SMTP is enabled but points at an unreachable host; delivery must surface as
// the coarse ErrSendEmail sentinel instead of a raw transport error.
func TestResendVerification_DeliveryFailureReturnsSentinel(t *testing.T) {
	svc, _, db := newTestService(t)
	enableSMTP(t, db)
	seedUser(t, db, "bob@example.com", "password123", model.RoleGuest, false)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := svc.ResendVerification(ctx, ResendVerificationRequest{
		Email: "bob@example.com", Protocol: "https", Host: "example.com",
	}); !errors.Is(err, ErrSendEmail) {
		t.Errorf("expected ErrSendEmail, got %v", err)
	}
}
