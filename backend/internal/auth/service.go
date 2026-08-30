package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/wenlng/go-captcha/v2/slide"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler. Each error carries
// the exact message of the original handler response it replaces.
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrCaptchaCheckFailed = errors.New("failed to check captcha settings")
	ErrCaptchaRequired    = errors.New("please complete the captcha verification")
	ErrCaptchaNotFound    = errors.New("captcha does not exist or has expired")
	ErrCaptchaExpired     = errors.New("captcha has expired")
	ErrCaptchaMismatch    = errors.New("verification failed, please try again")
	ErrCaptchaFailed      = errors.New("captcha verification failed")
	ErrEmailUnverified    = errors.New("email not verified")
	ErrTokenGeneration    = errors.New("token generation failed")

	ErrRegistrationDisabled = errors.New("registration is disabled, please contact administrator")
	ErrSettingsCheckFailed  = errors.New("failed to check registration settings")
	ErrUserExists           = errors.New("user already exists")
	ErrHashPassword         = errors.New("failed to hash password")
	ErrCreateUser           = errors.New("failed to create user")

	ErrUserNotFound    = errors.New("user does not exist")
	ErrWrongPassword   = errors.New("current password is incorrect")
	ErrEncryptPassword = errors.New("failed to encrypt password")
	ErrSaveSettings    = errors.New("failed to save settings")
	ErrSameEmail       = errors.New("new email cannot be the same as current email")
	ErrEmailInUse      = errors.New("this email is already used by another user")
	ErrMailConfigCheck = errors.New("failed to check mail configuration")
	ErrGenerateToken   = errors.New("failed to generate verification token")
	ErrSendEmail       = errors.New("failed to send verification email")

	ErrInvalidResetToken        = errors.New("invalid reset token")
	ErrInvalidVerificationToken = errors.New("invalid verification token")
	ErrQueryFailed              = errors.New("query failed")
	ErrResetTokenExpired        = errors.New("reset token has expired")
	ErrUpdatePassword           = errors.New("failed to update password")
	ErrGenerateResetToken       = errors.New("failed to generate reset token")
	ErrSendResetEmail           = errors.New("failed to send email")

	// Email-verification domain sentinels.
	ErrVerificationTokenExpired = errors.New("verification token has expired")
	ErrUpdateUserVerification   = errors.New("failed to update email verification status")
	ErrEmailChangeNoPending     = errors.New("no pending email change")
	ErrEmailChangeEmailInUse    = errors.New("this email is already used by another account")
	ErrUpdateEmailChange        = errors.New("failed to update email")
)

const (
	verificationLinkPath string = "/verify-email"
	resetLinkPath        string = "/reset-password"

	// dummyPasswordSource is the plaintext hashed into dummyPasswordHash. Its
	// value is arbitrary; only its cost matters.
	dummyPasswordSource = "timing-equalizer-not-a-real-password"
)

// dummyPasswordHash is compared against during login when the email address
// does not exist. It costs one bcrypt evaluation at DefaultCost — the same
// work as a real comparison — so response timing cannot reveal which
// addresses are registered.
var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(dummyPasswordSource), bcrypt.DefaultCost)
	if err != nil {
		panic("auth: failed to generate dummy password hash: " + err.Error())
	}
	return hash
}()

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

// captchaEnabled reports whether captcha verification is enabled in the
// general settings, delegating to the verification domain.
func (s *Service) captchaEnabled(ctx context.Context) (bool, error) {
	return s.captcha.IsCaptchaEnabled(ctx)
}

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
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
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
		if err == gorm.ErrRecordNotFound {
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
	slog.Debug("starting password hashing", "email", req.Email)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error(
			"failed to hash password",
			"email", req.Email,
			"err", err,
		)
		return nil, ErrHashPassword
	}
	slog.Debug("password hashed successfully", "email", req.Email)

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

// GetCurrentUser loads a user by ID.
func (s *Service) GetCurrentUser(ctx context.Context, userID uint) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
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
		if err == gorm.ErrRecordNotFound {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// If updating avatar, delete old avatar
	if req.Avatar != nil && *req.Avatar != user.Avatar && user.Avatar != "" {
		// Delete old avatar file
		if err := s.files.Delete(user.Avatar); err != nil {
			// Log error but continue execution to avoid avatar update failure
			slog.Warn(
				"failed to delete old avatar",
				"url", user.Avatar,
				"err", err,
			)
		}
		user.Avatar = *req.Avatar
	} else if req.Avatar != nil {
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

// ChangePassword verifies the old password and replaces it with the new one,
// incrementing the password version to invalidate existing tokens.
func (s *Service) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
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

// UpdateSettings updates the user's privacy settings.
func (s *Service) UpdateSettings(ctx context.Context, userID uint, req UpdateSettingsRequest) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
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
		if err == gorm.ErrRecordNotFound {
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
		if err == gorm.ErrRecordNotFound {
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
		if err := s.repo.MarkCaptchaUsed(ctx, arg.ID, arg.Token); err != nil {
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

	// If captcha has not been used yet, mark it as used. The conditional
	// update makes the claim atomic; an already-used captcha passes either
	// way because the drop-time pre-verification marked it.
	if !captcha.Used {
		if err := s.repo.MarkCaptchaUsed(ctx, arg.ID, arg.Token); err != nil {
			slog.Error(
				"failed to mark captcha as used",
				"captchaID", arg.ID,
				"email", arg.Email,
				"err", err,
			)
			return ErrCaptchaFailed
		}
		slog.Debug("captcha marked as used", "captchaID", arg.ID)
	}
	// If captcha already used, pre-verification successful, pass directly

	return nil
}

// GeneratePasswordResetToken generates password reset token
func (s *Service) GeneratePasswordResetToken(ctx context.Context, userID uint) (string, error) {
	// Generate random token
	token := model.TokenPrefixReset + fmt.Sprintf("%d-%d", userID, time.Now().UnixNano())

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database
	if err := s.repo.UpdateUserToken(ctx, userID, token, expiresAt); err != nil {
		return "", fmt.Errorf("failed to save password reset token: %w", err)
	}

	return token, nil
}

// GenerateEmailChangeToken generates email change verification token
func (s *Service) GenerateEmailChangeToken(ctx context.Context, userID uint, newEmail string) (string, error) {
	// Generate random token
	token := model.TokenPrefixEmailChange + fmt.Sprintf("%d-%d", userID, time.Now().UnixNano())

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database, also store pending new email
	if err := s.repo.UpdateEmailChangeToken(ctx, userID, newEmail, token, expiresAt); err != nil {
		return "", fmt.Errorf("failed to update email change token: %w", err)
	}

	return token, nil
}

func (s *Service) GenerateVerificationToken(ctx context.Context, userID uint) (string, error) {
	// Generate random token (should use more secure method in production)
	token := model.TokenPrefixVerify + fmt.Sprintf("%d-%d", userID, time.Now().UnixNano())

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database
	if err := s.repo.UpdateUserToken(ctx, userID, token, expiresAt); err != nil {
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
