package auth

// LoginRequest carries the credentials and captcha inputs for Login.
type LoginRequest struct {
	Email        string
	Password     string
	CaptchaID    string
	CaptchaToken string
	CaptchaX     int
	CaptchaY     int
}

// RegisterRequest carries the registration inputs.
type RegisterRequest struct {
	Email        string
	Password     string
	Username     string
	CaptchaID    string
	CaptchaToken string
	CaptchaX     int
	CaptchaY     int
	Protocol     string
	Host         string
}

// UpdateProfileRequest carries the optional profile fields.
type UpdateProfileRequest struct {
	Username *string
	Avatar   *string
	Birthday *string
	Bio      *string
}

// UpdateSettingsRequest carries the optional privacy settings.
type UpdateSettingsRequest struct {
	ProfileVisibility *string
	HideEmail         *bool
	HideBirthday      *bool
	HideBio           *bool
}

// UpdateEmailRequest carries the email change inputs.
type UpdateEmailRequest struct {
	UserID   uint
	NewEmail string
	Protocol string
	Host     string
}

// ResendVerificationRequest carries the email and request origin used to build
// a verification link.
type ResendVerificationRequest struct {
	Email    string
	Protocol string
	Host     string
}
