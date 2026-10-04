package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	// dummyPasswordSource is the plaintext hashed into dummyPasswordHash. Its
	// value is arbitrary; only its cost matters.
	dummyPasswordSource = "timing-equalizer-not-a-real-password"

	// dummyPasswordHash is compared against during login when the email address
	// does not exist. It costs one bcrypt evaluation at DefaultCost — the same
	// work as a real comparison — so response timing cannot reveal which
	// addresses are registered.
	//
	// The literal is a hash of dummyPasswordSource at bcrypt.DefaultCost. Since
	// both inputs are fixed the result is too, so there is nothing to compute at
	// startup — and deriving it at runtime could only fail, in package
	// initialisation, before the logger exists. TestDummyPasswordHashMatchesSource
	// keeps the literal in step with dummyPasswordSource and DefaultCost.
	dummyPasswordHash = "$2a$10$hmu1R6rZHBXkDRgiC38cdeGmpATYlujC3EghVel5waX47kFlpcljK"
)

// Login authenticates a user by email and password and returns a signed JWT
// together with the user record. Captcha is enforced when enabled.
func (s *Service) Login(ctx context.Context, req LoginRequest) (string, *model.User, error) {
	slog.Debug("user login attempt started")

	if err := s.verifyCaptcha(ctx, &verifyCaptchaArgs{
		Token:     req.CaptchaToken,
		X:         req.CaptchaX,
		Y:         req.CaptchaY,
		Email:     req.Email,
		ID:        req.CaptchaID,
		Tolerance: 10,
	}); err != nil {
		return "", nil, err
	}

	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		// Unknown address: burn one bcrypt comparison anyway so this branch
		// costs about as much as the real comparison below; otherwise the
		// timing gap would allow enumerating registered emails.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
		}
		return "", nil, ErrInvalidCredentials
	}

	// Use bcrypt to compare hashed password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	// Check if SMTP is enabled; if so, only verified emails may log in.
	//
	// Deliberate fail-open: if reading the mail configuration fails
	// (transient DB trouble) every user is allowed through instead of
	// locking all logins behind that read. Logging in still requires the
	// correct password (and captcha), so the extra window of exposure for
	// one unverified account is smaller than the availability cost of
	// failing closed. The decision is surfaced via the warning below.
	enabled, err := s.mailer.Enabled(ctx)
	if err != nil {
		slog.Warn(
			"failed to check SMTP status for email verification, failing open",
			"userID", user.ID,
			"emailVerified", user.EmailVerified,
			"err", err,
		)
	} else if enabled && !user.EmailVerified {
		return "", nil, ErrEmailUnverified
	}

	// Update last login time to invalidate old tokens
	user.LastLoginAt = time.Now()
	if err := s.repo.SaveUser(ctx, user); err != nil {
		slog.Warn("failed to update last login time", "err", err)
		// Don't fail the login, just log
	}

	token, err := IssueJWT(user, s.jwtSecret)
	if err != nil {
		return "", nil, ErrTokenGeneration
	}

	return token, user, nil
}
