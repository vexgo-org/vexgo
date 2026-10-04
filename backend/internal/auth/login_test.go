package auth

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

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

// Logging in with an unknown address must still pay for one bcrypt
// comparison: if that branch returns after a bare DB lookup, the response-time
// gap versus a real comparison lets attackers enumerate registered emails.
// Compares medians of both failure paths (relative, not absolute) so the test
// is stable across machines.
func TestLogin_UnknownEmailRunsDummyHashComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive")
	}
	svc, _, db := newTestService(t)
	seedUser(t, db, "alice@example.com", "password123", model.RoleAuthor, true)

	sample := func(email, password string) time.Duration {
		start := time.Now()
		if _, _, err := svc.Login(context.Background(), LoginRequest{Email: email, Password: password}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials for %q, got %v", email, err)
		}
		return time.Since(start)
	}

	var unknown, wrongPassword []time.Duration
	for range 9 {
		unknown = append(unknown, sample("ghost@example.com", "password123"))
		wrongPassword = append(wrongPassword, sample("alice@example.com", "wrong-password"))
	}
	slices.Sort(unknown)
	slices.Sort(wrongPassword)
	medianUnknown := unknown[len(unknown)/2]
	medianWrongPassword := wrongPassword[len(wrongPassword)/2]

	if medianUnknown < medianWrongPassword/4 {
		t.Errorf(
			"unknown-email login too fast to include a bcrypt comparison: %v vs %v",
			medianUnknown, medianWrongPassword,
		)
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

	loginReq := func(captchaID, token string, x, y int) LoginRequest {
		return LoginRequest{
			Email:        "alice@example.com",
			Password:     "password123",
			CaptchaID:    captchaID,
			CaptchaToken: token,
			CaptchaX:     x,
			CaptchaY:     y,
		}
	}

	// missing captcha fields
	if _, _, err := svc.Login(context.Background(), loginReq("", "", 0, 0)); !errors.Is(err, ErrCaptchaRequired) {
		t.Errorf("expected ErrCaptchaRequiredLogin, got %v", err)
	}

	// missing captcha y coordinate
	captcha := seedCaptcha(t, db, "c1", "t1", 100, 50)

	if _, _, err := svc.Login(context.Background(), loginReq("c1", "t1", 100, 0)); !errors.Is(err, ErrCaptchaRequired) {
		t.Errorf("expected ErrCaptchaRequired for missing y, got %v", err)
	}

	// wrong x position — the failed attempt consumes the challenge
	if _, _, err := svc.Login(context.Background(), loginReq("c1", "t1", 10, 50)); !errors.Is(err, ErrCaptchaMismatch) {
		t.Errorf("expected ErrCaptchaMismatch for wrong x, got %v", err)
	}
	if !captchaGone(t, db, captcha.ID) {
		t.Errorf("expected failed-attempt captcha to be invalidated")
	}

	// wrong y position (fresh challenge — the previous one was consumed)
	c2 := seedCaptcha(t, db, "c1b", "t1b", 100, 50)
	if _, _, err := svc.Login(context.Background(), loginReq("c1b", "t1b", 100, 20)); !errors.Is(err, ErrCaptchaMismatch) {
		t.Errorf("expected ErrCaptchaMismatch for wrong y, got %v", err)
	}
	if !captchaGone(t, db, c2.ID) {
		t.Errorf("expected failed-attempt captcha to be invalidated")
	}

	// correct position passes (fresh challenge)
	seedCaptcha(t, db, "c1c", "t1c", 100, 50)
	token, _, err := svc.Login(context.Background(), loginReq("c1c", "t1c", 100, 50))
	if err != nil {
		t.Fatalf("Login with captcha error: %v", err)
	}
	if token == "" {
		t.Errorf("expected token")
	}
	// the challenge is consumed at the sensitive action, not merely marked used
	if !captchaGone(t, db, "c1c") {
		t.Errorf("expected consumed captcha to be deleted")
	}

	// security: replaying the same (id, token, x, y) answer must fail — the
	// consumed challenge no longer exists.
	if _, _, err := svc.Login(context.Background(), loginReq("c1c", "t1c", 100, 50)); !errors.Is(err, ErrCaptchaNotFound) {
		t.Errorf("expected ErrCaptchaNotFound for replayed captcha, got %v", err)
	}
}

// TestLogin_CaptchaLookupFailureFailsClosed checks that an unexpected captcha
// lookup failure surfaces as ErrCaptchaFailed instead of masquerading as a
// missing challenge.
func TestLogin_CaptchaLookupFailureFailsClosed(t *testing.T) {
	svc, _, db := newTestService(t)
	if err := db.Create(&model.GeneralSettings{CaptchaEnabled: true}).Error; err != nil {
		t.Fatalf("failed to enable captcha: %v", err)
	}
	svc.repo = failingCaptchaLookupRepo{Repository: svc.repo}

	if _, _, err := svc.Login(context.Background(), LoginRequest{
		Email: "alice@example.com", Password: "password123",
		CaptchaID: "c1", CaptchaToken: "t1", CaptchaX: 100, CaptchaY: 50,
	}); !errors.Is(err, ErrCaptchaFailed) {
		t.Errorf("expected ErrCaptchaFailed on captcha lookup failure, got %v", err)
	}
}

// TestDummyPasswordHashMatchesSource pins the literal dummyPasswordHash to
// dummyPasswordSource at bcrypt.DefaultCost. Login hashes against it for an
// unknown email so the response costs the same as a real comparison; a stale
// literal would either stop matching, making bcrypt bail out early, or cost
// less than a real comparison. Either way the timing signal returns.
func TestDummyPasswordHashMatchesSource(t *testing.T) {
	if err := bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(dummyPasswordSource)); err != nil {
		t.Fatalf("dummyPasswordHash no longer matches dummyPasswordSource: %v", err)
	}

	cost, err := bcrypt.Cost([]byte(dummyPasswordHash))
	if err != nil {
		t.Fatalf("dummyPasswordHash is not a valid bcrypt hash: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Errorf("dummyPasswordHash cost = %d, want %d so an unknown account costs the same as a real one",
			cost, bcrypt.DefaultCost)
	}
}
