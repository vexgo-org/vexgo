package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Handler exposes the auth domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth

	// linkScheme and linkHost come from the configured public site origin
	// (BASE_URL / cfg.BaseURL). When empty, email links fall back to the
	// request origin with reduced guarantees.
	linkScheme string
	linkHost   string

	// honorForwardedProto mirrors cfg.BehindReverseProxy: X-Forwarded-Proto is
	// only honored when the deployment declares itself to be behind a reverse
	// proxy. Gin's trusted-proxies check filters the client IP but does not
	// sanitize raw header reads, so this gate lives at the read site.
	honorForwardedProto bool
}

// NewHandler creates an auth HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	scheme, host := parseLinkOrigin(deps.BaseURL)
	return &Handler{
		svc:                 NewService(deps),
		mw:                  middleware.NewAuth(deps.DB, deps.JWTSecret),
		linkScheme:          scheme,
		linkHost:            host,
		honorForwardedProto: deps.BehindReverseProxy,
	}
}

// parseLinkOrigin extracts scheme://host from a configured public origin.
// Anything unusable (empty, unparseable, wrong scheme) logs a warning and
// yields empty strings so callers can detect "not configured".
func parseLinkOrigin(raw string) (scheme, host string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		slog.Warn("invalid base_url configured; emailed links will use the request origin", "baseURL", raw)
		return "", ""
	}
	if trimmed := strings.Trim(u.Path, "/"); trimmed != "" {
		slog.Warn("base_url contains a path; only its origin is used for emailed links", "baseURL", raw)
	}
	return u.Scheme, u.Host
}

// emailLinkOrigin derives the protocol and host used to build absolute links
// inside emails (verification, password reset, email change).
//
// The configured site origin always wins: request-supplied values (Host,
// X-Forwarded-Proto) never leak into emails when BASE_URL is set.
// Without configuration, the request origin is used as a degraded fallback:
// X-Forwarded-Proto is trusted only when behind_reverse_proxy is enabled —
// otherwise a client could poison emailed links with a forged header.
func (h *Handler) emailLinkOrigin(c *gin.Context) (protocol, host string) {
	if h.linkHost != "" {
		return h.linkScheme, h.linkHost
	}
	protocol = "http"
	if c.Request.TLS != nil || (h.honorForwardedProto && c.GetHeader("X-Forwarded-Proto") == "https") {
		protocol = "https"
	}
	return protocol, c.Request.Host
}

// Login logs a user in and returns a signed JWT
func (h *Handler) Login(c *gin.Context) {
	slog.Debug("user login attempt started")

	var req struct {
		Email        string `json:"email" binding:"required"`
		Password     string `json:"password" binding:"required"`
		CaptchaID    string `json:"captcha_id"`
		CaptchaToken string `json:"captcha_token"`
		CaptchaX     int    `json:"captcha_x"`
		CaptchaY     int    `json:"captcha_y"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("failed to bind login request JSON", "err", err)
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	slog.Debug("login request parsed successfully", "email", req.Email)

	token, user, err := h.svc.Login(c.Request.Context(), LoginRequest{
		Email:        req.Email,
		Password:     req.Password,
		CaptchaID:    req.CaptchaID,
		CaptchaToken: req.CaptchaToken,
		CaptchaX:     req.CaptchaX,
		CaptchaY:     req.CaptchaY,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrCaptchaCheckFailed):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCaptchaRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCaptchaNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCaptchaExpired), errors.Is(err, ErrCaptchaMismatch):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCaptchaFailed):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		case errors.Is(err, ErrEmailUnverified):
			c.JSON(http.StatusForbidden, gin.H{
				"message":        "Please verify your email address first. Check your inbox and click the verification link, or request to resend the verification email.",
				"email_verified": false,
			})
		case errors.Is(err, ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		case errors.Is(err, ErrTokenGeneration):
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
			"avatar":   user.Avatar,
			"bio":      user.Bio,
			"birthday": user.Birthday,
		},
	})
}

// Register creates a new user account
func (h *Handler) Register(c *gin.Context) {
	slog.Debug("user registration attempt started")

	var req struct {
		Email        string `json:"email" binding:"required,email"`
		Password     string `json:"password" binding:"required"`
		Username     string `json:"username" binding:"required"`
		CaptchaID    string `json:"captcha_id"`
		CaptchaToken string `json:"captcha_token"`
		CaptchaX     int    `json:"captcha_x"`
		CaptchaY     int    `json:"captcha_y"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind registration request JSON", "err", err)
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	slog.Debug(
		"registration request parsed successfully",
		"email", req.Email,
		"username", req.Username,
	)

	protocol, host := h.emailLinkOrigin(c)
	result, err := h.svc.Register(c.Request.Context(), RegisterRequest{
		Email:        req.Email,
		Password:     req.Password,
		Username:     req.Username,
		CaptchaID:    req.CaptchaID,
		CaptchaToken: req.CaptchaToken,
		CaptchaX:     req.CaptchaX,
		CaptchaY:     req.CaptchaY,
		Protocol:     protocol,
		Host:         host,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrSettingsCheckFailed), errors.Is(err, ErrCaptchaCheckFailed):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		case errors.Is(err, ErrRegistrationDisabled):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCaptchaRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCaptchaNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCaptchaExpired), errors.Is(err, ErrCaptchaMismatch):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCaptchaFailed):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUserExists):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrHashPassword):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCreateUser):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		case errors.Is(err, ErrMailConfigCheck), errors.Is(err, ErrSendEmail):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register"})
		}
		return
	}

	if result.RequiresVerification {
		c.JSON(http.StatusCreated, gin.H{
			"message":               "Registration successful! Please verify your email address before logging in. Check your inbox and click the verification link.",
			"user":                  result.User,
			"email_verified":        false,
			"requires_verification": true,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":               "Registration successful",
		"user":                  result.User,
		"email_verified":        result.User.EmailVerified,
		"requires_verification": false,
	})
}

// GetCurrentUser gets the current logged-in user's information
func (h *Handler) GetCurrentUser(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	user, err := h.svc.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// UpdateProfile updates the current user's profile
func (h *Handler) UpdateProfile(c *gin.Context) {
	var req struct {
		Username *string `json:"username"`
		Avatar   *string `json:"avatar"`
		Birthday *string `json:"birthday"`
		Bio      *string `json:"bio"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	userID := middleware.CurrentUserID(c)

	user, err := h.svc.UpdateProfile(c.Request.Context(), userID, UpdateProfileRequest{
		Username: req.Username,
		Avatar:   req.Avatar,
		Birthday: req.Birthday,
		Bio:      req.Bio,
	})
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

// ChangePassword changes the current user's password
func (h *Handler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	userID := middleware.CurrentUserID(c)

	err := h.svc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrWrongPassword):
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case errors.Is(err, ErrEncryptPassword):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// UpdateSettings updates the current user's privacy settings
func (h *Handler) UpdateSettings(c *gin.Context) {
	var req struct {
		ProfileVisibility *string `json:"profile_visibility"`
		HideEmail         *bool   `json:"hide_email"`
		HideBirthday      *bool   `json:"hide_birthday"`
		HideBio           *bool   `json:"hide_bio"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	userID := middleware.CurrentUserID(c)

	user, err := h.svc.UpdateSettings(c.Request.Context(), userID, UpdateSettingsRequest{
		ProfileVisibility: req.ProfileVisibility,
		HideEmail:         req.HideEmail,
		HideBirthday:      req.HideBirthday,
		HideBio:           req.HideBio,
	})
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrSaveSettings) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Settings updated successfully",
		"user":    user,
	})
}

// UpdateEmail changes the current user's email
func (h *Handler) UpdateEmail(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	userID := middleware.CurrentUserID(c)

	protocol, host := h.emailLinkOrigin(c)
	pending, err := h.svc.UpdateEmail(c.Request.Context(), UpdateEmailRequest{
		UserID:   userID,
		NewEmail: req.Email,
		Protocol: protocol,
		Host:     host,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrSameEmail), errors.Is(err, ErrEmailInUse):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrMailConfigCheck), errors.Is(err, ErrGenerateToken), errors.Is(err, ErrSendEmail):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update email"})
		}
		return
	}

	if pending {
		c.JSON(http.StatusOK, gin.H{
			"message": "Verification email sent. Please check your inbox and click the link to complete email change.",
			"pending": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Email updated successfully",
		"pending": false,
		"user": gin.H{
			"email": req.Email,
		},
	})
}

// RequestPasswordReset requests a password reset email
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	protocol, host := h.emailLinkOrigin(c)
	err := h.svc.RequestPasswordReset(c.Request.Context(), req.Email, protocol, host)
	if err != nil {
		switch {
		case errors.Is(err, ErrGenerateResetToken):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		case errors.Is(err, ErrSendResetEmail):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request password reset"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, reset link has been sent"})
}

// ResendVerification sends another verification email for an unverified
// account. The HTTP response is intentionally uniform for every outcome
// (unknown email, verified account, SMTP failure, database fault): any status
// or body difference would let callers probe whether an address exists and is
// unverified. Failures are logged inside the service layer.
func (h *Handler) ResendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	protocol, host := h.emailLinkOrigin(c)
	// Intentional discard (uniform anti-enumeration response above).
	_ = h.svc.ResendVerification(c.Request.Context(), ResendVerificationRequest{
		Email: req.Email, Protocol: protocol, Host: host,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "If the account exists and is not verified, a verification email has been sent.",
	})
}

// ResetPassword resets the password with an emailed token
func (h *Handler) ResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidResetToken):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrQueryFailed):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		case errors.Is(err, ErrResetTokenExpired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrEncryptPassword), errors.Is(err, ErrUpdatePassword):
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	slog.Debug("email verification request received", "hasToken", token != "")

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token cannot be empty"})
		return
	}

	emailChange, newEmail, err := h.svc.VerifyEmail(c.Request.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidVerificationToken):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrVerificationTokenExpired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrEmailChangeNoPending), errors.Is(err, ErrEmailChangeEmailInUse):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrQueryFailed),
			errors.Is(err, ErrUpdateUserVerification),
			errors.Is(err, ErrUpdateEmailChange):
			// Internal failures: log is already emitted by the service; do not
			// leak the underlying error (e.g. DB dial string) to the client.
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		}
		return
	}

	if emailChange {
		if newEmail != "" {
			c.JSON(http.StatusOK, gin.H{
				"message":         "Email change successful! Your new email is now active.",
				"require_relogin": true,
				"new_email":       newEmail,
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"message":         "Email change successful! Your new email is now active.",
				"require_relogin": true,
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Email verification successful! You can now log in.",
	})
}

// GetVerificationStatus gets current user's email verification status
func (h *Handler) GetVerificationStatus(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	emailVerified, email, err := h.svc.VerificationStatus(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"email_verified": emailVerified,
		"email":          email,
	})
}
