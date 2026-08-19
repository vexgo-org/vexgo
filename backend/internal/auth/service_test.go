package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"vexgo/backend/internal/mailer"
	"vexgo/backend/internal/model"
	"vexgo/backend/internal/verification"

	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var testJWTSecret = []byte("test-secret-for-auth-tests")

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
		Mailer:    mailer.NewMailer(db),
		Captcha:   verification.NewService(verification.Deps{DB: db}),
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
