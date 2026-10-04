package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/wenlng/go-captcha/v2/slide"
	"gorm.io/gorm"
)

// verifyCaptchaArgs carries the captcha inputs for one verification check.
type verifyCaptchaArgs struct {
	Token     string
	X         int
	Y         int
	Email     string
	ID        string
	Tolerance int
}

// verifyCaptcha enforces the sliding-puzzle captcha when it is enabled: it
// checks the required fields, looks the captcha up, verifies expiry and the
// dropped position on both axes within tolerance, then marks the captcha as
// used.
func (s *Service) verifyCaptcha(ctx context.Context, arg *verifyCaptchaArgs) error {
	// Check if captcha verification is enabled
	captchaEnabled, err := s.captchaEnabled(ctx)
	if err != nil {
		slog.Error("failed to check captcha settings", "err", err)
		return ErrCaptchaCheckFailed
	}

	// If captcha verification is not enabled, return `nil`
	if !captchaEnabled {
		return nil
	}

	// Verify captcha
	slog.Debug("captcha verification enabled, validating user captcha")
	if arg.ID == "" || arg.Token == "" || arg.X == 0 || arg.Y == 0 {
		slog.Warn(
			"captcha verification failed: missing required fields",
			"email", arg.Email,
			"captchaID", arg.ID,
			"captchaX", arg.X,
			"captchaY", arg.Y,
		)
		return ErrCaptchaRequired
	}
	// Query captcha
	captcha, err := s.repo.FindCaptcha(ctx, arg.ID, arg.Token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Warn(
				"captcha verification failed: captcha not found or invalid token",
				"captchaID", arg.ID,
				"email", arg.Email,
			)
			return ErrCaptchaNotFound
		}
		// Unexpected lookup failure: report it as an internal error instead
		// of masquerading as a missing challenge.
		slog.Error(
			"captcha verification failed: captcha lookup error",
			"captchaID", arg.ID,
			"email", arg.Email,
			"err", err,
		)
		return ErrCaptchaFailed
	}

	// Check if expired
	if time.Now().After(captcha.ExpiresAt) {
		slog.Warn(
			"captcha verification failed: captcha expired",
			"captchaID", arg.ID,
			"expiresAt", captcha.ExpiresAt,
			"email", arg.Email,
		)
		return ErrCaptchaExpired
	}

	// Verify position (allow certain tolerance on both axes). The challenge
	// is one-shot: a failed attempt invalidates the captcha so the answer
	// cannot be brute-forced through this endpoint within its lifetime.
	//
	// security: the stored answer is deliberately kept out of the logs —
	// aggregated or leaked logs must not allow reconstructing it.
	if !slide.Validate(arg.X, arg.Y, captcha.X, captcha.Y, arg.Tolerance) {
		slog.Warn(
			"captcha verification failed: incorrect position",
			"captchaID", arg.ID,
			"userX", arg.X,
			"userY", arg.Y,
			"tolerance", arg.Tolerance,
			"email", arg.Email,
		)
		if err := s.repo.DeleteCaptcha(ctx, arg.ID, arg.Token); err != nil {
			slog.Error(
				"failed to invalidate captcha after mismatch",
				"captchaID", arg.ID,
				"email", arg.Email,
				"err", err,
			)
			return ErrCaptchaFailed
		}
		return ErrCaptchaMismatch
	}

	slog.Debug(
		"captcha verification passed",
		"captchaID", arg.ID,
		"email", arg.Email,
	)

	// Consume the challenge here, at the sensitive action it protects: the
	// drop-time pre-verification (POST /api/captcha/verify) only marks it
	// used, so without this deletion the same (id, token, x, y) answer could
	// be replayed against login/register until the challenge expires.
	if err := s.repo.DeleteCaptcha(ctx, arg.ID, arg.Token); err != nil {
		slog.Error(
			"failed to consume captcha after successful verification",
			"captchaID", arg.ID,
			"email", arg.Email,
			"err", err,
		)
		return ErrCaptchaFailed
	}

	return nil
}

// captchaEnabled reports whether captcha verification is enabled in the
// general settings, delegating to the verification domain.
func (s *Service) captchaEnabled(ctx context.Context) (bool, error) {
	return s.captcha.IsCaptchaEnabled(ctx)
}
