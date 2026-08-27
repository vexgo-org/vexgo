package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/captcha"
	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestGetVerificationStatus_UserContextUint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, _, db := newTestService(t)
	u := model.User{
		Username:      "alice",
		Email:         "alice@example.com",
		Role:          model.RoleGuest,
		EmailVerified: true,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	h := NewHandler(Deps{
		DB:        db,
		JWTSecret: testJWTSecret,
		Files:     &fakeFiles{},
		Mailer:    mailer.NewService(mailer.Deps{DB: db}),
		Captcha:   captcha.NewService(captcha.Deps{DB: db}),
	})

	r := gin.New()
	r.GET("/verification-status", func(c *gin.Context) {
		c.Set(middleware.CtxUserIDKey, u.ID)
		h.GetVerificationStatus(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verification-status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp struct {
		EmailVerified bool   `json:"email_verified"`
		Email         string `json:"email"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.EmailVerified || resp.Email != "alice@example.com" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGetVerificationStatus_UserNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, _, db := newTestService(t)
	h := NewHandler(Deps{
		DB:        db,
		JWTSecret: testJWTSecret,
		Files:     &fakeFiles{},
		Mailer:    mailer.NewService(mailer.Deps{DB: db}),
		Captcha:   captcha.NewService(captcha.Deps{DB: db}),
	})

	r := gin.New()
	r.GET("/verification-status", func(c *gin.Context) {
		c.Set(middleware.CtxUserIDKey, uint(99999))
		h.GetVerificationStatus(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verification-status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// errRepo is a stub Repository whose FindUserByToken fails with a non
// ErrRecordNotFound error, simulating an internal data-store failure (e.g. a
// DB dial error). It exercises the leak-prevention path of the VerifyEmail
// handler.
type errRepo struct{}

func (errRepo) FindUserByID(context.Context, uint) (*model.User, error) {
	return nil, errors.New("dial tcp 127.0.0.1:5432: connection refused")
}

func (errRepo) FindUserByEmail(context.Context, string) (*model.User, error) {
	return nil, errors.New("dial tcp 127.0.0.1:5432: connection refused")
}

func (errRepo) FindUserByEmailExcluding(context.Context, string, uint) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}

func (errRepo) FindUserByToken(context.Context, string) (*model.User, error) {
	return nil, errors.New("dial tcp 127.0.0.1:5432: connection refused")
}
func (errRepo) CreateUser(context.Context, *model.User) error { return nil }
func (errRepo) UpdateUserToken(context.Context, uint, string, time.Time) error {
	return nil
}
func (errRepo) SaveUser(context.Context, *model.User) error     { return nil }
func (errRepo) UpdateEmail(context.Context, uint, string) error { return nil }
func (errRepo) UpdateVerifiedEmail(context.Context, uint, string) error {
	return nil
}
func (errRepo) UpdateUserEmailVerified(context.Context, uint) error { return nil }
func (errRepo) ResetPassword(context.Context, uint, string) error   { return nil }
func (errRepo) GetGeneralSettings(context.Context) (model.GeneralSettings, error) {
	return model.GeneralSettings{}, nil
}

func (errRepo) FindCaptcha(context.Context, string, string) (*model.Captcha, error) {
	return nil, gorm.ErrRecordNotFound
}
func (errRepo) SaveCaptcha(context.Context, *model.Captcha) error { return nil }
func (errRepo) UpdateEmailChangeToken(context.Context, uint, string, string, time.Time) error {
	return nil
}

// TestVerifyEmail_HandlerDoesNotLeakInternalErrors ensures an internal
// data-store failure is mapped to a generic 500 and the underlying error
// string (e.g. a DB dial address) is never returned to the client.
func TestVerifyEmail_HandlerDoesNotLeakInternalErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &Service{
		repo:      errRepo{},
		jwtSecret: testJWTSecret,
		mailer:    mailer.NewService(mailer.Deps{DB: newTestDB(t)}),
	}
	h := NewHandler(Deps{
		DB:        newTestDB(t),
		JWTSecret: testJWTSecret,
		Files:     &fakeFiles{},
		Mailer:    svc.mailer,
	})
	// Inject the erroring repository so the handler exercises the
	// internal-error path without a real data store.
	h.svc = svc

	r := gin.New()
	r.GET("/verify-email", h.VerifyEmail)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verify-email?token=email-change-abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, leak := range []string{"dial tcp", "connection refused", "127.0.0.1", "5432"} {
		if strings.Contains(body, leak) {
			t.Errorf("internal detail leaked to client: %q present in body %s", leak, body)
		}
	}
	if !strings.Contains(body, "Failed to verify email") {
		t.Errorf("expected generic message, got body %s", body)
	}
}
