package auth

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"vexgo/backend/internal/model"

	"github.com/sirupsen/logrus"
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

	ErrInvalidResetToken  = errors.New("invalid reset token")
	ErrQueryFailed        = errors.New("query failed")
	ErrResetTokenExpired  = errors.New("reset token has expired")
	ErrUpdatePassword     = errors.New("failed to update password")
	ErrGenerateResetToken = errors.New("failed to generate reset token")
	ErrSendResetEmail     = errors.New("failed to send email")
)

// Deps holds the dependencies required by the auth domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Files     FileRemover
	Mailer    model.Mailer
	Captcha   CaptchaChecker
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
	mailer    model.Mailer
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
func (s *Service) Login(ctx context.Context, email, password, captchaID, captchaToken string, captchaX int) (string, *model.User, error) {
	logrus.Info("User login attempt started")

	if err := s.verifyCaptcha(ctx, &verifyCaptchaArgs{
		Token:     captchaToken,
		X:         captchaX,
		Email:     email,
		ID:        captchaID,
		Tolerance: 10,
	}); err != nil {
		return "", nil, err
	}

	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return "", nil, ErrInvalidCredentials
	}

	// Use bcrypt to compare hashed password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	// Check if SMTP is enabled, if so verify email status
	enabled, err := s.mailer.IsEmailEnabled()
	if err == nil && enabled && !user.EmailVerified {
		return "", nil, ErrEmailUnverified
	}

	// Update last login time to invalidate old tokens
	user.LastLoginAt = time.Now()
	if err := s.repo.SaveUser(ctx, user); err != nil {
		logrus.WithError(err).Warn("Failed to update last login time")
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
// RequiresVerification is set. protocol and host are used to build the
// verification link.
func (s *Service) Register(ctx context.Context, email, password, username, captchaID, captchaToken string, captchaX int, protocol, host string) (*RegisterResult, error) {
	logrus.Info("User registration attempt started")

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
		Token:     captchaToken,
		X:         captchaX,
		Email:     email,
		ID:        captchaID,
		Tolerance: 5,
	}); err != nil {
		return nil, err
	}

	// Check if user already exists
	if _, err := s.repo.FindUserByEmail(ctx, email); err == nil {
		logrus.WithField("email", email).Warn("Registration failed: user already exists")
		return nil, ErrUserExists
	}

	// Encrypt password
	logrus.WithField("email", email).Debug("Starting password hashing")
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"email": email,
		}).WithError(err).Error("Failed to hash password")
		return nil, ErrHashPassword
	}
	logrus.WithField("email", email).Debug("Password hashed successfully")

	// Create new user
	newUser := model.User{
		Username:      username,
		Email:         email,
		Password:      string(hashedPassword),
		Role:          model.RoleGuest, // Default role is guest
		EmailVerified: false,
	}

	logrus.WithFields(logrus.Fields{
		"username": username,
		"email":    email,
		"role":     model.RoleGuest,
	}).Info("Creating new user")
	if err := s.repo.CreateUser(ctx, &newUser); err != nil {
		logrus.WithFields(logrus.Fields{
			"username": username,
			"email":    email,
		}).WithError(err).Error("Failed to create user in database")
		return nil, ErrCreateUser
	}
	logrus.WithFields(logrus.Fields{
		"userID":   newUser.ID,
		"username": username,
		"email":    email,
	}).Info("User created successfully")

	// Check if SMTP is enabled, if so send verification email
	enabled, err := s.mailer.IsEmailEnabled()
	if err != nil {
		logrus.WithField("email", newUser.Email).WithError(err).Warn("Failed to check if SMTP is enabled")
	} else if enabled {
		logrus.WithFields(logrus.Fields{
			"userID":   newUser.ID,
			"username": newUser.Username,
			"email":    newUser.Email,
		}).Info("Email verification enabled, generating verification token")

		// Generate verification token
		token, err := s.mailer.GenerateVerificationToken(newUser.ID)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"userID":   newUser.ID,
				"username": newUser.Username,
				"email":    newUser.Email,
			}).WithError(err).Error("Failed to generate verification token")
		} else {
			logrus.WithFields(logrus.Fields{
				"userID":   newUser.ID,
				"username": newUser.Username,
				"email":    newUser.Email,
			}).Debug("Verification token generated successfully")

			// Build verification link - use request protocol and hostname
			verificationLink := fmt.Sprintf("%s://%s/verify-email?token=%s", protocol, host, token)

			logrus.WithFields(logrus.Fields{
				"userID":   newUser.ID,
				"username": newUser.Username,
				"email":    newUser.Email,
				"protocol": protocol,
				"host":     host,
			}).Debug("Sending verification email")

			// Send verification email
			if err := s.mailer.SendVerificationEmail(newUser.Email, newUser.Username, verificationLink); err != nil {
				logrus.WithFields(logrus.Fields{
					"userID":   newUser.ID,
					"username": newUser.Username,
					"email":    newUser.Email,
				}).WithError(err).Error("Failed to send verification email")
			} else {
				logrus.WithFields(logrus.Fields{
					"userID":   newUser.ID,
					"username": newUser.Username,
					"email":    newUser.Email,
				}).Info("Verification email sent successfully")
				return &RegisterResult{User: &newUser, RequiresVerification: true}, nil
			}
		}
	} else {
		logrus.WithField("email", newUser.Email).Info("SMTP not enabled, skipping email verification")
	}

	return &RegisterResult{User: &newUser, RequiresVerification: false}, nil
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

// UpdateProfileRequest carries the optional profile fields.
type UpdateProfileRequest struct {
	Username *string
	Avatar   *string
	Birthday *string
	Bio      *string
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
			logrus.WithError(err).WithField("url", user.Avatar).Warn("Failed to delete old avatar")
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

// UpdateSettingsRequest carries the optional privacy settings.
type UpdateSettingsRequest struct {
	ProfileVisibility *string
	HideEmail         *bool
	HideBirthday      *bool
	HideBio           *bool
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
// It returns whether confirmation is pending. protocol and host are used to
// build the verification link.
func (s *Service) UpdateEmail(ctx context.Context, userID uint, newEmail, protocol, host string) (pending bool, err error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, ErrUserNotFound
		}
		return false, err
	}

	// Check if new email is the same as current email
	if newEmail == user.Email {
		return false, ErrSameEmail
	}

	// Check if new email is already used by another user
	if _, err := s.repo.FindUserByEmailExcluding(ctx, newEmail, userID); err == nil {
		return false, ErrEmailInUse
	}

	// Check if SMTP is enabled
	enabled, err := s.mailer.IsEmailEnabled()
	if err != nil {
		return false, ErrMailConfigCheck
	}

	if enabled {
		// If SMTP enabled, generate email change verification token and send confirmation email
		token, err := s.mailer.GenerateEmailChangeToken(userID, newEmail)
		if err != nil {
			return false, ErrGenerateToken
		}

		// Build verification link
		verificationLink := fmt.Sprintf("%s://%s/verify-email?token=%s", protocol, host, token)

		// Send confirmation email
		if err := s.mailer.SendEmailChangeEmail(user.Email, user.Username, newEmail, verificationLink); err != nil {
			return false, ErrSendEmail
		}

		return true, nil
	}

	// If SMTP not enabled, update email directly
	if err := s.repo.UpdateEmail(ctx, userID, newEmail); err != nil {
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
	enabled, err := s.mailer.IsEmailEnabled()
	if err != nil || !enabled {
		return nil
	}

	// Generate password reset token
	token, err := s.mailer.GeneratePasswordResetToken(user.ID)
	if err != nil {
		return ErrGenerateResetToken
	}

	// Build reset link - use request protocol and hostname
	resetLink := fmt.Sprintf("%s://%s/reset-password?token=%s", protocol, host, token)

	// Send email
	if err := s.mailer.SendPasswordResetEmail(user.Email, user.Username, resetLink); err != nil {
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

	// Check if token has expired
	if user.TokenExpiresAt.Before(time.Now()) {
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
	Email     string
	ID        string
	Tolerance int
}

// verifyCaptcha enforces the sliding-puzzle captcha when it is enabled: it
// checks the required fields, looks the captcha up, verifies expiry and the
// clicked position within tolerance, then marks the captcha as used.
func (s *Service) verifyCaptcha(ctx context.Context, arg *verifyCaptchaArgs) error {
	// Check if captcha verification is enabled
	captchaEnabled, err := s.captchaEnabled(ctx)
	if err != nil {
		return ErrCaptchaCheckFailed
	}

	// If captcha verification is not enabled, return `nil`
	if !captchaEnabled {
		return nil
	}

	// Verify captcha
	logrus.Debug("Captcha verification enabled, validating user captcha")
	if arg.ID == "" || arg.Token == "" || arg.X == 0 {
		logrus.WithFields(logrus.Fields{
			"email":     arg.Email,
			"captchaID": arg.ID,
			"captchaX":  arg.X,
		}).Warn("Captcha verification failed: missing required fields")
		return ErrCaptchaRequired
	}
	// Query captcha
	captcha, err := s.repo.FindCaptcha(ctx, arg.ID, arg.Token)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"captchaID": arg.ID,
			"email":     arg.Email,
		}).Warn("Captcha verification failed: captcha not found or invalid token")
		return ErrCaptchaNotFound
	}

	// Check if expired
	if time.Now().After(captcha.ExpiresAt) {
		logrus.WithFields(logrus.Fields{
			"captchaID": arg.ID,
			"expiresAt": captcha.ExpiresAt,
			"email":     arg.Email,
		}).Warn("Captcha verification failed: captcha expired")
		return ErrCaptchaExpired
	}

	// Verify position (allow certain tolerance)
	if math.Abs(float64(arg.X-captcha.X)) > float64(arg.Tolerance) {
		logrus.WithFields(logrus.Fields{
			"captchaID": arg.ID,
			"userX":     arg.X,
			"correctX":  captcha.X,
			"tolerance": arg.Tolerance,
			"email":     arg.Email,
		}).Warn("Captcha verification failed: incorrect position")
		return ErrCaptchaMismatch
	}

	logrus.WithFields(logrus.Fields{
		"captchaID": arg.ID,
		"email":     arg.Email,
	}).Debug("Captcha verification passed")

	// If captcha has not been used yet, mark it as used
	if !captcha.Used {
		captcha.Used = true
		if err := s.repo.SaveCaptcha(ctx, captcha); err != nil {
			logrus.WithFields(logrus.Fields{
				"captchaID": arg.ID,
				"email":     arg.Email,
			}).WithError(err).Error("Failed to mark captcha as used")
			return ErrCaptchaFailed
		}
		logrus.WithField("captchaID", arg.ID).Debug("Captcha marked as used")
	}
	// If captcha already used, pre-verification successful, pass directly

	return nil
}
