package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/captcha"
	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var testJWTSecret = []byte("test-secret-for-auth-tests")

// capturedEmails records emails captured by the mailer test seam.
var capturedEmails []capturedEmail

// fakeFiles records deleted URLs.
type fakeFiles struct {
	deleted []string
}

func (f *fakeFiles) Delete(url string) error {
	f.deleted = append(f.deleted, url)
	return nil
}

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

func newTestService(t *testing.T) (*Service, *fakeFiles, *gorm.DB) {
	t.Helper()
	files := &fakeFiles{}
	db := newTestDB(t)
	svc := NewService(Deps{
		DB:        db,
		JWTSecret: testJWTSecret,
		Files:     files,
		Mailer:    mailer.NewService(mailer.Deps{DB: db}),
		Captcha:   captcha.NewService(captcha.Deps{DB: db}),
	})
	return svc, files, db
}

func seedUser(t *testing.T, db *gorm.DB, email, password, role string, verified bool) model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	u := model.User{
		Username:      email,
		Email:         email,
		Password:      string(hash),
		Role:          role,
		EmailVerified: verified,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return u
}

func enableSMTP(t *testing.T, db *gorm.DB) {
	t.Helper()
	cfg := model.SMTPConfig{Enabled: true, Host: "localhost", Port: 25, FromEmail: "a@b.c", FromName: "Test"}
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatalf("failed to seed smtp config: %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	svc, _, db := newTestService(t)
	seedUser(t, db, "alice@example.com", "password123", model.RoleAuthor, true)

	token, user, err := svc.Login(context.Background(), LoginRequest{Email: "alice@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if token == "" {
		t.Errorf("expected non-empty token")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected alice, got %s", user.Email)
	}

	// token is signed with the injected secret and carries the user id
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return testJWTSecret, nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("expected valid token, got err=%v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if uint(claims["user_id"].(float64)) != user.ID {
		t.Errorf("expected user id in token")
	}

	// last login time updated
	var stored model.User
	if err := db.First(&stored, user.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if stored.LastLoginAt.IsZero() {
		t.Errorf("expected last login time updated")
	}
}

func TestLogin_WrongCredentials(t *testing.T) {
	svc, _, db := newTestService(t)
	seedUser(t, db, "alice@example.com", "password123", model.RoleAuthor, true)

	if _, _, err := svc.Login(context.Background(), LoginRequest{Email: "alice@example.com", Password: "wrong"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
	if _, _, err := svc.Login(context.Background(), LoginRequest{Email: "nobody@example.com", Password: "password123"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for unknown email, got %v", err)
	}
}

func TestLogin_EmailUnverified(t *testing.T) {
	svc, _, db := newTestService(t)
	enableSMTP(t, db)
	seedUser(t, db, "alice@example.com", "password123", model.RoleAuthor, false)

	if _, _, err := svc.Login(context.Background(), LoginRequest{Email: "alice@example.com", Password: "password123"}); !errors.Is(err, ErrEmailUnverified) {
		t.Errorf("expected ErrEmailUnverified, got %v", err)
	}
}

func TestLogin_WithCaptcha(t *testing.T) {
	svc, _, db := newTestService(t)
	settings := model.GeneralSettings{CaptchaEnabled: true}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("failed to enable captcha: %v", err)
	}
	seedUser(t, db, "alice@example.com", "password123", model.RoleAuthor, true)

	// missing captcha fields
	if _, _, err := svc.Login(context.Background(), LoginRequest{Email: "alice@example.com", Password: "password123"}); !errors.Is(err, ErrCaptchaRequired) {
		t.Errorf("expected ErrCaptchaRequiredLogin, got %v", err)
	}

	// wrong position
	captcha := model.Captcha{ID: "c1", Token: "t1", X: 100, Y: 50, Width: 60, Height: 60, ExpiresAt: time.Now().Add(5 * time.Minute)}
	if err := db.Create(&captcha).Error; err != nil {
		t.Fatalf("failed to seed captcha: %v", err)
	}
	if _, _, err := svc.Login(context.Background(), LoginRequest{Email: "alice@example.com", Password: "password123", CaptchaID: "c1", CaptchaToken: "t1", CaptchaX: 10}); !errors.Is(err, ErrCaptchaMismatch) {
		t.Errorf("expected ErrCaptchaMismatch, got %v", err)
	}

	// correct position passes
	token, _, err := svc.Login(context.Background(), LoginRequest{Email: "alice@example.com", Password: "password123", CaptchaID: "c1", CaptchaToken: "t1", CaptchaX: 100})
	if err != nil {
		t.Fatalf("Login with captcha error: %v", err)
	}
	if token == "" {
		t.Errorf("expected token")
	}
	// captcha marked used
	var stored model.Captcha
	if err := db.First(&stored, "id = ?", "c1").Error; err != nil {
		t.Fatalf("failed to reload captcha: %v", err)
	}
	if !stored.Used {
		t.Errorf("expected captcha marked as used")
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

// failingRepo decorates Repository to force a failure on FindUserByEmail,
// simulating a transient database error that is NOT a missing record.
type failingRepo struct {
	Repository
	findUserByEmailErr error
}

func (f *failingRepo) FindUserByEmail(ctx context.Context, email string) (*model.User, error) {
	if f.findUserByEmailErr != nil {
		return nil, f.findUserByEmailErr
	}
	return f.Repository.FindUserByEmail(ctx, email)
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

func TestUpdateSettings(t *testing.T) {
	svc, _, db := newTestService(t)
	u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)

	hideEmail := true
	visibility := "private"
	user, err := svc.UpdateSettings(context.Background(), u.ID, UpdateSettingsRequest{
		ProfileVisibility: &visibility,
		HideEmail:         &hideEmail,
	})
	if err != nil {
		t.Fatalf("UpdateSettings error: %v", err)
	}
	if !user.HideEmail || user.ProfileVisibility != "private" {
		t.Errorf("expected settings applied, got %+v", user)
	}

	if _, err := svc.UpdateSettings(context.Background(), 99999, UpdateSettingsRequest{}); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUpdateProfile_DeletesOldAvatar(t *testing.T) {
	svc, files, db := newTestService(t)
	u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
	u.Avatar = "/uploads/old-avatar.png"
	if err := db.Save(&u).Error; err != nil {
		t.Fatalf("failed to set avatar: %v", err)
	}

	newAvatar := "/uploads/new-avatar.png"
	user, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &newAvatar})
	if err != nil {
		t.Fatalf("UpdateProfile error: %v", err)
	}
	if user.Avatar != newAvatar {
		t.Errorf("expected new avatar, got %s", user.Avatar)
	}
	if len(files.deleted) != 1 || files.deleted[0] != "/uploads/old-avatar.png" {
		t.Errorf("expected old avatar deleted, got %v", files.deleted)
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
		VerificationToken: "reset-token",
		TokenExpiresAt:    &expiresAt,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	// invalid token
	if err := svc.ResetPassword(context.Background(), "nope", "newpass123"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("expected ErrInvalidResetToken, got %v", err)
	}

	// expired token (with the correct reset- prefix, so it reaches the
	// expiry check instead of being rejected by the prefix check)
	expiredAt := time.Now().Add(-1 * time.Minute)
	u2 := model.User{
		Username:          "bob",
		Email:             "bob@example.com",
		Password:          "hash",
		Role:              model.RoleGuest,
		VerificationToken: "reset-expired",
		TokenExpiresAt:    &expiredAt,
	}
	if err := db.Create(&u2).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := svc.ResetPassword(context.Background(), "reset-expired", "newpass123"); !errors.Is(err, ErrResetTokenExpired) {
		t.Errorf("expected ErrResetTokenExpired, got %v", err)
	}

	// success: token cleared
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
		VerificationToken: "verify-abc",
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
		VerificationToken: "email-change-abc",
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
		if err := db.Where("verification_token = ?", token).First(&stored).Error; err != nil {
			t.Fatalf("token %q unexpectedly cleared: %v", token, err)
		}
	}
}

func TestIssueJWT(t *testing.T) {
	u := &model.User{ID: 1, Username: "alice", Role: model.RoleAdmin, PasswordVersion: 2}
	token, err := IssueJWT(u, testJWTSecret)
	if err != nil {
		t.Fatalf("IssueJWT error: %v", err)
	}
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return testJWTSecret, nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("expected valid token, got err=%v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["username"] != "alice" || claims["role"] != model.RoleAdmin {
		t.Errorf("unexpected claims: %v", claims)
	}
	if uint(claims["password_version"].(float64)) != 2 {
		t.Errorf("expected password version 2 in claims")
	}
}

// capturedEmail holds the rendered parts of an outgoing email captured by the
// mailer test seam.
type capturedEmail struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// captureEmails installs the mailer capture hook and resets it after the test.
func captureEmails(t *testing.T) {
	t.Helper()
	capturedEmails = nil
	mailer.SetMailCaptureHook(func(to, subject, textBody, htmlBody string) {
		capturedEmails = append(capturedEmails, capturedEmail{to, subject, textBody, htmlBody})
	})
	t.Cleanup(func() { mailer.SetMailCaptureHook(nil) })
}

// extractToken pulls the token query parameter out of an emailed link.
func extractToken(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("failed to parse link %q: %v", link, err)
	}
	tok := u.Query().Get("token")
	if tok == "" {
		t.Fatalf("no token in link %q", link)
	}
	return tok
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

	var stored model.User
	if err := db.First(&stored, result.User.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	wantLink := "https://example.com/verify-email?token=" + stored.VerificationToken
	if !strings.Contains(email.TextBody, wantLink) || !strings.Contains(email.HTMLBody, wantLink) {
		t.Errorf("expected verification link %q in email body", wantLink)
	}

	// The emailed link actually verifies the address.
	tok := extractToken(t, wantLink)
	emailChange, _, err := svc.VerifyEmail(context.Background(), tok)
	if err != nil {
		t.Fatalf("VerifyEmail via link error: %v", err)
	}
	if emailChange {
		t.Errorf("expected normal verification, got email change")
	}
}

func TestRegister_UnverifiedDuplicateResendsVerificationEmail(t *testing.T) {
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
		t.Fatalf("duplicate registration should resend verification: result=%+v err=%v", second, err)
	}
	var count int64
	if err := db.Model(&model.User{}).Where("email = ?", "repeat@example.com").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one user, got %d", count)
	}
	if len(capturedEmails) != 2 {
		t.Fatalf("expected two verification emails, got %d", len(capturedEmails))
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

	var stored model.User
	if err := db.First(&stored, u.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !strings.HasPrefix(stored.VerificationToken, model.TokenPrefixEmailChange) {
		t.Fatalf("expected email-change token, got %q", stored.VerificationToken)
	}
	wantLink := "https://example.com/verify-email?token=" + stored.VerificationToken
	if !strings.Contains(email.TextBody, wantLink) || !strings.Contains(email.HTMLBody, wantLink) {
		t.Errorf("expected change link %q in email body", wantLink)
	}

	// The emailed link actually confirms the email change.
	tok := extractToken(t, wantLink)
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

	var stored model.User
	if err := db.First(&stored, u.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !strings.HasPrefix(stored.VerificationToken, model.TokenPrefixReset) {
		t.Fatalf("expected reset token, got %q", stored.VerificationToken)
	}
	wantLink := "https://example.com/reset-password?token=" + stored.VerificationToken
	if !strings.Contains(email.TextBody, wantLink) || !strings.Contains(email.HTMLBody, wantLink) {
		t.Errorf("expected reset link %q in email body", wantLink)
	}

	// The emailed link actually resets the password.
	tok := extractToken(t, wantLink)
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
		VerificationToken: "verify-abc",
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
		VerificationToken: "email-change-abc",
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
		VerificationToken: "reset-abc",
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
