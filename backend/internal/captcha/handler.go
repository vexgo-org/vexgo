package captcha

import (
	"errors"
	"net/http"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Handler exposes the verification domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates a verification HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// GetVerificationStatus gets current user's email verification status
func (h *Handler) GetVerificationStatus(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	emailVerified, email, err := h.svc.VerificationStatus(c.Request.Context(), u.ID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save captcha"})
		}
		return
	}

	// Return captcha information (without correct answer)
	c.JSON(http.StatusOK, gin.H{
		"id":         captcha.ID,
		"token":      captcha.Token,
		"bg_image":   captcha.BgImage,
		"puzzle_img": captcha.PuzzleImg,
		"y":          captcha.Y, // Return puzzle y coordinate
		"expires_at": captcha.ExpiresAt,
	})
}

// VerifyCaptcha verifies sliding puzzle and marks as used (pre-verification)
func (h *Handler) VerifyCaptcha(c *gin.Context) {
	var req struct {
		ID    string `json:"id" binding:"required"`
		Token string `json:"token" binding:"required"`
		X     int    `json:"x" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.VerifyCaptcha(c.Request.Context(), req.ID, req.Token, req.X)
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
