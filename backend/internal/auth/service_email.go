package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// VerifyEmail verifies an email address. Tokens prefixed with "email-change-"
// confirm a pending email change; all other tokens verify the initial email.
// It returns whether the token was an email change and the user's new email
// (only meaningful for email changes).
func (s *Service) VerifyEmail(ctx context.Context, token string) (emailChange bool, newEmail string, err error) {
	if strings.HasPrefix(token, model.TokenPrefixEmailChange) {
		slog.Debug("detected email change token, calling ConfirmEmailChange")
		// The pending email must be read before the token is consumed:
		// ConfirmEmailChange clears verification_token together with
		// pending_email, so a lookup afterwards can never find the user.
		user, err := s.repo.FindUserByToken(ctx, token)
		if err != nil {
			slog.Debug("find user by token failed", "err", err)
			return false, "", err
		}
		pendingEmail := user.PendingEmail

		if err := s.ConfirmEmailChange(ctx, token); err != nil {
			slog.Debug("confirm email change failed", "err", err)
			return false, "", err
		}
		slog.Debug("confirm email change succeeded")
		return true, pendingEmail, nil
	}

	slog.Debug("normal email verification token, calling VerifyEmail")
	if err := s.verifyEmailToken(ctx, token); err != nil {
		slog.Debug("verify email failed", "err", err)
		return false, "", err
	}
	slog.Debug("verify email succeeded")
	return false, "", nil
}

// ConfirmEmailChange confirms email change
func (s *Service) ConfirmEmailChange(ctx context.Context, token string) error {
	slog.Debug("confirm email change processing started", "token", token)

	// Only email-change tokens may confirm an email change.
	if !strings.HasPrefix(token, model.TokenPrefixEmailChange) {
		slog.Warn("invalid verification token")
		return ErrInvalidVerificationToken
	}

	user, err := s.repo.FindUserByToken(ctx, token)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			slog.Warn("invalid verification token")
			return ErrInvalidVerificationToken
		}
		slog.Error("failed to query user for email change", "err", err)
		return fmt.Errorf("failed to find user: %w", ErrQueryFailed)
	}
	slog.Debug("found user for email change",
		"userID", user.ID,
		"username", user.Username,
		"pendingEmail", user.PendingEmail,
	)

	// Check if token is expired
	if user.TokenExpiresAt == nil || user.TokenExpiresAt.Before(time.Now()) {
		slog.Warn("email change token expired", "expiresAt", user.TokenExpiresAt)
		return ErrVerificationTokenExpired
	}

	// Check if there is a pending email
	if user.PendingEmail == "" {
		slog.Warn("no pending email in email change request")
		return ErrEmailChangeNoPending
	}

	// Check if the new email is already used by another user
	existingUser, err := s.repo.FindUserByEmailExcluding(ctx, user.PendingEmail, user.ID)
	if err == nil {
		slog.Warn("email already in use by another user", "existingUserID", existingUser.ID)
		return ErrEmailChangeEmailInUse
	}

	// Update email address
	if err := s.repo.UpdateVerifiedEmail(ctx, user.ID, user.PendingEmail); err != nil {
		slog.Error("failed to update email", "err", err)
		return ErrUpdateEmailChange
	}

	slog.Info("email change confirmed successfully", "newEmail", user.PendingEmail)
	return nil
}

// verifyEmailToken verifies email address. Only email-verification tokens are
// accepted: password-reset and email-change tokens are rejected so they
// cannot be cross-used to verify an email.
func (s *Service) verifyEmailToken(ctx context.Context, token string) error {
	if strings.HasPrefix(token, model.TokenPrefixReset) || strings.HasPrefix(token, model.TokenPrefixEmailChange) {
		return ErrInvalidVerificationToken
	}

	user, err := s.repo.FindUserByToken(ctx, token)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInvalidVerificationToken
		}
		return fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}

	// Check if token is expired
	if user.TokenExpiresAt == nil || user.TokenExpiresAt.Before(time.Now()) {
		return ErrVerificationTokenExpired
	}

	// Update user verification status
	if err := s.repo.UpdateUserEmailVerified(ctx, user.ID); err != nil {
		return fmt.Errorf("%w: %w", ErrUpdateUserVerification, err)
	}

	return nil
}

// VerificationStatus returns the email verification status of a user.
func (s *Service) VerificationStatus(ctx context.Context, userID uint) (emailVerified bool, email string, err error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, "", ErrUserNotFound
		}
		return false, "", err
	}
	return user.EmailVerified, user.Email, nil
}
