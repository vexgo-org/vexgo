// Wire types for the auth domain. Swag reads the JSON tags
// to populate the OpenAPI spec; orval turns the generated
// schemas into TypeScript interfaces.
package auth

import (
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// LoginRequestWire is the body of POST /api/auth/login.
type LoginRequestWire struct {
	Email        string `json:"email" binding:"required" example:"alice@example.com"`
	Password     string `json:"password" binding:"required" example:"hunter2"`
	CaptchaID    string `json:"captcha_id" example:"ck_2f4e..."`
	CaptchaToken string `json:"captcha_token" example:"ct_2f4e..."`
	CaptchaX     int    `json:"captcha_x" example:"42"`
	CaptchaY     int    `json:"captcha_y" example:"118"`
}

// LoginUser is the slim user shape returned by /api/auth/login
// (id, username, email, role, avatar, bio, birthday only).
type LoginUser struct {
	ID       uint   `json:"id" example:"42"`
	Username string `json:"username" example:"alice"`
	Email    string `json:"email" example:"alice@example.com"`
	Role     string `json:"role" example:"author"`
	Avatar   string `json:"avatar" example:""`
	Bio      string `json:"bio" example:""`
	Birthday string `json:"birthday" example:""`
}

// LoginResponse is the body of POST /api/auth/login.
type LoginResponse struct {
	Token string    `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  LoginUser `json:"user"`
}

// EmailUnverifiedResponse is the body of POST /api/auth/login
// when the account exists but the email has not been verified.
// The frontend can show a "check your inbox" message and offer
// to resend the verification email.
type EmailUnverifiedResponse struct {
	Message       string `json:"message" example:"Please verify your email address first."`
	EmailVerified bool   `json:"email_verified" example:"false"`
}

// InvalidCredentialsResponse is the body of POST /api/auth/login
// when the email or password is wrong. The shape is intentionally
// uniform with EmailUnverifiedResponse so the frontend can render
// both with the same component.
type InvalidCredentialsResponse struct {
	Message string `json:"message" example:"Invalid email or password"`
}

// RegisterRequestWire is the body of POST /api/auth/register.
type RegisterRequestWire struct {
	Email        string `json:"email" binding:"required,email" example:"newuser@example.com"`
	Password     string `json:"password" binding:"required" example:"hunter2"`
	Username     string `json:"username" binding:"required" example:"newuser"`
	CaptchaID    string `json:"captcha_id" example:"ck_2f4e..."`
	CaptchaToken string `json:"captcha_token" example:"ct_2f4e..."`
	CaptchaX     int    `json:"captcha_x" example:"42"`
	CaptchaY     int    `json:"captcha_y" example:"118"`
}

// RegisterUser is the slim user shape returned by
// /api/auth/register. Mirrors LoginUser but the field set is
// whatever RegisterResult.User holds (typically the public
// profile, not the JWT claim).
type RegisterUser struct {
	ID            uint      `json:"id" example:"42"`
	Username      string    `json:"username" example:"newuser"`
	Email         string    `json:"email" example:"newuser@example.com"`
	Role          string    `json:"role" example:"guest"`
	Avatar        string    `json:"avatar"`
	EmailVerified bool      `json:"email_verified" example:"false"`
	CreatedAt     time.Time `json:"createdAt"`
	Birthday      string    `json:"birthday"`
	Bio           string    `json:"bio"`
}

// RegisterResponse is the body of POST /api/auth/register
// when email verification is required.
type RegisterResponse struct {
	Message              string       `json:"message" example:"Registration successful! Please verify your email."`
	User                 RegisterUser `json:"user"`
	EmailVerified        bool         `json:"email_verified" example:"false"`
	RequiresVerification bool         `json:"requires_verification" example:"true"`
}

// CurrentUserResponse is the body of GET /api/auth/me.
type CurrentUserResponse struct {
	User *model.User `json:"user"`
}

// UpdateProfileRequestWire is the body of PUT /api/auth/profile.
// All fields are optional pointers so a missing field is
// distinct from an empty string.
type UpdateProfileRequestWire struct {
	Username *string `json:"username" example:"alice"`
	Avatar   *string `json:"avatar" example:"https://example.com/avatar.png"`
	Birthday *string `json:"birthday" example:"2000-01-01"`
	Bio      *string `json:"bio" example:"Hello, I'm alice."`
}

// ChangePasswordRequestWire is the body of PUT /api/auth/password.
type ChangePasswordRequestWire struct {
	OldPassword string `json:"oldPassword" binding:"required" example:"hunter2"`
	NewPassword string `json:"newPassword" binding:"required,min=6" example:"correcthorsebatterystaple"`
}

// UpdateSettingsRequestWire is the body of PUT /api/auth/settings.
// Privacy flags are pointers so a missing field is distinct
// from a false value.
type UpdateSettingsRequestWire struct {
	ProfileVisibility *string `json:"profile_visibility" enums:"public,private" example:"public"`
	HideEmail         *bool   `json:"hide_email" example:"false"`
	HideBirthday      *bool   `json:"hide_birthday" example:"false"`
	HideBio           *bool   `json:"hide_bio" example:"false"`
}

// UpdateSettingsResponse is the body of PUT /api/auth/settings.
type UpdateSettingsResponse struct {
	Message string      `json:"message" example:"Settings updated successfully"`
	User    *model.User `json:"user"`
}

// UpdateEmailRequestWire is the body of PUT /api/auth/email.
type UpdateEmailRequestWire struct {
	Email string `json:"email" binding:"required,email" example:"newmail@example.com"`
}

// UpdateEmailPendingResponse is the body of PUT /api/auth/email
// when a verification email has been sent.
type UpdateEmailPendingResponse struct {
	Message string `json:"message" example:"Verification email sent."`
	Pending bool   `json:"pending" example:"true"`
}

// UpdateEmailCompleteResponse is the body of PUT /api/auth/email
// when the change is applied directly (no verification step).
type UpdateEmailCompleteResponse struct {
	Message string `json:"message" example:"Email updated successfully"`
	Pending bool   `json:"pending" example:"false"`
	User    struct {
		Email string `json:"email" example:"newmail@example.com"`
	} `json:"user"`
}

// RequestPasswordResetRequestWire is the body of POST
// /api/auth/password/reset/request.
type RequestPasswordResetRequestWire struct {
	Email string `json:"email" binding:"required,email" example:"alice@example.com"`
}

// ResendVerificationRequestWire is the body of POST
// /api/auth/email/verify/resend.
type ResendVerificationRequestWire struct {
	Email string `json:"email" binding:"required,email" example:"alice@example.com"`
}

// GenericMessageResponse is the uniform `{ "message": "..." }`
// body used by every endpoint whose only payload is a
// confirmation string.
type GenericMessageResponse struct {
	Message string `json:"message" example:"If the email exists, reset link has been sent"`
}

// ResetPasswordRequestWire is the body of POST
// /api/auth/password/reset.
type ResetPasswordRequestWire struct {
	Token    string `json:"token" binding:"required" example:"reset-token-abc123"`
	Password string `json:"password" binding:"required,min=6" example:"correcthorsebatterystaple"`
}

// VerifyEmailResponse is the body of GET /api/auth/email/verify.
type VerifyEmailResponse struct {
	Message        string `json:"message" example:"Email verification successful! You can now log in."`
	RequireRelogin bool   `json:"require_relogin,omitempty" example:"false"`
	NewEmail       string `json:"new_email,omitempty" example:"newmail@example.com"`
}

// VerificationStatusResponse is the body of GET
// /api/auth/email/verify/status.
type VerificationStatusResponse struct {
	EmailVerified bool   `json:"email_verified" example:"true"`
	Email         string `json:"email" example:"alice@example.com"`
}
