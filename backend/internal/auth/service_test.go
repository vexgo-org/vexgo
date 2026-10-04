package auth

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/captcha"
	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
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

// seedMedia inserts a media_files row for the given owner and URL.
func seedMedia(t *testing.T, db *gorm.DB, userID uint, url string) model.MediaFile {
	t.Helper()
	media := model.MediaFile{URL: url, UserID: userID, Type: "image/png"}
	if err := db.Create(&media).Error; err != nil {
		t.Fatalf("failed to seed media: %v", err)
	}
	return media
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
