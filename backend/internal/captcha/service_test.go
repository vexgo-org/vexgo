package captcha

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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
	if err := db.AutoMigrate(&model.Captcha{}, &model.GeneralSettings{}, &model.SMTPConfig{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	return NewService(Deps{DB: db}), db
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

func TestGenerateCaptcha(t *testing.T) {
	svc, db := newTestService(t)
	captcha, err := svc.GenerateCaptcha(context.Background())
	if err != nil {
		t.Fatalf("GenerateCaptcha error: %v", err)
	}
	if captcha.ID == "" || captcha.Token == "" {
		t.Errorf("expected id and token, got %+v", captcha)
	}
	if !strings.HasPrefix(captcha.Image, "data:image/jpeg;base64,") {
		t.Errorf("expected JPEG base64 master image, got prefix %q", captcha.Image[:min(30, len(captcha.Image))])
	}
	if !strings.HasPrefix(captcha.Thumb, "data:image/png;base64,") {
		t.Errorf("expected PNG base64 tile image, got prefix %q", captcha.Thumb[:min(30, len(captcha.Thumb))])
	}
	if captcha.ThumbWidth <= 0 || captcha.ThumbHeight <= 0 {
		t.Errorf("expected positive tile size, got %dx%d", captcha.ThumbWidth, captcha.ThumbHeight)
	}
	if captcha.ThumbX <= 0 || captcha.ThumbY < 0 {
		t.Errorf("expected in-bounds tile position, got (%d, %d)", captcha.ThumbX, captcha.ThumbY)
	}
	if captcha.ThumbX+captcha.ThumbWidth > captchaImageWidth {
		t.Errorf("expected tile to fit the master image, got x %d width %d", captcha.ThumbX, captcha.ThumbWidth)
	}
	if captcha.ThumbY+captcha.ThumbHeight > captchaImageHeight {
		t.Errorf("expected tile to fit the master image, got y %d height %d", captcha.ThumbY, captcha.ThumbHeight)
	}

	// persisted for later verification; the stored answer is the hole
	// position, while thumbX/thumbY is the tile's initial display position
	var stored model.Captcha
	if err := db.First(&stored, "id = ?", captcha.ID).Error; err != nil {
		t.Fatalf("failed to load captcha: %v", err)
	}
	if stored.X <= 0 || stored.X+stored.Width > captchaImageWidth {
		t.Errorf("expected in-bounds hole position x %d", stored.X)
	}
	if stored.Y < 0 || stored.Y+stored.Height > captchaImageHeight {
		t.Errorf("expected in-bounds hole position y %d", stored.Y)
	}
	if stored.Width != captcha.ThumbWidth || stored.Height != captcha.ThumbHeight {
		t.Errorf("expected stored size %dx%d, got %dx%d", captcha.ThumbWidth, captcha.ThumbHeight, stored.Width, stored.Height)
	}
}

// TestGenerateCaptcha_AnswerIsHolePosition guards the coordinate mapping: the
// persisted answer must be the hole position (block.X), not the tile's initial
// display position (block.DX). The two ranges overlap only marginally, so
// assert that the stored answer is not always equal to the display position.
func TestGenerateCaptcha_AnswerIsHolePosition(t *testing.T) {
	svc, db := newTestService(t)
	differ := false
	for range 8 {
		captcha, err := svc.GenerateCaptcha(context.Background())
		if err != nil {
			t.Fatalf("GenerateCaptcha error: %v", err)
		}
		var stored model.Captcha
		if err := db.First(&stored, "id = ?", captcha.ID).Error; err != nil {
			t.Fatalf("failed to load captcha: %v", err)
		}
		if stored.X != captcha.ThumbX {
			differ = true
		}
	}
	if !differ {
		t.Errorf("stored answer always equals the tile display position; expected the hole position")
	}
}

func TestVerifyCaptcha(t *testing.T) {
	svc, db := newTestService(t)
	captcha, err := svc.GenerateCaptcha(context.Background())
	if err != nil {
		t.Fatalf("GenerateCaptcha error: %v", err)
	}
	var stored model.Captcha
	if err := db.First(&stored, "id = ?", captcha.ID).Error; err != nil {
		t.Fatalf("failed to load captcha: %v", err)
	}

	// correct position passes and marks as used
	if err := svc.VerifyCaptcha(context.Background(), captcha.ID, captcha.Token, stored.X, stored.Y); err != nil {
		t.Fatalf("VerifyCaptcha error: %v", err)
	}
	if err := svc.VerifyCaptcha(context.Background(), captcha.ID, captcha.Token, stored.X, stored.Y); !errors.Is(err, ErrCaptchaUsed) {
		t.Errorf("expected ErrCaptchaUsed on second use, got %v", err)
	}

	// wrong x position
	captcha2, err := svc.GenerateCaptcha(context.Background())
	if err != nil {
		t.Fatalf("GenerateCaptcha error: %v", err)
	}
	var stored2 model.Captcha
	if err := db.First(&stored2, "id = ?", captcha2.ID).Error; err != nil {
		t.Fatalf("failed to load captcha: %v", err)
	}
	if err := svc.VerifyCaptcha(context.Background(), captcha2.ID, captcha2.Token, stored2.X+100, stored2.Y); !errors.Is(err, ErrCaptchaMismatch) {
		t.Errorf("expected ErrCaptchaMismatch for wrong x, got %v", err)
	}

	// wrong y position
	if err := svc.VerifyCaptcha(context.Background(), captcha2.ID, captcha2.Token, stored2.X, stored2.Y+100); !errors.Is(err, ErrCaptchaMismatch) {
		t.Errorf("expected ErrCaptchaMismatch for wrong y, got %v", err)
	}

	// unknown captcha
	if err := svc.VerifyCaptcha(context.Background(), "nope", "nope", 10, 10); !errors.Is(err, ErrCaptchaNotFound) {
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
	if err := svc.VerifyCaptcha(context.Background(), "expired-id", "expired-token", 50, 20); !errors.Is(err, ErrCaptchaExpired) {
		t.Errorf("expected ErrCaptchaExpired, got %v", err)
	}
}

func TestVerifyCaptcha_ToleranceBounded(t *testing.T) {
	svc, db := newTestService(t)
	captcha, err := svc.GenerateCaptcha(context.Background())
	if err != nil {
		t.Fatalf("GenerateCaptcha error: %v", err)
	}
	var stored model.Captcha
	if err := db.First(&stored, "id = ?", captcha.ID).Error; err != nil {
		t.Fatalf("failed to load captcha: %v", err)
	}

	// Within the padding window the submission passes
	if err := svc.VerifyCaptcha(context.Background(), captcha.ID, captcha.Token, stored.X+verifyPadding, stored.Y+verifyPadding); err != nil {
		t.Errorf("expected drop within padding to pass, got %v", err)
	}

	// Just outside the padding window on either axis it fails
	captcha2, err := svc.GenerateCaptcha(context.Background())
	if err != nil {
		t.Fatalf("GenerateCaptcha error: %v", err)
	}
	var stored2 model.Captcha
	if err := db.First(&stored2, "id = ?", captcha2.ID).Error; err != nil {
		t.Fatalf("failed to load captcha: %v", err)
	}
	if err := svc.VerifyCaptcha(context.Background(), captcha2.ID, captcha2.Token, stored2.X+verifyPadding*2, stored2.Y); !errors.Is(err, ErrCaptchaMismatch) {
		t.Errorf("expected ErrCaptchaMismatch beyond padding on x, got %v", err)
	}
	captcha3, err := svc.GenerateCaptcha(context.Background())
	if err != nil {
		t.Fatalf("GenerateCaptcha error: %v", err)
	}
	var stored3 model.Captcha
	if err := db.First(&stored3, "id = ?", captcha3.ID).Error; err != nil {
		t.Fatalf("failed to load captcha: %v", err)
	}
	if err := svc.VerifyCaptcha(context.Background(), captcha3.ID, captcha3.Token, stored3.X, stored3.Y+verifyPadding*2); !errors.Is(err, ErrCaptchaMismatch) {
		t.Errorf("expected ErrCaptchaMismatch beyond padding on y, got %v", err)
	}
}
