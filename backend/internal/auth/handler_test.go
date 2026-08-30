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

// The endpoint must return byte-identical responses whether the address is
// unknown or a server-side fault happens underneath: any difference lets
// callers probe whether an address exists and is unverified.
func TestResendVerification_UniformResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setup := func(t *testing.T, breakLookup bool) gin.HandlerFunc {
		t.Helper()
		_, _, db := newTestService(t)
		h := NewHandler(Deps{
			DB:        db,
			JWTSecret: testJWTSecret,
			Files:     &fakeFiles{},
			Mailer:    mailer.NewService(mailer.Deps{DB: db}),
			Captcha:   captcha.NewService(captcha.Deps{DB: db}),
		})
		if breakLookup {
			h.svc.repo = &failingRepo{Repository: h.svc.repo, findUserByEmailErr: errors.New("database is unavailable")}
		}
		return h.ResendVerification
	}

	call := func(handler gin.HandlerFunc, email string) (int, string) {
		r := gin.New()
		r.POST("/resend-verification", handler)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/resend-verification",
			strings.NewReader(`{"email":"`+email+`"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	codeUnknown, bodyUnknown := call(setup(t, false), "ghost@example.com")
	codeBroken, bodyBroken := call(setup(t, true), "victim@example.com")

	if codeUnknown != codeBroken || bodyUnknown != bodyBroken {
		t.Errorf("responses must be identical regardless of lookup outcome:\nunknown=%d %q\nbroken=%d %q",
			codeUnknown, bodyUnknown, codeBroken, bodyBroken)
	}
	if codeUnknown != http.StatusOK || !strings.Contains(bodyUnknown, "has been sent") {
		t.Errorf("expected generic 200 response, got %d %q", codeUnknown, bodyUnknown)
	}
}

// POSTs a registration request with attacker-controlled routing metadata
// (spoofed Host + X-Forwarded-Proto) and returns the verification link found
// in the captured email.
func registerAndGetEmailedLink(t *testing.T, db *gorm.DB, baseURL string, behindProxy bool) string {
	t.Helper()
	enableSMTP(t, db)
	captureEmails(t)

	h := NewHandler(Deps{
		DB:                 db,
		JWTSecret:          testJWTSecret,
		Files:              &fakeFiles{},
		Mailer:             mailer.NewService(mailer.Deps{DB: db}),
		Captcha:            captcha.NewService(captcha.Deps{DB: db}),
		BaseURL:            baseURL,
		BehindReverseProxy: behindProxy,
	})
	r := gin.New()
	r.POST("/api/auth/register", h.Register)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register",
		strings.NewReader(`{"email":"victim@example.com","password":"password123","username":"victim"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "evil.example"                    // spoofed Host header
	req.Header.Set("X-Forwarded-Proto", "https") // spoofed forwarding header
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	emails := capturedEmails
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
	return emails[0].TextBody
}

// Emailed links must be built from trusted configuration only: an attacker
// able to set the Host header or X-Forwarded-Proto must never steer password
// reset / verification links to a domain they control.
func TestEmailLinkOrigin_NotPoisonedByRequestHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		baseURL     string
		behindProxy bool
		wantScheme  string
		wantHost    string // empty means "whatever the request carried"
	}{
		{
			name:       "configured origin beats spoofed host and header",
			baseURL:    "https://good.example",
			wantScheme: "https", wantHost: "good.example",
		},
		{
			name:        "configured origin wins even when forwarded headers are honored",
			baseURL:     "http://good.example:8080",
			behindProxy: true,
			wantScheme:  "http", wantHost: "good.example:8080",
		},
		{
			name:       "without config and without proxy, forged proto is ignored",
			wantScheme: "http", wantHost: "evil.example",
		},
		{
			name:        "without config behind proxy, forwarded proto is honored",
			behindProxy: true,
			wantScheme:  "https", wantHost: "evil.example",
		},
		{
			name:       "invalid base_url falls back to request origin",
			baseURL:    "not a url at all",
			wantScheme: "http", wantHost: "evil.example",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, db := newTestService(t)
			body := registerAndGetEmailedLink(t, db, tt.baseURL, tt.behindProxy)

			wantPrefix := tt.wantScheme + "://" + tt.wantHost + "/verify-email?token="
			if !strings.Contains(body, wantPrefix) {
				t.Errorf("email link should start with %q, got:\n%s", wantPrefix, body)
			}
		})
	}
}

func TestParseLinkOrigin(t *testing.T) {
	tests := []struct {
		raw    string
		scheme string
		host   string
	}{
		{"", "", ""},
		{"   ", "", ""},
		{"not a url", "", ""},
		{"example.com", "", ""},             // missing scheme
		{"ftp://files.example.com", "", ""}, // unsupported scheme
		{"https://blog.example.com", "https", "blog.example.com"},
		{"http://localhost:3001/", "http", "localhost:3001"},
		{"https://host.example/base/path", "https", "host.example"}, // path ignored, origin kept
	}
	for _, tt := range tests {
		scheme, host := parseLinkOrigin(tt.raw)
		if scheme != tt.scheme || host != tt.host {
			t.Errorf("parseLinkOrigin(%q) = (%q, %q), want (%q, %q)", tt.raw, scheme, host, tt.scheme, tt.host)
		}
	}
}

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
func (errRepo) MarkCaptchaUsed(context.Context, string, string) error { return nil }
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
