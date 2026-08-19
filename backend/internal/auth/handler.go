package auth

import (
	"errors"
	"net/http"

	"vexgo/backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Handler exposes the auth domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates an auth HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// requestProtocolAndHost derives the protocol and host for building absolute
// links, honoring X-Forwarded-Proto when behind a reverse proxy.
func requestProtocolAndHost(c *gin.Context) (protocol, host string) {
	protocol = "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		protocol = "https"
	}
	return protocol, c.Request.Host
}

// Login logs a user in and returns a signed JWT
func (h *Handler) Login(c *gin.Context) {
	logrus.Info("User login attempt started")

	var req struct {
		Email        string `json:"email" binding:"required"`
		Password     string `json:"password" binding:"required"`
		CaptchaID    string `json:"captcha_id"`
		CaptchaToken string `json:"captcha_token"`
		CaptchaX     int    `json:"captcha_x"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Warn("Failed to bind login request JSON")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logrus.WithField("email", req.Email).Debug("Login request parsed successfully")

	token, user, err := h.svc.Login(req.Email, req.Password, req.CaptchaID, req.CaptchaToken, req.CaptchaX)
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
	logrus.Info("User registration attempt started")

	var req struct {
		Email        string `json:"email" binding:"required,email"`
		Password     string `json:"password" binding:"required"`
		Username     string `json:"username" binding:"required"`
		CaptchaID    string `json:"captcha_id"`
		CaptchaToken string `json:"captcha_token"`
		CaptchaX     int    `json:"captcha_x"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Warn("Failed to bind registration request JSON")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logrus.WithFields(logrus.Fields{
		"email":    req.Email,
		"username": req.Username,
	}).Debug("Registration request parsed successfully")

	protocol, host := requestProtocolAndHost(c)
	result, err := h.svc.Register(req.Email, req.Password, req.Username, req.CaptchaID, req.CaptchaToken, req.CaptchaX, protocol, host)
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
	if uid, ok := c.Get("userID"); ok {
		userID, ok := uid.(uint)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
			return
		}
		user, err := h.svc.GetCurrentUser(userID)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": user})
		return
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)

	user, err := h.svc.UpdateProfile(userID, UpdateProfileRequest{
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)

	err := h.svc.ChangePassword(userID, req.OldPassword, req.NewPassword)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)

	user, err := h.svc.UpdateSettings(userID, UpdateSettingsRequest{
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)

	protocol, host := requestProtocolAndHost(c)
	pending, err := h.svc.UpdateEmail(userID, req.Email, protocol, host)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	protocol, host := requestProtocolAndHost(c)
	err := h.svc.RequestPasswordReset(req.Email, protocol, host)
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

// ResetPassword resets the password with an emailed token
func (h *Handler) ResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.ResetPassword(req.Token, req.Password)
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
