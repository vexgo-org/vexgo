package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// LoginRequest is the body of POST /api/auth/login.
type LoginRequest struct {
	Email        string `json:"email" required:"" format:"email" doc:"User email"`
	Password     string `json:"password" required:"" doc:"User password"`
	CaptchaID    string `json:"captcha_id,omitempty" doc:"Optional captcha ID"`
	CaptchaToken string `json:"captcha_token,omitempty" doc:"Optional captcha verification token"`
	CaptchaX     int    `json:"captcha_x,omitempty"`
	CaptchaY     int    `json:"captcha_y,omitempty"`
}

// LoginResponse is the body of POST /api/auth/login.
type LoginResponse struct {
	Token string     `json:"token" doc:"Signed JWT for subsequent calls"`
	User  model.User `json:"user" doc:"The authenticated user"`
}

// RegisterRequest is the body of POST /api/auth/register.
type RegisterRequest struct {
	Email        string `json:"email" required:"" format:"email"`
	Password     string `json:"password" required:"" minLength:"6"`
	Username     string `json:"username" required:""`
	CaptchaID    string `json:"captcha_id,omitempty"`
	CaptchaToken string `json:"captcha_token,omitempty"`
	CaptchaX     int    `json:"captcha_x,omitempty"`
	CaptchaY     int    `json:"captcha_y,omitempty"`
}

// RegisterResponse is the body of POST /api/auth/register.
type RegisterResponse struct {
	Message              string     `json:"message"`
	User                 model.User `json:"user"`
	EmailVerified        bool       `json:"email_verified"`
	RequiresVerification bool       `json:"requires_verification" doc:"True when the new account must verify its email before login"`
}

// UserResponse is the body of GET /api/auth/me, /me, /user.
type UserResponse struct {
	User    model.User `json:"user"`
	Message string     `json:"message,omitempty" doc:"Set on PUT responses"`
}

// UpdateProfileRequest is the body of PUT /api/auth/profile.
type UpdateProfileRequest struct {
	Username *string `json:"username,omitempty"`
	Avatar   *string `json:"avatar,omitempty"`
	Birthday *string `json:"birthday,omitempty"`
	Bio      *string `json:"bio,omitempty"`
}

// ChangePasswordRequest is the body of PUT /api/auth/password.
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" required:""`
	NewPassword string `json:"newPassword" required:"" minLength:"6"`
}

// UpdateEmailRequest is the body of PUT /api/auth/email.
type UpdateEmailRequest struct {
	Email string `json:"email" required:"" format:"email"`
}

// UpdateEmailResponse is the body of PUT /api/auth/email.
type UpdateEmailResponse struct {
	Message string      `json:"message"`
	Pending bool        `json:"pending" doc:"True when a verification email is required before the new address is active"`
	User    *UserSummary `json:"user,omitempty" doc:"Set when the email is updated without verification"`
}

// UserSummary is a tiny user shape used in update responses.
type UserSummary struct {
	Email string `json:"email"`
}

// UpdateSettingsRequest is the body of PUT /api/auth/settings.
type UpdateSettingsRequest struct {
	ProfileVisibility *string `json:"profile_visibility,omitempty" doc:"public|registered|private"`
	HideEmail         *bool   `json:"hide_email,omitempty"`
	HideBirthday      *bool   `json:"hide_birthday,omitempty"`
	HideBio           *bool   `json:"hide_bio,omitempty"`
}

// VerifyEmailResponse is the body of GET /api/verify-email.
type VerifyEmailResponse struct {
	Message       string `json:"message"`
	RequireRelogin bool   `json:"requireRelogin,omitempty" doc:"True when the verification is for an email change"`
	NewEmail      string `json:"newEmail,omitempty"`
}

// VerificationStatusResponse is the body of GET /api/auth/verification-status.
type VerificationStatusResponse struct {
	EmailVerified bool   `json:"email_verified"`
	Email         string `json:"email"`
}

// RequestPasswordResetRequest is the body of POST /api/auth/request-password-reset.
type RequestPasswordResetRequest struct {
	Email string `json:"email" required:"" format:"email"`
}

// ResendVerificationRequest is the body of POST /api/auth/resend-verification.
type ResendVerificationRequest struct {
	Email string `json:"email" required:"" format:"email"`
}

// ResetPasswordRequest is the body of POST /api/auth/reset-password.
type ResetPasswordRequest struct {
	Token    string `json:"token" required:"" doc:"Password-reset token emailed to the user"`
	Password string `json:"password" required:"" minLength:"6"`
}
