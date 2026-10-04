package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Deps holds the dependencies required by the auth domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Files     FileRemover
	Mailer    *mailer.Service
	Captcha   CaptchaChecker

	// BaseURL is the public site origin (e.g. https://blog.example.com) used
	// to build absolute links inside emails, sourced from BASE_URL / cfg.BaseURL.
	// When set it overrides any request-supplied Host or forwarding header.
	BaseURL string
	// BehindReverseProxy enables honoring X-Forwarded-Proto when BaseURL is
	// not configured. Mirrors cfg.BehindReverseProxy / behind_reverse_proxy.
	BehindReverseProxy bool
	// RateLimitPerMinute caps unauthenticated auth requests (register, login,
	// password reset, verification resend) per client IP per minute; 0 or less
	// disables the limiter.
	RateLimitPerMinute int
	// RateLimit stores the per-IP request budget. nil keeps it in-process; a
	// distributed store shares one budget across instances.
	RateLimit middleware.RateLimitStore
}

// FileRemover is an alias for model.FileRemover kept for backward compatibility.
type FileRemover = model.FileRemover

// CaptchaChecker is the seam for checking whether captcha verification is
// enabled; implemented by the verification domain and injected so it can be
// faked in tests.
type CaptchaChecker interface {
	IsCaptchaEnabled(ctx context.Context) (bool, error)
}

// Service contains the business logic of the auth domain.
type Service struct {
	repo      Repository
	jwtSecret []byte
	files     FileRemover
	mailer    *mailer.Service
	captcha   CaptchaChecker
}

// NewService creates an auth service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{
		repo:      NewRepository(deps.DB),
		jwtSecret: deps.JWTSecret,
		files:     deps.Files,
		mailer:    deps.Mailer,
		captcha:   deps.Captcha,
	}
}

// GetCurrentUser loads a user by ID.
func (s *Service) GetCurrentUser(ctx context.Context, userID uint) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// UpdateProfile updates the optional profile fields, deleting the old avatar
// file when the avatar changes.
func (s *Service) UpdateProfile(ctx context.Context, userID uint, req UpdateProfileRequest) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if req.Avatar != nil {
		// The value is rendered as an <img src> by the public pages and by
		// the theme's comment widget, which writes it straight to the DOM;
		// anything the browser would resolve as a script is refused here
		// (see isSafeAvatarURL) instead of being stored for those sinks.
		if !isSafeAvatarURL(*req.Avatar) {
			return nil, ErrInvalidAvatar
		}

		// Delete the old avatar file — but only when it maps to a media
		// record owned by this user. The stored URL is client-controlled
		// (set by a previous profile update), and Storage.Delete resolves it
		// back to a storage key (for S3, any key in the bucket), so deleting
		// on faith would let a user wipe arbitrary objects by first pointing
		// their avatar at them.
		if *req.Avatar != user.Avatar && user.Avatar != "" {
			s.deleteOldAvatar(ctx, userID, user.Avatar)
		}
		user.Avatar = *req.Avatar
	}

	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Birthday != nil {
		user.Birthday = *req.Birthday
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	if err := s.repo.SaveUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// deleteOldAvatar removes the file behind a replaced avatar when — and only
// when — it is a media file owned by the acting user. Anything else
// (external URLs, records that no longer exist, other users' files, DB
// errors) is logged and skipped: the cleanup is best-effort and must never
// widen into deleting unmanaged or third-party storage objects.
func (s *Service) deleteOldAvatar(ctx context.Context, userID uint, url string) {
	media, err := s.repo.FindMediaByURL(ctx, url)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		slog.Warn("old avatar has no media record, skipping deletion", "userID", userID, "url", url)
		return
	case err != nil:
		slog.Warn("failed to look up old avatar media record, skipping deletion", "userID", userID, "url", url, "err", err)
		return
	}
	if media.UserID != userID {
		slog.Warn("old avatar media record belongs to another user, skipping deletion", "userID", userID, "ownerID", media.UserID, "url", url)
		return
	}
	if err := s.files.Delete(ctx, url); err != nil {
		slog.Warn("failed to delete old avatar", "url", url, "err", err)
	}
}

// UpdateSettings updates the user's privacy settings.
func (s *Service) UpdateSettings(ctx context.Context, userID uint, req UpdateSettingsRequest) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if req.ProfileVisibility != nil {
		user.ProfileVisibility = *req.ProfileVisibility
	}
	if req.HideEmail != nil {
		user.HideEmail = *req.HideEmail
	}
	if req.HideBirthday != nil {
		user.HideBirthday = *req.HideBirthday
	}
	if req.HideBio != nil {
		user.HideBio = *req.HideBio
	}

	if err := s.repo.SaveUser(ctx, user); err != nil {
		return nil, ErrSaveSettings
	}

	return user, nil
}

// UpdateEmail changes the user's email. When SMTP is enabled it requires
// confirmation via an emailed token; otherwise the email is changed directly.
// It returns whether confirmation is pending.
func (s *Service) UpdateEmail(ctx context.Context, req UpdateEmailRequest) (pending bool, err error) {
	user, err := s.repo.FindUserByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrUserNotFound
		}
		return false, err
	}

	// Check if new email is the same as current email
	if req.NewEmail == user.Email {
		return false, ErrSameEmail
	}

	// Check if new email is already used by another user
	if _, err := s.repo.FindUserByEmailExcluding(ctx, req.NewEmail, req.UserID); err == nil {
		return false, ErrEmailInUse
	}

	// Check if SMTP is enabled
	enabled, err := s.mailer.Enabled(ctx)
	if err != nil {
		return false, ErrMailConfigCheck
	}

	if enabled {
		// If SMTP enabled, generate email change verification token and send confirmation email
		token, err := s.GenerateEmailChangeToken(ctx, req.UserID, req.NewEmail)
		if err != nil {
			return false, ErrGenerateToken
		}

		// Build verification link
		verificationLink := buildLinkWithToken(req.Protocol, req.Host, verificationLinkPath, token)

		// Send confirmation email to the new address so the change is only
		// completed after the new mailbox is confirmed.
		if err := s.mailer.SendEmailChangeEmail(
			ctx,
			req.NewEmail,
			&mailer.EmailChangeEmailTemplateData{
				Name:     user.Username,
				NewEmail: req.NewEmail,
				Link:     verificationLink,
			},
		); err != nil {
			return false, ErrSendEmail
		}

		return true, nil
	}

	// If SMTP not enabled, update email directly
	if err := s.repo.UpdateEmail(ctx, req.UserID, req.NewEmail); err != nil {
		return false, err
	}
	return false, nil
}

// secureTokenEntropy is the number of random bytes in every emailed account
// token (reset, verification, email change): 32 bytes give 256 bits, far above
// the 128-bit floor for unguessable one-time tokens.
const secureTokenEntropy = 32

// generateSecureToken returns prefix + 256 bits of crypto/rand entropy encoded
// as unpadded base64url. The token is emailed in this raw form; only its
// storage form (see tokenStorageForm) is persisted.
func generateSecureToken(prefix string) (string, error) {
	b := make([]byte, secureTokenEntropy)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// tokenStorageForm maps a raw emailed token to its at-rest representation:
// the kind prefix followed by the SHA-256 of the rest. Only this form is
// persisted, so a database leak does not expose live one-time tokens, and
// lookups compare hashes instead of plaintext. The prefix is preserved so the
// token-kind checks on stored values (e.g. the verification-resend cooldown)
// keep working. Unknown tokens hash in full (empty prefix) and match nothing.
func tokenStorageForm(rawToken string) string {
	prefix := tokenKindPrefix(rawToken)
	sum := sha256.Sum256([]byte(rawToken[len(prefix):]))
	return prefix + hex.EncodeToString(sum[:])
}

// tokenKindPrefix returns the known account-token prefix of a raw token, or
// "" for unrecognized input. Prefixes are matched longest-first because
// TokenPrefixEmailChange contains an inner hyphen.
func tokenKindPrefix(rawToken string) string {
	for _, prefix := range []string{model.TokenPrefixEmailChange, model.TokenPrefixReset, model.TokenPrefixVerify} {
		if strings.HasPrefix(rawToken, prefix) {
			return prefix
		}
	}
	return ""
}

// GeneratePasswordResetToken generates password reset token
func (s *Service) GeneratePasswordResetToken(ctx context.Context, userID uint) (string, error) {
	token, err := generateSecureToken(model.TokenPrefixReset)
	if err != nil {
		return "", fmt.Errorf("failed to generate password reset token: %w", err)
	}

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database
	if err := s.repo.UpdateUserToken(ctx, userID, tokenStorageForm(token), expiresAt); err != nil {
		return "", fmt.Errorf("failed to save password reset token: %w", err)
	}

	return token, nil
}

// GenerateEmailChangeToken generates email change verification token
func (s *Service) GenerateEmailChangeToken(ctx context.Context, userID uint, newEmail string) (string, error) {
	token, err := generateSecureToken(model.TokenPrefixEmailChange)
	if err != nil {
		return "", fmt.Errorf("failed to generate email change token: %w", err)
	}

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database, also store pending new email
	if err := s.repo.UpdateEmailChangeToken(ctx, userID, newEmail, tokenStorageForm(token), expiresAt); err != nil {
		return "", fmt.Errorf("failed to update email change token: %w", err)
	}

	return token, nil
}

func (s *Service) GenerateVerificationToken(ctx context.Context, userID uint) (string, error) {
	token, err := generateSecureToken(model.TokenPrefixVerify)
	if err != nil {
		return "", fmt.Errorf("failed to generate verification token: %w", err)
	}

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database
	if err := s.repo.UpdateUserToken(ctx, userID, tokenStorageForm(token), expiresAt); err != nil {
		return "", fmt.Errorf("failed to save verification token: %w", err)
	}

	return token, nil
}

func buildLinkWithToken(protocol, host, path, token string) string {
	u := url.URL{
		Scheme: protocol,
		Host:   host,
		Path:   path,
		RawQuery: url.Values{
			"token": []string{token},
		}.Encode(),
	}
	return u.String()
}
