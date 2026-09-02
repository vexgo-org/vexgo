package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// LoginResponse is the body of POST /api/auth/login.
type LoginResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

// RegisterResponse is the body of POST /api/auth/register.
type RegisterResponse struct {
	Message              string     `json:"message"`
	User                 model.User `json:"user"`
	EmailVerified        bool       `json:"email_verified"`
	RequiresVerification bool       `json:"requires_verification"`
}

// UserResponse is the body of GET /api/auth/me, PUT /api/auth/profile and
// PUT /api/auth/settings. The message is only present on the settings path.
type UserResponse struct {
	Message string     `json:"message,omitempty"`
	User    model.User `json:"user"`
}

// VerificationStatusResponse is the body of GET /api/auth/verification-status.
type VerificationStatusResponse struct {
	EmailVerified bool   `json:"email_verified"`
	Email         string `json:"email"`
}

// VerifyEmailResponse is the body of GET /api/verify-email. The optional
// fields are only present on the email-change path.
type VerifyEmailResponse struct {
	Message        string `json:"message"`
	RequireRelogin bool   `json:"require_relogin,omitempty"`
	NewEmail       string `json:"new_email,omitempty"`
}

// LoginRequest is the body of POST /api/auth/login.
type LoginRequest struct {
	Email        string `json:"email" binding:"required"`
	Password     string `json:"password" binding:"required"`
	CaptchaID    string `json:"captcha_id,omitempty"`
	CaptchaToken string `json:"captcha_token,omitempty"`
	CaptchaX     int    `json:"captcha_x,omitempty"`
	CaptchaY     int    `json:"captcha_y,omitempty"`
}

// RegisterRequest is the body of POST /api/auth/register.
type RegisterRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required"`
	Username     string `json:"username" binding:"required"`
	CaptchaID    string `json:"captcha_id,omitempty"`
	CaptchaToken string `json:"captcha_token,omitempty"`
	CaptchaX     int    `json:"captcha_x,omitempty"`
	CaptchaY     int    `json:"captcha_y,omitempty"`
}

// UpdateProfileRequest is the body of PUT /api/auth/profile. Pointer fields
// distinguish "not provided" from "set to empty".
type UpdateProfileRequest struct {
	Username *string `json:"username"`
	Avatar   *string `json:"avatar"`
	Birthday *string `json:"birthday"`
	Bio      *string `json:"bio"`
}

// ChangePasswordRequest is the body of PUT /api/auth/password.
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// UpdateSettingsRequest is the body of PUT /api/auth/settings. Pointer fields
// distinguish "not provided" from "set to false/empty".
type UpdateSettingsRequest struct {
	ProfileVisibility *string `json:"profile_visibility"`
	HideEmail         *bool   `json:"hide_email"`
	HideBirthday      *bool   `json:"hide_birthday"`
	HideBio           *bool   `json:"hide_bio"`
}

// UpdateEmailRequest is the body of PUT /api/auth/email.
type UpdateEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RequestPasswordResetRequest is the body of POST /api/auth/request-password-reset.
type RequestPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResendVerificationRequest is the body of POST /api/auth/resend-verification.
type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest is the body of POST /api/auth/reset-password.
type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}
