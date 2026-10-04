package auth

import (
	"context"
	"errors"
	"regexp"
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

// seedCaptcha inserts a ready-to-use captcha challenge for login/register tests.
func seedCaptcha(t *testing.T, db *gorm.DB, id, token string, x, y int) model.Captcha {
	t.Helper()
	captcha := model.Captcha{ID: id, Token: token, X: x, Y: y, Width: 60, Height: 60, ExpiresAt: time.Now().Add(5 * time.Minute)}
	if err := db.Create(&captcha).Error; err != nil {
		t.Fatalf("failed to seed captcha: %v", err)
	}
	return captcha
}

// captchaGone reports whether the challenge has been consumed (deleted).
func captchaGone(t *testing.T, db *gorm.DB, id string) bool {
	t.Helper()
	var count int64
	db.Model(&model.Captcha{}).Where("id = ?", id).Count(&count)
	return count == 0
}

// capturedEmails records emails captured by the mailer test seam.
var capturedEmails []capturedEmail

// fakeFiles records deleted URLs.
type fakeFiles struct {
	deleted []string
}

func (f *fakeFiles) Delete(_ context.Context, url string) error {
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
	if err := db.AutoMigrate(&model.User{}, &model.Captcha{}, &model.GeneralSettings{}, &model.SMTPConfig{}, &model.MediaFile{}); err != nil {
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

// failingCaptchaLookupRepo forces FindCaptcha to fail with a non-not-found
// error to exercise the internal-error path.
type failingCaptchaLookupRepo struct {
	Repository
}

func (f failingCaptchaLookupRepo) FindCaptcha(context.Context, string, string) (*model.Captcha, error) {
	return nil, errors.New("database is unavailable")
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

// seedMedia inserts a media_files row for the given owner and URL.
func seedMedia(t *testing.T, db *gorm.DB, userID uint, url string) model.MediaFile {
	t.Helper()
	media := model.MediaFile{URL: url, UserID: userID, Type: "image/png"}
	if err := db.Create(&media).Error; err != nil {
		t.Fatalf("failed to seed media: %v", err)
	}
	return media
}

func TestUpdateProfile_DeletesOldAvatar(t *testing.T) {
	t.Run("owned media record is deleted", func(t *testing.T) {
		svc, files, db := newTestService(t)
		u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
		u.Avatar = "/uploads/old-avatar.png"
		if err := db.Save(&u).Error; err != nil {
			t.Fatalf("failed to set avatar: %v", err)
		}
		seedMedia(t, db, u.ID, "/uploads/old-avatar.png")

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
	})

	// security: the stored avatar URL is client-controlled; pointing it at
	// someone else's media (or an arbitrary S3 URL) must never trigger a
	// delete of that object when the avatar changes again.
	t.Run("media owned by another user is not deleted", func(t *testing.T) {
		svc, files, db := newTestService(t)
		u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
		other := seedUser(t, db, "bob@example.com", "password123", model.RoleGuest, true)
		u.Avatar = "/uploads/victim.png"
		if err := db.Save(&u).Error; err != nil {
			t.Fatalf("failed to set avatar: %v", err)
		}
		seedMedia(t, db, other.ID, "/uploads/victim.png")

		newAvatar := "/uploads/new-avatar.png"
		if _, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &newAvatar}); err != nil {
			t.Fatalf("UpdateProfile error: %v", err)
		}
		if len(files.deleted) != 0 {
			t.Errorf("expected no deletions, got %v", files.deleted)
		}
	})

	t.Run("unknown URL is not deleted", func(t *testing.T) {
		svc, files, db := newTestService(t)
		u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
		u.Avatar = "https://bucket.s3.amazonaws.com/anything/secret.png"
		if err := db.Save(&u).Error; err != nil {
			t.Fatalf("failed to set avatar: %v", err)
		}

		newAvatar := "/uploads/new-avatar.png"
		if _, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &newAvatar}); err != nil {
			t.Fatalf("UpdateProfile error: %v", err)
		}
		if len(files.deleted) != 0 {
			t.Errorf("expected no deletions, got %v", files.deleted)
		}
	})
}

// The stored avatar is rendered as an <img src> by the public pages and by the
// theme's comment widget, so a value the browser would execute as a script
// must be refused instead of stored.
func TestUpdateProfile_RejectsUnsafeAvatar(t *testing.T) {
	for _, tc := range []struct {
		name   string
		avatar string
	}{
		{"javascript scheme", `javascript:document.title="xss"`},
		{"data scheme", "data:text/html,<script>alert(1)</script>"},
		{"vbscript scheme", "vbscript:msgbox(1)"},
		{"mixed-case scheme", "JaVaScRiPt:alert(1)"},
		{"protocol-relative host", "//evil.example.com/avatar.png"},
		{"relative path", "avatar.png"},
		{"path traversal", "/uploads/../../secret.png"},
		{"uploads directory itself", "/uploads/"},
		{"theme asset path", "/theme-assets/evil.svg"},
		{"embedded newline", "java\nscript:alert(1)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, files, db := newTestService(t)
			u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
			u.Avatar = "/uploads/original.png"
			if err := db.Save(&u).Error; err != nil {
				t.Fatalf("failed to set avatar: %v", err)
			}

			avatar := tc.avatar
			_, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &avatar})
			if !errors.Is(err, ErrInvalidAvatar) {
				t.Fatalf("expected ErrInvalidAvatar, got %v", err)
			}

			// The rejected value must not be stored, and the previous avatar
			// file must not have been deleted on the way.
			stored, err := svc.GetCurrentUser(context.Background(), u.ID)
			if err != nil {
				t.Fatalf("failed to reload user: %v", err)
			}
			if stored.Avatar != "/uploads/original.png" {
				t.Errorf("expected the previous avatar to survive, got %q", stored.Avatar)
			}
			if len(files.deleted) != 0 {
				t.Errorf("expected no deletions, got %v", files.deleted)
			}
		})
	}
}

// Upload paths and absolute http(s) URLs stay accepted: the admin SPA sends
// the URL returned by the upload endpoint, which is either form.
func TestUpdateProfile_AcceptsSafeAvatar(t *testing.T) {
	for _, avatar := range []string{
		"",
		"/uploads/new-avatar.png",
		"https://cdn.example.com/avatars/alice.png",
	} {
		svc, _, db := newTestService(t)
		u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)

		value := avatar
		user, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &value})
		if err != nil {
			t.Fatalf("UpdateProfile(%q) error: %v", avatar, err)
		}
		if user.Avatar != avatar {
			t.Errorf("expected avatar %q, got %q", avatar, user.Avatar)
		}
	}
}

// TestGenerateTokens_CryptoRandomAndPrefixed ensures the emailed account
// tokens (reset / verification / email change) carry high entropy instead of
// the former predictable "userID-nanotime" format, and keep their prefixes so
// token kinds stay distinguishable.
func TestGenerateTokens_CryptoRandomAndPrefixed(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	cases := []struct {
		prefix string
		gen    func(ctx context.Context, userID uint) (string, error)
	}{
		{model.TokenPrefixReset, svc.GeneratePasswordResetToken},
		{model.TokenPrefixVerify, svc.GenerateVerificationToken},
	}

	for _, tc := range cases {
		t1, err := tc.gen(ctx, 1)
		if err != nil {
			t.Fatalf("generate token error: %v", err)
		}
		t2, err := tc.gen(ctx, 1)
		if err != nil {
			t.Fatalf("generate token error: %v", err)
		}

		if !strings.HasPrefix(t1, tc.prefix) {
			t.Errorf("expected prefix %q, got %q", tc.prefix, t1)
		}
		if t1 == t2 {
			t.Errorf("expected two tokens for the same user to differ")
		}
		if len(t1) < len(tc.prefix)+43 { // 43 = base64url length of 32 bytes
			t.Errorf("expected >= 256 bits of entropy, token too short: %q (%d chars)", t1, len(t1))
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
// emailedToken extracts the raw link token from a captured email's HTML body.
// Raw tokens exist only in the email; the database holds their storage form
// (see tokenStorageForm).
func emailedToken(t *testing.T, email capturedEmail) string {
	t.Helper()
	m := regexp.MustCompile(`token=([A-Za-z0-9_-]+)`).FindStringSubmatch(email.HTMLBody)
	if m == nil {
		t.Fatal("no token link in email body")
	}
	return m[1]
}
