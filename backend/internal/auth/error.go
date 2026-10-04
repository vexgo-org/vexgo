package auth

import "errors"

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
	ErrInvalidAvatar   = errors.New("avatar must be an /uploads/ path or an http(s) URL")
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
