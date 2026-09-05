// Package auth exposes authentication, password reset, email
// verification, and profile management endpoints.
//
// The handler is built around huma; the service layer
// (service.go, jwt.go, repository.go) is unchanged. Existing gin
// middleware (rate limit, JWTAuth) is applied at the huma
// sub-group level via humagin so the original request flow
// stays intact.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
)

// Handler exposes the auth domain over HTTP.
type Handler struct {
	svc                 *Service
	jwtAuth             gin.HandlerFunc
	linkScheme          string
	linkHost            string
	honorForwardedProto bool

	// rateLimit configures the per-IP rate limiter applied to
	// the credential sub-group. When perMinute <= 0, no
	// limiter is attached.
	rateLimitPerMinute int
	rateLimit          middleware.RateLimitStore
}

// NewHandler creates an auth HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	scheme, host := parseLinkOrigin(deps.BaseURL)
	return &Handler{
		svc:                 NewService(deps),
		jwtAuth:             middleware.NewAuth(deps.DB, deps.JWTSecret).JWTAuth(),
		linkScheme:          scheme,
		linkHost:            host,
		honorForwardedProto: deps.BehindReverseProxy,
		rateLimitPerMinute:  deps.RateLimitPerMinute,
		rateLimit:           deps.RateLimit,
	}
}

// parseLinkOrigin extracts scheme://host from a configured
// public origin. Anything unusable (empty, unparseable, wrong
// scheme) logs a warning and yields empty strings so callers
// can detect "not configured".
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

// authHumaConfig returns a huma config used by the auth sub-APIs.
// The auth domain mounts several sub-APIs on the same gin engine
// (rate-limited, authenticated, etc.); the OpenAPI/docs auto-routes
// are disabled so they don't collide when multiple sub-APIs share
// the same engine.
func authHumaConfig() huma.Config {
	c := huma.DefaultConfig("VexGo API", "0.1.0")
	c.OpenAPIPath = ""
	c.DocsPath = ""
	c.SchemasPath = ""
	return c
}

// ---------- input / output types ----------

type loginInput struct {
	Body api.LoginRequest
}

type loginOutput struct {
	Body api.LoginResponse
}

type registerInput struct {
	Body api.RegisterRequest
}

type registerOutput struct {
	Status int                     `status:"201" required:""`
	Body   api.RegisterResponse
}

type currentUserOutput struct {
	Body api.UserResponse
}

type updateProfileInput struct {
	Body api.UpdateProfileRequest
}

type changePasswordInput struct {
	Body api.ChangePasswordRequest
}

type changePasswordOutput struct {
	Body api.MessageResponse
}

type updateEmailInput struct {
	Body api.UpdateEmailRequest
}

type updateEmailOutput struct {
	Body api.UpdateEmailResponse
}

type updateSettingsInput struct {
	Body api.UpdateSettingsRequest
}

type verifyEmailInput struct {
	Token string `query:"token" required:"" doc:"Verification token from the email link"`
}

type verifyEmailOutput struct {
	Body api.VerifyEmailResponse
}

type verificationStatusOutput struct {
	Body api.VerificationStatusResponse
}

type requestPasswordResetInput struct {
	Body api.RequestPasswordResetRequest
}

type resendVerificationInput struct {
	Body api.ResendVerificationRequest
}

type resetPasswordInput struct {
	Body api.ResetPasswordRequest
}

// RegisterRoutes registers the auth domain operations.
func (h *Handler) RegisterRoutes(r *gin.Engine, parentAPI huma.API, parentGroup *gin.RouterGroup) {
	// The auth handler reads the request origin (X-Forwarded-Proto,
	// Host) when the configured BaseURL is empty. The gin
	// context must therefore be threaded into the huma
	// request context for email-link derivation.
	authAPIConfig := authHumaConfig()
	// Authenticated + rate-limited credential sub-group. The
	// rate limiter is a gin middleware and stays put.
	authGroup := parentGroup.Group("/auth")
	if limited := middleware.NewRateLimiter("auth", h.rateLimitPerMinute, h.rateLimit); limited != nil {
		authGroup.Use(limited)
	}
	authAPI := humagin.NewWithGroup(r, authGroup, authAPIConfig)
	authAPI.UseMiddleware(ContextMiddleware, GinContextMiddleware)

	huma.Register(authAPI, huma.Operation{
		OperationID: "register",
		Method:      http.MethodPost,
		Path:        "/register",
		Summary:     "Register a new user",
		Tags:        []string{"auth"},
	}, h.register)
	huma.Register(authAPI, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/login",
		Summary:     "Log in",
		Tags:        []string{"auth"},
	}, h.login)
	huma.Register(authAPI, huma.Operation{
		OperationID: "request-password-reset",
		Method:      http.MethodPost,
		Path:        "/request-password-reset",
		Summary:     "Request a password reset email",
		Tags:        []string{"auth"},
	}, h.requestPasswordReset)
	huma.Register(authAPI, huma.Operation{
		OperationID: "resend-verification",
		Method:      http.MethodPost,
		Path:        "/resend-verification",
		Summary:     "Resend the verification email",
		Tags:        []string{"auth"},
	}, h.resendVerification)
	huma.Register(authAPI, huma.Operation{
		OperationID: "reset-password",
		Method:      http.MethodPost,
		Path:        "/reset-password",
		Summary:     "Reset the password with a token",
		Tags:        []string{"auth"},
	}, h.resetPassword)

	// Authenticated sub-group.
	authedGroup := parentGroup.Group("/auth", h.jwtAuth)
	authedAPI := humagin.NewWithGroup(r, authedGroup, authHumaConfig())
	authedAPI.UseMiddleware(ContextMiddleware, GinContextMiddleware)

	huma.Register(authedAPI, huma.Operation{
		OperationID: "get-current-user",
		Method:      http.MethodGet,
		Path:        "/me",
		Summary:     "Get the current user",
		Tags:        []string{"auth"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.getCurrentUser)
	huma.Register(authedAPI, huma.Operation{
		OperationID: "get-current-user-alias",
		Method:      http.MethodGet,
		Path:        "/user",
		Summary:     "Get the current user (alias of /auth/me)",
		Tags:        []string{"auth"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.getCurrentUser)
	huma.Register(authedAPI, huma.Operation{
		OperationID: "update-profile",
		Method:      http.MethodPut,
		Path:        "/profile",
		Summary:     "Update the current user's profile",
		Tags:        []string{"auth"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.updateProfile)
	huma.Register(authedAPI, huma.Operation{
		OperationID: "change-password",
		Method:      http.MethodPut,
		Path:        "/password",
		Summary:     "Change the current user's password",
		Tags:        []string{"auth"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.changePassword)
	huma.Register(authedAPI, huma.Operation{
		OperationID: "update-email",
		Method:      http.MethodPut,
		Path:        "/email",
		Summary:     "Update the current user's email",
		Tags:        []string{"auth"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.updateEmail)
	huma.Register(authedAPI, huma.Operation{
		OperationID: "update-settings",
		Method:      http.MethodPut,
		Path:        "/settings",
		Summary:     "Update the current user's privacy settings",
		Tags:        []string{"auth"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.updateSettings)
	huma.Register(authedAPI, huma.Operation{
		OperationID: "get-verification-status",
		Method:      http.MethodGet,
		Path:        "/verification-status",
		Summary:     "Get the current user's email verification status",
		Tags:        []string{"auth"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.getVerificationStatus)

	// VerifyEmail is unauthenticated but lives outside /auth.
	huma.Register(parentAPI, huma.Operation{
		OperationID: "verify-email",
		Method:      http.MethodGet,
		Path:        "/verify-email",
		Summary:     "Verify an email via token",
		Tags:        []string{"auth"},
	}, h.verifyEmail)
}

// ---------- handlers ----------

func (h *Handler) login(ctx context.Context, in *loginInput) (*loginOutput, error) {
	slog.Debug("user login attempt", "email", in.Body.Email)
	token, user, err := h.svc.Login(ctx, LoginRequest{
		Email: in.Body.Email, Password: in.Body.Password,
		CaptchaID: in.Body.CaptchaID, CaptchaToken: in.Body.CaptchaToken,
		CaptchaX: in.Body.CaptchaX, CaptchaY: in.Body.CaptchaY,
	})
	if err != nil {
		return nil, mapAuthError(err, "Failed to login")
	}
	return &loginOutput{Body: api.LoginResponse{Token: token, User: *user}}, nil
}

func (h *Handler) register(ctx context.Context, in *registerInput) (*registerOutput, error) {
	slog.Debug("user registration attempt", "email", in.Body.Email, "username", in.Body.Username)
	protocol, host := h.emailLinkOrigin(ctx)
	result, err := h.svc.Register(ctx, RegisterRequest{
		Email: in.Body.Email, Password: in.Body.Password, Username: in.Body.Username,
		CaptchaID: in.Body.CaptchaID, CaptchaToken: in.Body.CaptchaToken,
		CaptchaX: in.Body.CaptchaX, CaptchaY: in.Body.CaptchaY,
		Protocol: protocol, Host: host,
	})
	if err != nil {
		return nil, mapAuthError(err, "Failed to register")
	}
	return &registerOutput{
		Status: http.StatusCreated,
		Body: api.RegisterResponse{
			Message:              messageForRegistration(result.RequiresVerification),
			User:                 *result.User,
			EmailVerified:        result.User.EmailVerified,
			RequiresVerification: result.RequiresVerification,
		},
	}, nil
}

// emailLinkOrigin returns the protocol/host used to build absolute
// links inside emails (verification, password reset, email
// change).
//
// The configured site origin always wins: request-supplied values
// (Host, X-Forwarded-Proto) never leak into emails when
// BaseURL is set. Without configuration, the request origin is
// used as a degraded fallback: X-Forwarded-Proto is trusted
// only when behind_reverse_proxy is enabled — otherwise a
// client could poison emailed links with a forged header.
func (h *Handler) emailLinkOrigin(ctx context.Context) (protocol, host string) {
	if h.linkHost != "" {
		return h.linkScheme, h.linkHost
	}
	gc := GinContextFromContext(ctx)
	if gc == nil {
		return h.linkScheme, h.linkHost
	}
	c := gc.Request
	protocol = "http"
	if c.TLS != nil || (h.honorForwardedProto && gc.GetHeader("X-Forwarded-Proto") == "https") {
		protocol = "https"
	}
	return protocol, c.Host
}

func (h *Handler) getCurrentUser(ctx context.Context, _ *struct{}) (*currentUserOutput, error) {
	uid := UserIDFromContext(ctx)
	if uid == 0 {
		return nil, huma.NewError(401, "Not logged in")
	}
	user, err := h.svc.GetCurrentUser(ctx, uid)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, huma.NewError(404, err.Error())
		}
		return nil, huma.NewError(500, "Failed to get user")
	}
	return &currentUserOutput{Body: api.UserResponse{User: *user}}, nil
}

func (h *Handler) updateProfile(ctx context.Context, in *updateProfileInput) (*currentUserOutput, error) {
	uid := UserIDFromContext(ctx)
	user, err := h.svc.UpdateProfile(ctx, uid, UpdateProfileRequest{
		Username: in.Body.Username, Avatar: in.Body.Avatar,
		Birthday: in.Body.Birthday, Bio: in.Body.Bio,
	})
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, huma.NewError(404, err.Error())
		}
		return nil, huma.NewError(500, "Failed to update profile")
	}
	return &currentUserOutput{Body: api.UserResponse{User: *user}}, nil
}

func (h *Handler) changePassword(ctx context.Context, in *changePasswordInput) (*changePasswordOutput, error) {
	uid := UserIDFromContext(ctx)
	if err := h.svc.ChangePassword(ctx, uid, in.Body.OldPassword, in.Body.NewPassword); err != nil {
		return nil, mapChangePasswordError(err)
	}
	return &changePasswordOutput{Body: api.MessageResponse{Message: "Password changed successfully"}}, nil
}

func (h *Handler) updateEmail(ctx context.Context, in *updateEmailInput) (*updateEmailOutput, error) {
	uid := UserIDFromContext(ctx)
	pending, err := h.svc.UpdateEmail(ctx, UpdateEmailRequest{
		UserID: uid, NewEmail: in.Body.Email,
		Protocol: h.linkScheme, Host: h.linkHost,
	})
	if err != nil {
		return nil, mapUpdateEmailError(err)
	}
	if pending {
		return &updateEmailOutput{Body: api.UpdateEmailResponse{
			Message: "Verification email sent. Please check your inbox and click the link to complete email change.",
			Pending: true,
		}}, nil
	}
	return &updateEmailOutput{Body: api.UpdateEmailResponse{
		Message: "Email updated successfully",
		Pending: false,
		User:    &api.UserSummary{Email: in.Body.Email},
	}}, nil
}

func (h *Handler) updateSettings(ctx context.Context, in *updateSettingsInput) (*currentUserOutput, error) {
	uid := UserIDFromContext(ctx)
	user, err := h.svc.UpdateSettings(ctx, uid, UpdateSettingsRequest{
		ProfileVisibility: in.Body.ProfileVisibility,
		HideEmail:         in.Body.HideEmail,
		HideBirthday:      in.Body.HideBirthday,
		HideBio:           in.Body.HideBio,
	})
	if err != nil {
		return nil, mapUpdateSettingsError(err)
	}
	return &currentUserOutput{Body: api.UserResponse{
		Message: "Settings updated successfully", User: *user,
	}}, nil
}

func (h *Handler) getVerificationStatus(ctx context.Context, _ *struct{}) (*verificationStatusOutput, error) {
	uid := UserIDFromContext(ctx)
	user, err := h.svc.GetCurrentUser(ctx, uid)
	if err != nil {
		return nil, huma.NewError(500, "Failed to get verification status")
	}
	return &verificationStatusOutput{Body: api.VerificationStatusResponse{
		EmailVerified: user.EmailVerified, Email: user.Email,
	}}, nil
}

func (h *Handler) verifyEmail(ctx context.Context, in *verifyEmailInput) (*verifyEmailOutput, error) {
	emailChange, newEmail, err := h.svc.VerifyEmail(ctx, in.Token)
	if err != nil {
		return nil, mapVerifyEmailError(err)
	}
	out := verifyEmailOutput{Body: api.VerifyEmailResponse{Message: "Email verified"}}
	if emailChange {
		out.Body.RequireRelogin = true
		out.Body.NewEmail = newEmail
	}
	return &out, nil
}

func (h *Handler) requestPasswordReset(ctx context.Context, in *requestPasswordResetInput) (*changePasswordOutput, error) {
	if err := h.svc.RequestPasswordReset(ctx, in.Body.Email, h.linkScheme, h.linkHost); err != nil {
		return nil, huma.NewError(500, "Failed to request password reset")
	}
	return &changePasswordOutput{Body: api.MessageResponse{Message: "If the email exists, reset link has been sent"}}, nil
}

func (h *Handler) resendVerification(ctx context.Context, in *resendVerificationInput) (*changePasswordOutput, error) {
	_ = h.svc.ResendVerification(ctx, ResendVerificationRequest{
		Email: in.Body.Email, Protocol: h.linkScheme, Host: h.linkHost,
	})
	return &changePasswordOutput{Body: api.MessageResponse{Message: "If the account exists and is not verified, a verification email has been sent."}}, nil
}

func (h *Handler) resetPassword(ctx context.Context, in *resetPasswordInput) (*changePasswordOutput, error) {
	if err := h.svc.ResetPassword(ctx, in.Body.Token, in.Body.Password); err != nil {
		switch {
		case errors.Is(err, ErrInvalidResetToken), errors.Is(err, ErrResetTokenExpired):
			return nil, huma.NewError(400, err.Error())
		default:
			return nil, huma.NewError(500, "Failed to reset password")
		}
	}
	return &changePasswordOutput{Body: api.MessageResponse{Message: "Password reset successfully"}}, nil
}

// ---------- error mapping ----------

func mapAuthError(err error, defaultMsg string) error {
	switch {
	case errors.Is(err, ErrCaptchaCheckFailed), errors.Is(err, ErrCaptchaFailed), errors.Is(err, ErrTokenGeneration):
		return huma.NewError(500, err.Error())
	case errors.Is(err, ErrRegistrationDisabled), errors.Is(err, ErrEmailUnverified):
		return huma.NewError(403, err.Error())
	case errors.Is(err, ErrCaptchaRequired), errors.Is(err, ErrCaptchaExpired), errors.Is(err, ErrCaptchaMismatch):
		return huma.NewError(400, err.Error())
	case errors.Is(err, ErrCaptchaNotFound):
		return huma.NewError(404, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		return huma.NewError(401, err.Error())
	case errors.Is(err, ErrUserExists):
		return huma.NewError(409, err.Error())
	case errors.Is(err, ErrHashPassword), errors.Is(err, ErrCreateUser),
		errors.Is(err, ErrMailConfigCheck), errors.Is(err, ErrSendEmail):
		return huma.NewError(500, err.Error())
	default:
		return huma.NewError(500, defaultMsg)
	}
}

func mapChangePasswordError(err error) error {
	switch {
	case errors.Is(err, ErrUserNotFound), errors.Is(err, ErrWrongPassword):
		return huma.NewError(401, err.Error())
	case errors.Is(err, ErrEncryptPassword):
		return huma.NewError(500, err.Error())
	default:
		return huma.NewError(500, "Failed to change password")
	}
}

func mapUpdateEmailError(err error) error {
	switch {
	case errors.Is(err, ErrUserNotFound):
		return huma.NewError(404, err.Error())
	case errors.Is(err, ErrSameEmail), errors.Is(err, ErrEmailInUse):
		return huma.NewError(400, err.Error())
	case errors.Is(err, ErrMailConfigCheck), errors.Is(err, ErrGenerateToken), errors.Is(err, ErrSendEmail):
		return huma.NewError(500, err.Error())
	default:
		return huma.NewError(500, "Failed to update email")
	}
}

func mapUpdateSettingsError(err error) error {
	switch {
	case errors.Is(err, ErrUserNotFound):
		return huma.NewError(404, err.Error())
	default:
		return huma.NewError(500, "Failed to update settings")
	}
}

func mapVerifyEmailError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidVerificationToken), errors.Is(err, ErrVerificationTokenExpired),
		errors.Is(err, ErrEmailChangeNoPending), errors.Is(err, ErrEmailChangeEmailInUse):
		return huma.NewError(400, err.Error())
	default:
		return huma.NewError(500, "Failed to verify email")
	}
}

func messageForRegistration(requires bool) string {
	if requires {
		return "Registration successful! Please verify your email address before logging in."
	}
	return "Registration successful"
}
