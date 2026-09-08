package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
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

	// rateLimitPerMinute caps unauthenticated credential endpoints per client
	// IP; mirrored from deps.RateLimitPerMinute.
	rateLimitPerMinute int

	// rateLimit stores the per-IP request budget; nil keeps it in-process.
	rateLimit middleware.RateLimitStore
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
		rateLimitPerMinute:  deps.RateLimitPerMinute,
		rateLimit:           deps.RateLimit,
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

// Login godoc
//
//	@Summary		Log in
//	@Description	Verifies the email and password, returns a signed
//	@Description	JWT for use in the Authorization header. The
//	@Description	captcha fields are required when the server is
//	@Description	configured to require captcha on login.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequestWire	true	"login payload"
//	@Success		200		{object}	LoginResponse
//	@Failure		400		{object}	api.ErrorResponse	"validation / captcha failed"
//	@Failure		401		{object}	InvalidCredentialsResponse
//	@Failure		403		{object}	EmailUnverifiedResponse	"email not verified"
//	@Failure		404		{object}	api.ErrorResponse		"captcha not found"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	slog.Debug("user login attempt started")

	var req LoginRequestWire
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("failed to bind login request JSON", "err", err)
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	slog.Debug("login request parsed successfully", "email", req.Email)

	token, user, err := h.svc.Login(c.Request.Context(), LoginRequest(req))
	if err != nil {
		switch {
		case errors.Is(err, ErrCaptchaCheckFailed):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrCaptchaRequired):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrCaptchaNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrCaptchaExpired), errors.Is(err, ErrCaptchaMismatch):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrCaptchaFailed):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrEmailUnverified):
			c.JSON(http.StatusForbidden, EmailUnverifiedResponse{
				Message:       "Please verify your email address first. Check your inbox and click the verification link, or request to resend the verification email.",
				EmailVerified: false,
			})
		case errors.Is(err, ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, InvalidCredentialsResponse{Message: err.Error()})
		case errors.Is(err, ErrTokenGeneration):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to generate token"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to login"})
		}
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User: LoginUser{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
			Avatar:   user.Avatar,
			Bio:      user.Bio,
			Birthday: user.Birthday,
		},
	})
}

// Register godoc
//
//	@Summary		Register a new account
//	@Description	Creates a new user account. The captcha fields are
//	@Description	required when the server is configured to require
//	@Description	captcha on registration. When email verification
//	@Description	is required, the response carries
//	@Description	`requires_verification: true` and the new account
//	@Description	cannot log in until it is verified.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RegisterRequestWire	true	"registration payload"
//	@Success		201		{object}	RegisterResponse
//	@Failure		400		{object}	api.ErrorResponse	"validation / captcha failed"
//	@Failure		403		{object}	api.ErrorResponse	"registration disabled"
//	@Failure		404		{object}	api.ErrorResponse	"captcha not found"
//	@Failure		409		{object}	api.ErrorResponse	"user already exists"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	slog.Debug("user registration attempt started")

	var req RegisterRequestWire
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind registration request JSON", "err", err)
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
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
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrRegistrationDisabled):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrCaptchaRequired):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrCaptchaNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrCaptchaExpired), errors.Is(err, ErrCaptchaMismatch):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrCaptchaFailed):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrUserExists):
			c.JSON(http.StatusConflict, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrHashPassword):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrCreateUser):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrMailConfigCheck), errors.Is(err, ErrSendEmail):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to register"})
		}
		return
	}

	if result.RequiresVerification {
		c.JSON(http.StatusCreated, RegisterResponse{
			Message:              "Registration successful! Please verify your email address before logging in. Check your inbox and click the verification link.",
			User:                 userToRegisterUser(result.User),
			EmailVerified:        false,
			RequiresVerification: true,
		})
		return
	}

	c.JSON(http.StatusCreated, RegisterResponse{
		Message:              "Registration successful",
		User:                 userToRegisterUser(result.User),
		EmailVerified:        result.User.EmailVerified,
		RequiresVerification: false,
	})
}

// userToRegisterUser projects a model.User into the slim
// RegisterUser shape.
func userToRegisterUser(u *model.User) RegisterUser {
	if u == nil {
		return RegisterUser{}
	}
	return RegisterUser{
		ID:            u.ID,
		Username:      u.Username,
		Email:         u.Email,
		Role:          u.Role,
		Avatar:        u.Avatar,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		Birthday:      u.Birthday,
		Bio:           u.Bio,
	}
}

// GetCurrentUser godoc
//
//	@Summary	Get the authenticated user
//	@Tags		auth
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	CurrentUserResponse
//	@Failure	401	{object}	api.ErrorResponse	"not logged in"
//	@Failure	404	{object}	api.ErrorResponse	"user not found"
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/auth/me [get]
func (h *Handler) GetCurrentUser(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "Not logged in"})
		return
	}

	user, err := h.svc.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to get user"})
		return
	}
	c.JSON(http.StatusOK, CurrentUserResponse{User: user})
}

// UpdateProfile godoc
//
//	@Summary		Update profile fields
//	@Description	Updates the authenticated user's username, avatar,
//	@Description	birthday, and bio. Any field left out of the
//	@Description	request is left unchanged.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		UpdateProfileRequestWire	true	"profile fields to update"
//	@Success		200		{object}	CurrentUserResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid payload"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		404		{object}	api.ErrorResponse	"user not found"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/auth/profile [put]
func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequestWire
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	userID := middleware.CurrentUserID(c)

	user, err := h.svc.UpdateProfile(c.Request.Context(), userID, UpdateProfileRequest(req))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, CurrentUserResponse{User: user})
}

// ChangePassword godoc
//
//	@Summary	Change the authenticated user's password
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		ChangePasswordRequestWire	true	"old and new password"
//	@Success	200		{object}	GenericMessageResponse
//	@Failure	400		{object}	api.ErrorResponse	"invalid payload"
//	@Failure	400		{object}	api.ErrorResponse	"old password is wrong"
//	@Failure	404		{object}	api.ErrorResponse	"user not found"
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/auth/password [put]
func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequestWire
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	userID := middleware.CurrentUserID(c)

	err := h.svc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrWrongPassword):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrEncryptPassword):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to change password"})
		}
		return
	}

	c.JSON(http.StatusOK, GenericMessageResponse{Message: "Password changed successfully"})
}

// UpdateSettings godoc
//
//	@Summary		Update privacy settings
//	@Description	Updates the profile visibility and the per-field
//	@Description	hide flags. Any field left out of the request is
//	@Description	left unchanged.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		UpdateSettingsRequestWire	true	"settings fields"
//	@Success		200		{object}	UpdateSettingsResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid payload"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		404		{object}	api.ErrorResponse	"user not found"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/auth/settings [put]
func (h *Handler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequestWire
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	userID := middleware.CurrentUserID(c)

	user, err := h.svc.UpdateSettings(c.Request.Context(), userID, UpdateSettingsRequest(req))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
			return
		}
		if errors.Is(err, ErrSaveSettings) {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, UpdateSettingsResponse{
		Message: "Settings updated successfully",
		User:    user,
	})
}

// UpdateEmail godoc
//
//	@Summary		Change the authenticated user's email
//	@Description	Sends a verification link to the new address when
//	@Description	SMTP is enabled, or applies the change directly
//	@Description	otherwise. The 200 response indicates the request
//	@Description	was accepted; the body distinguishes the two paths
//	@Description	via the `pending` boolean.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		UpdateEmailRequestWire		true	"new email"
//	@Success		200		{object}	UpdateEmailPendingResponse	"pending verification"
//	@Success		200		{object}	UpdateEmailCompleteResponse	"applied directly"
//	@Failure		400		{object}	api.ErrorResponse			"invalid payload / same email / email in use"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		404		{object}	api.ErrorResponse	"user not found"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/auth/email [put]
func (h *Handler) UpdateEmail(c *gin.Context) {
	var req UpdateEmailRequestWire
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
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
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrSameEmail), errors.Is(err, ErrEmailInUse):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrMailConfigCheck), errors.Is(err, ErrGenerateToken), errors.Is(err, ErrSendEmail):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update email"})
		}
		return
	}

	if pending {
		c.JSON(http.StatusOK, UpdateEmailPendingResponse{
			Message: "Verification email sent. Please check your inbox and click the link to complete email change.",
			Pending: true,
		})
		return
	}

	resp := UpdateEmailCompleteResponse{
		Message: "Email updated successfully",
		Pending: false,
	}
	resp.User.Email = req.Email
	c.JSON(http.StatusOK, resp)
}

// RequestPasswordReset godoc
//
//	@Summary		Request a password reset email
//	@Description	The response is intentionally uniform for every
//	@Description	outcome: an unknown address, an SMTP failure, and a
//	@Description	successful send all return the same body. This
//	@Description	prevents callers from probing whether an email is
//	@Description	registered.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RequestPasswordResetRequestWire	true	"email"
//	@Success		200		{object}	GenericMessageResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid payload"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/auth/password/reset/request [post]
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req RequestPasswordResetRequestWire
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	protocol, host := h.emailLinkOrigin(c)
	err := h.svc.RequestPasswordReset(c.Request.Context(), req.Email, protocol, host)
	if err != nil {
		switch {
		case errors.Is(err, ErrGenerateResetToken):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrSendResetEmail):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to request password reset"})
		}
		return
	}

	c.JSON(http.StatusOK, GenericMessageResponse{Message: "If the email exists, reset link has been sent"})
}

// ResendVerification godoc
//
//	@Summary		Resend the verification email
//	@Description	The response is intentionally uniform: unknown
//	@Description	email, verified account, SMTP failure, and
//	@Description	database fault all return the same body. This
//	@Description	prevents callers from probing whether an address
//	@Description	exists and is unverified.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ResendVerificationRequestWire	true	"email"
//	@Success		200		{object}	GenericMessageResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid payload"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/auth/email/verify/resend [post]
func (h *Handler) ResendVerification(c *gin.Context) {
	var req ResendVerificationRequestWire
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	protocol, host := h.emailLinkOrigin(c)
	// Intentional discard (uniform anti-enumeration response above).
	_ = h.svc.ResendVerification(c.Request.Context(), ResendVerificationRequest{
		Email: req.Email, Protocol: protocol, Host: host,
	})

	c.JSON(http.StatusOK, GenericMessageResponse{
		Message: "If the account exists and is not verified, a verification email has been sent.",
	})
}

// ResetPassword godoc
//
//	@Summary		Reset password with an emailed token
//	@Description	Consumes the single-use token from
//	@Description	/api/auth/password/reset/request and sets a new
//	@Description	password for the matching account.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ResetPasswordRequestWire	true	"reset token and new password"
//	@Success		200		{object}	GenericMessageResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid / expired token or invalid payload"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/auth/password/reset [post]
func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequestWire
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidResetToken):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrQueryFailed):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrResetTokenExpired):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrEncryptPassword), errors.Is(err, ErrUpdatePassword):
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to reset password"})
		}
		return
	}

	c.JSON(http.StatusOK, GenericMessageResponse{Message: "Password reset successfully"})
}

// VerifyEmail godoc
//
//	@Summary		Verify the email address (or a pending email change)
//	@Description	Consumes the single-use token from the verification
//	@Description	email. The same endpoint handles both new-account
//	@Description	verification and pending email changes; the
//	@Description	`require_relogin` and `new_email` response fields
//	@Description	appear only on the email-change path.
//	@Tags			auth
//	@Produce		json
//	@Param			token	query		string	true	"single-use verification token"
//	@Success		200		{object}	VerifyEmailResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid / expired token"
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/auth/email/verify [get]
func (h *Handler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	slog.Debug("email verification request received", "hasToken", token != "")

	if token == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Verification token cannot be empty"})
		return
	}

	emailChange, newEmail, err := h.svc.VerifyEmail(c.Request.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidVerificationToken):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrVerificationTokenExpired):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrEmailChangeNoPending), errors.Is(err, ErrEmailChangeEmailInUse):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrQueryFailed),
			errors.Is(err, ErrUpdateUserVerification),
			errors.Is(err, ErrUpdateEmailChange):
			// Internal failures: log is already emitted by the service; do not
			// leak the underlying error (e.g. DB dial string) to the client.
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to verify email"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to verify email"})
		}
		return
	}

	if emailChange {
		resp := VerifyEmailResponse{
			Message:        "Email change successful! Your new email is now active.",
			RequireRelogin: true,
		}
		if newEmail != "" {
			resp.NewEmail = newEmail
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	c.JSON(http.StatusOK, VerifyEmailResponse{
		Message: "Email verification successful! You can now log in.",
	})
}

// GetVerificationStatus godoc
//
//	@Summary		Email verification status
//	@Description	Returns the authenticated user's verification
//	@Description	flag and current email. Used by the frontend to
//	@Description	decide whether to show the "verify your email"
//	@Description	banner.
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	VerificationStatusResponse
//	@Failure		401	{object}	api.ErrorResponse	"not logged in"
//	@Failure		404	{object}	api.ErrorResponse	"user not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/auth/email/verify/status [get]
func (h *Handler) GetVerificationStatus(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "Not logged in"})
		return
	}

	emailVerified, email, err := h.svc.VerificationStatus(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to get user information"})
		return
	}

	c.JSON(http.StatusOK, VerificationStatusResponse{
		EmailVerified: emailVerified,
		Email:         email,
	})
}
