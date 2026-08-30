package captcha

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler exposes the captcha domain over HTTP.
type Handler struct {
	svc                *Service
	rateLimitPerMinute int
}

// NewHandler creates a captcha HTTP handler with the given dependencies. A
// positive deps.RateLimitPerMinute installs a per-client-IP rate limit on the
// unauthenticated captcha endpoints.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), rateLimitPerMinute: deps.RateLimitPerMinute}
}

// GenerateCaptcha generates sliding puzzle captcha
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
	c.JSON(http.StatusOK, gin.H{
		"id":          captcha.ID,
		"token":       captcha.Token,
		"thumbX":      captcha.ThumbX,
		"thumbY":      captcha.ThumbY,
		"thumbWidth":  captcha.ThumbWidth,
		"thumbHeight": captcha.ThumbHeight,
		"image":       captcha.Image,
		"thumb":       captcha.Thumb,
		"expires_at":  captcha.ExpiresAt,
	})
}

// VerifyCaptcha verifies sliding puzzle and marks as used (pre-verification)
func (h *Handler) VerifyCaptcha(c *gin.Context) {
	var req struct {
		ID    string `json:"id" binding:"required"`
		Token string `json:"token" binding:"required"`
		X     int    `json:"x" binding:"required"`
		Y     int    `json:"y" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Captcha does not exist or has expired"})
		case errors.Is(err, ErrCaptchaUsed):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Captcha already used"})
		case errors.Is(err, ErrCaptchaExpired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Captcha has expired"})
		case errors.Is(err, ErrCaptchaMismatch):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Verification failed, please try again"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Captcha verification failed"})
		}
		return
	}

	// Return verification success
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Verification successful"})
}
