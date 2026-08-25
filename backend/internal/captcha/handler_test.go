package captcha

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func TestGetVerificationStatus_UserContextUint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, db := newTestService(t)
	u := model.User{
		Username:      "alice",
		Email:         "alice@example.com",
		Role:          model.RoleGuest,
		EmailVerified: true,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	h := NewHandler(Deps{DB: db})

	// The middleware stores the user id as uint in the "user" context map.
	// The handler used to assert `.(float64)`, which always failed and fell
	// through to a 500 — this test locks in the uint assertion.
	r := gin.New()
	r.GET("/verification-status", func(c *gin.Context) {
		c.Set("user", map[string]any{"id": u.ID, "username": u.Username, "role": u.Role})
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

	_, db := newTestService(t)
	h := NewHandler(Deps{DB: db})

	r := gin.New()
	r.GET("/verification-status", func(c *gin.Context) {
		c.Set("user", map[string]any{"id": uint(99999), "username": "ghost", "role": model.RoleGuest})
		h.GetVerificationStatus(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verification-status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}
