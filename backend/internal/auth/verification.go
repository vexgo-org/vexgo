package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

const verificationLinkPath string = "/admin/verify-email"

// ResendVerification generates and sends a verification email for an
// unverified account. Absent and verified accounts (and disabled SMTP) return
// `nil` indistinguishable from success so the endpoint cannot be used to
// enumerate account state. Real failures return sentinel errors for internal
// callers: the HTTP handler renders the same generic response for every
// outcome, while Register relies on these sentinels to report delivery truth.
//
// While an account's previous verification token is still live, no new email
// is generated or sent: the live token doubles as a per-account resend
// cooldown (one email per token window), so hammering this endpoint cannot
// flood a mailbox.
func (s *Service) ResendVerification(ctx context.Context, req ResendVerificationRequest) error {
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// Unknown email: identical outcome to an existing one.
		return nil
	case err != nil:
		slog.Error("failed to find user for verification resend", "email", req.Email, "err", err)
		return ErrQueryFailed
	}

	if user.EmailVerified {
		return nil
	}

	if hasLiveVerifyToken(user) {
		slog.Info("verification resend skipped: previous token still valid", "userID", user.ID)
		return nil
	}

	if err := s.sendVerificationEmail(ctx, user, req.Protocol, req.Host); err != nil {
		// Technical details are already logged inside sendVerificationEmail;
		// only the coarse sentinel crosses this boundary.
		return ErrSendEmail
	}
	return nil
}

// hasLiveVerifyToken reports whether the user still holds an unexpired
// email-verification token. The token column is shared with password-reset
// and email-change tokens, so only a "verify-" prefixed one counts: a parked
// reset or email-change token must not suppress a legitimate verification
// resend.
func hasLiveVerifyToken(user *model.User) bool {
	if !strings.HasPrefix(user.VerificationToken, model.TokenPrefixVerify) {
		return false
	}
	return user.TokenExpiresAt != nil && user.TokenExpiresAt.After(time.Now())
}

// sendVerificationEmail sends the email-verification message for a newly
// created user. It returns `nil` when the message was sent (so registration
// requires verification); failures are logged and reported as error so a
// transient SMTP error does not block registration.
func (s *Service) sendVerificationEmail(ctx context.Context, user *model.User, protocol, host string) error {
	enabled, err := s.mailer.Enabled(ctx)
	logger := slog.With("email", user.Email)
	if err != nil {
		logger.Warn("failed to check if SMTP is enabled", "err", err)
		return err
	}
	if !enabled {
		logger.Info("SMTP not enabled, skipping email verification")
		return nil
	}

	token, err := s.GenerateVerificationToken(ctx, user.ID)
	if err != nil {
		logger.Error("failed to generate verification token", "err", err)
		return err
	}

	verificationLink := buildLinkWithToken(protocol, host, verificationLinkPath, token)
	if err := s.mailer.SendVerificationEmail(
		ctx,
		user.Email,
		&mailer.VerificationEmailTemplateData{
			Name: user.Username,
			Link: verificationLink,
		},
	); err != nil {
		logger.Error("failed to send verification email", "err", err)
		return err
	}

	logger.Info("verification email sent successfully")
	return nil
}
