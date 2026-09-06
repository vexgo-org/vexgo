package captcha

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
)

// Handler exposes the captcha domain over HTTP.
type Handler struct {
	svc                *Service
	rateLimitPerMinute int
	rateLimit          middleware.RateLimitStore
}

// NewHandler creates a captcha HTTP handler with the given dependencies. A
// positive deps.RateLimitPerMinute installs a per-client-IP rate limit on the
// unauthenticated captcha endpoints.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), rateLimitPerMinute: deps.RateLimitPerMinute, rateLimit: deps.RateLimit}
}

// GenerateCaptcha godoc
//
//	@Summary		Generate a sliding-puzzle captcha
//	@Description	Returns a captcha id, token, and base64-encoded background /
//	@Description	puzzle images. The client solves the puzzle and submits the
//	@Description	coordinates to /captcha/verify. The captcha is single-use
//	@Description	and expires shortly after creation. This endpoint is rate-limited
//	@Description	per client IP.
//	@Tags			captcha
//	@Produce		json
//	@Success		200	{object}	CaptchaGenerateResponse
//	@Failure		429	{object}	api.ErrorResponse	"rate limit exceeded"
//	@Failure		500	{object}	api.ErrorResponse	"failed to generate captcha"
//	@Router			/captcha [get]
func (h *Handler) GenerateCaptcha(c *gin.Context) {
	captcha, err := h.svc.GenerateCaptcha(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, ErrEncodeBgImage):
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode background image"})
		case errors.Is(err, ErrEncodePuzzleImage):
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode puzzle image"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate captcha"})
		}
		return
	}

	// Return captcha information (without correct answer); the shape maps
	// directly onto the go-captcha-react Slide component's data prop.
	c.JSON(http.StatusOK, CaptchaGenerateResponse{
		ID:          captcha.ID,
		Token:       captcha.Token,
		ThumbX:      captcha.ThumbX,
		ThumbY:      captcha.ThumbY,
		ThumbWidth:  captcha.ThumbWidth,
		ThumbHeight: captcha.ThumbHeight,
		Image:       captcha.Image,
		Thumb:       captcha.Thumb,
		ExpiresAt:   captcha.ExpiresAt.Format(time.RFC3339),
	})
}

// VerifyCaptcha godoc
//
//	@Summary		Verify a captcha solution
//	@Description	Consumes a captcha id and the puzzle coordinates. On success
//	@Description	marks the captcha as used so it cannot be re-submitted. The
//	@Description	verification result is held in the JWT and must be presented
//	@Description	alongside credential endpoints (login, register, etc.) that
//	@Description	require captcha verification.
//	@Tags			captcha
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CaptchaVerifyRequest	true	"captcha id, token, and coordinates"
//	@Success		200		{object}	CaptchaVerifyResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid request, mismatch, expired, or already used"
//	@Failure		404		{object}	api.ErrorResponse	"captcha does not exist"
//	@Failure		500		{object}	api.ErrorResponse	"internal error"
//	@Router			/captcha/verify [post]
func (h *Handler) VerifyCaptcha(c *gin.Context) {
	var req CaptchaVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	err := h.svc.VerifyCaptcha(c.Request.Context(), VerifyArgs{
		ID:    req.ID,
		Token: req.Token,
		X:     req.X,
		Y:     req.Y,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrCaptchaNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Captcha does not exist or has expired"})
		case errors.Is(err, ErrCaptchaUsed):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Captcha already used"})
		case errors.Is(err, ErrCaptchaExpired):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Captcha has expired"})
		case errors.Is(err, ErrCaptchaMismatch):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Verification failed, please try again"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Captcha verification failed"})
		}
		return
	}

	// Return verification success
	c.JSON(http.StatusOK, CaptchaVerifyResponse{Success: true, Message: "Verification successful"})
}
