package auth

import (
	"context"
	"errors"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// RegisterResult carries the outcome of a registration attempt.
type RegisterResult struct {
	User                 *model.User
	RequiresVerification bool
}

// Register creates a new guest user, enforcing registration settings and
// captcha when enabled. When SMTP is enabled a verification email is sent and
// RequiresVerification is set.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	slog.Debug("user registration attempt started")

	// Check if registration is allowed
	settings, err := s.repo.GetGeneralSettings(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Allow registration by default
			settings.RegistrationEnabled = true
		} else {
			return nil, ErrSettingsCheckFailed
		}
	}

	if !settings.RegistrationEnabled {
		return nil, ErrRegistrationDisabled
	}

	if err := s.verifyCaptcha(ctx, &verifyCaptchaArgs{
		Token:     req.CaptchaToken,
		X:         req.CaptchaX,
		Y:         req.CaptchaY,
		Email:     req.Email,
		ID:        req.CaptchaID,
		Tolerance: 5,
	}); err != nil {
		return nil, err
	}

	// Check if user already exists. An unverified account can request a new
	// verification email instead of being treated as a failed registration.
	existingUser, err := s.repo.FindUserByEmail(ctx, req.Email)
	switch {
	case err == nil:
		if !existingUser.EmailVerified {
			enabled, err := s.mailer.Enabled(ctx)
			if err != nil || !enabled {
				if err != nil {
					slog.Warn("failed to check SMTP status during duplicate registration", "email", req.Email, "err", err)
				}
				// Without SMTP there is no verification flow at all — same
				// as fresh registration, the account is usable right away.
				// Claiming a verification email was sent would strand the
				// user waiting for a message nobody can send.
				return &RegisterResult{User: existingUser, RequiresVerification: false}, nil
			}
			if err := s.ResendVerification(ctx, ResendVerificationRequest{
				Email: req.Email, Protocol: req.Protocol, Host: req.Host,
			}); err != nil {
				return nil, err
			}
			return &RegisterResult{User: existingUser, RequiresVerification: true}, nil
		}
		slog.Warn("registration failed: user already exists", "email", req.Email)
		return nil, ErrUserExists

	case errors.Is(err, gorm.ErrRecordNotFound):
		// No existing account; proceed with registration below.

	default:
		// A lookup failure is not evidence that the account is absent: fail
		// closed instead of silently proceeding to create the user.
		slog.Error("failed to look up existing user during registration", "email", req.Email, "err", err)
		return nil, ErrQueryFailed
	}

	// Encrypt password
	slog.Debug("starting password hashing")
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error(
			"failed to hash password",
			"email", req.Email,
			"err", err,
		)
		return nil, ErrHashPassword
	}
	slog.Debug("password hashed successfully")

	// Create new user
	newUser := model.User{
		Username:      req.Username,
		Email:         req.Email,
		Password:      string(hashedPassword),
		Role:          model.RoleGuest, // Default role is guest
		EmailVerified: false,
	}

	slog.Debug(
		"creating new user",
		"username", req.Username,
		"email", req.Email,
		"role", model.RoleGuest,
	)
	if err := s.repo.CreateUser(ctx, &newUser); err != nil {
		slog.Error(
			"failed to create user in database",
			"username", req.Username,
			"email", req.Email,
			"err", err,
		)
		return nil, ErrCreateUser
	}
	slog.Info(
		"user created successfully",
		"userID", newUser.ID,
		"username", req.Username,
		"email", req.Email,
	)

	// Send a verification email when SMTP is enabled; otherwise the account
	// is immediately usable.
	enabled, err := s.mailer.Enabled(ctx)
	if err != nil || !enabled {
		// No SMTP configuration is equivalent to SMTP being disabled.
		return &RegisterResult{User: &newUser, RequiresVerification: false}, nil
	}

	if err := s.sendVerificationEmail(ctx, &newUser, req.Protocol, req.Host); err != nil {
		return nil, ErrSendEmail
	}
	return &RegisterResult{User: &newUser, RequiresVerification: true}, nil
}
