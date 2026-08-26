package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/captcha"
	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
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
