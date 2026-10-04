package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const resetLinkPath string = "/admin/reset-password"

// ChangePassword verifies the old password and replaces it with the new one,
// incrementing the password version to invalidate existing tokens.
func (s *Service) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return ErrWrongPassword
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return ErrEncryptPassword
	}

	// Increment password version to invalidate old tokens
	user.Password = string(hashed)
	user.PasswordVersion++
	return s.repo.SaveUser(ctx, user)
}

// RequestPasswordReset sends a password reset email when the account exists
// and SMTP is enabled. For security, the response is identical whether or not
// the account exists. protocol and host are used to build the reset link.
func (s *Service) RequestPasswordReset(ctx context.Context, email, protocol, host string) error {
	// Find user
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		// For security reasons, return success even if user doesn't exist to avoid information leakage
		return nil
	}

	// Check if SMTP is enabled
	enabled, err := s.mailer.Enabled(ctx)
	if err != nil || !enabled {
		return nil
	}

	// Generate password reset token
	token, err := s.GeneratePasswordResetToken(ctx, user.ID)
	if err != nil {
		return ErrGenerateResetToken
	}

	// Build reset link - use request protocol and hostname
	resetLink := buildLinkWithToken(protocol, host, resetLinkPath, token)

	// Send email
	if err := s.mailer.SendPasswordResetEmail(
		ctx,
		user.Email,
		&mailer.PasswordResetEmailTemplateData{
			Name: user.Username,
			Link: resetLink,
		},
	); err != nil {
		return ErrSendResetEmail
	}

	return nil
}

// ResetPassword resets a user's password using the emailed reset token.
func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	// Find user with this token
	user, err := s.repo.FindUserByToken(ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidResetToken
		}
		return ErrQueryFailed
	}

	// Only password-reset tokens may reset a password; reject email
	// verification and email-change tokens so they cannot be cross-used.
	if !strings.HasPrefix(token, model.TokenPrefixReset) {
		return ErrInvalidResetToken
	}

	// Check if token has expired
	if user.TokenExpiresAt == nil || user.TokenExpiresAt.Before(time.Now()) {
		return ErrResetTokenExpired
	}

	// Generate hash for new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ErrEncryptPassword
	}

	// Update password and clear reset token
	if err := s.repo.ResetPassword(ctx, user.ID, string(hashed)); err != nil {
		return ErrUpdatePassword
	}

	return nil
}
