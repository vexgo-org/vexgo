package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"github.com/sirupsen/logrus"
	"math"
	"net/http"
	"strings"
	"time"

	"vexgo/backend/model"
	"vexgo/backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// VerifyEmail verifies email (supports initial verification and email change)
func VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	logrus.Printf("[VerifyEmail] Received verification request, token: %s", token)

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token cannot be empty"})
		return
	}

	mailer := utils.NewMailer(db)

	// Determine if token is for email verification or email change based on prefix
	var err error
	if strings.HasPrefix(token, "email-change-") {
		logrus.Printf("[VerifyEmail] Detected email change token, calling ConfirmEmailChange")
		// Email change token
		err = mailer.ConfirmEmailChange(token)
		if err != nil {
			logrus.Printf("[VerifyEmail] ConfirmEmailChange failed: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		logrus.Printf("[VerifyEmail] ConfirmEmailChange succeeded")
		// Query user information after change
		var user model.User
		if err := db.Where("verification_token = ?", token).First(&user).Error; err == nil {
			c.JSON(http.StatusOK, gin.H{
				"message":         "Email change successful! Your new email is now active.",
				"require_relogin": true,
				"new_email":       user.Email,
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"message":         "Email change successful! Your new email is now active.",
				"require_relogin": true,
			})
		}
	} else {
		logrus.Printf("[VerifyEmail] Normal email verification token, calling VerifyEmail")
		// Normal email verification token
		err = mailer.VerifyEmail(token)
		if err != nil {
			logrus.Printf("[VerifyEmail] VerifyEmail failed: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		logrus.Printf("[VerifyEmail] VerifyEmail succeeded")
		c.JSON(http.StatusOK, gin.H{
			"message": "Email verification successful! You can now log in.",
		})
	}
}

// GetVerificationStatus gets current user's email verification status
func GetVerificationStatus(c *gin.Context) {
	userContext, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if userMap, ok := userContext.(map[string]interface{}); ok {
		if userID, ok := userMap["id"].(float64); ok {
			var user model.User
			if err := db.First(&user, uint(userID)).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"email_verified": user.EmailVerified,
				"email":          user.Email,
			})
			return
		}
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
}

// GenerateCaptcha generates sliding puzzle captcha
func GenerateCaptcha(c *gin.Context) {
	// Generate captcha ID and token
	captchaID := uuid.New().String()
	token := uuid.New().String()

	// Set puzzle size
	puzzleWidth := 60
	puzzleHeight := 60
	bgWidth := 320
	bgHeight := 160

	// Randomly generate puzzle position (ensure puzzle is fully inside image)
	// Left margin:right margin = 3:2, puzzle biased to the right
	totalWidth := bgWidth - puzzleWidth
	targetLeft := totalWidth * 3 / 5 // Target left margin (60% of total width)
	// Random fluctuation near target value (±20%)
	randomRange := totalWidth / 5
	minX := targetLeft - randomRange
	maxX := targetLeft + randomRange
	// Ensure minimum margin of at least 20 pixels
	if minX < 20 {
		minX = 20
	}
	if maxX > bgWidth-puzzleWidth-20 {
		maxX = bgWidth - puzzleWidth - 20
	}
	x := minX + randInt(maxX-minX)
	y := 20 + randInt(bgHeight-puzzleHeight-40) // Y position between 20-80

	// Create background image (blue gradient)
	bgImage := createGradientBackground(bgWidth, bgHeight)

	// Create puzzle shape
	puzzleShape := createPuzzleShape(puzzleWidth, puzzleHeight)

	// Extract puzzle part from background image
	puzzleImage := extractPuzzleImage(bgImage, x, y, puzzleShape, puzzleWidth, puzzleHeight)

	// Draw puzzle outline on background image
	bgImageWithHole := drawPuzzleHole(bgImage, x, y, puzzleShape, puzzleWidth, puzzleHeight)

	// Convert image to Base64
	bgImageBase64, err := imageToBase64(bgImageWithHole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode background image"})
		return
	}

	puzzleImageBase64, err := imageToBase64(puzzleImage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode puzzle image"})
		return
	}

	// Save captcha information to database
	captcha := model.Captcha{
		ID:        captchaID,
		Token:     token,
		X:         x,
		Y:         y,
		Width:     puzzleWidth,
		Height:    puzzleHeight,
		BgImage:   bgImageBase64,
		PuzzleImg: puzzleImageBase64,
		ExpiresAt: time.Now().Add(5 * time.Minute), // 5 minutes expiration
		Used:      false,
	}

	if err := db.Create(&captcha).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save captcha"})
		return
	}

	// Return captcha information (without correct answer)
	c.JSON(http.StatusOK, gin.H{
		"id":         captchaID,
		"token":      token,
		"bg_image":   bgImageBase64,
		"puzzle_img": puzzleImageBase64,
		"y":          y, // Return puzzle y coordinate
		"expires_at": captcha.ExpiresAt,
	})
}

// VerifyCaptcha verifies sliding puzzle and marks as used (pre-verification)
func VerifyCaptcha(c *gin.Context) {
	var req struct {
		ID    string `json:"id" binding:"required"`
		Token string `json:"token" binding:"required"`
		X     int    `json:"x" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Query captcha
	var captcha model.Captcha
	if err := db.Where("id = ? AND token = ?", req.ID, req.Token).First(&captcha).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Captcha does not exist or has expired"})
		return
	}

	// Check if already used
	if captcha.Used {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Captcha already used"})
		return
	}

	// Check if expired
	if time.Now().After(captcha.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Captcha has expired"})
		return
	}

	// Verify position (allow certain tolerance)
	tolerance := 10 // allow 10 pixel tolerance
	if math.Abs(float64(req.X-captcha.X)) > float64(tolerance) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification failed, please try again"})
		return
	}

	// Mark as used
	captcha.Used = true
	if err := db.Save(&captcha).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Captcha verification failed"})
		return
	}

	// Return verification success
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Verification successful"})
}

// Helper functions

// createGradientBackground creates a simple gradient background
func createGradientBackground(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a simple blue gradient
			r := uint8(100 + x*155/width)
			g := uint8(150 + y*105/height)
			b := uint8(200)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	// Add some simple decorations
	for i := 0; i < 5; i++ {
		x := i * width / 5
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 100})
		}
	}

	return img
}

// createPuzzleShape creates puzzle shape - symmetric cross
func createPuzzleShape(width, height int) [][]bool {
	// Create a puzzle shape
	shape := make([][]bool, height)

	// Calculate center and arm length of cross
	centerX := width / 2
	centerY := height / 2
	// Arm length takes half of the smaller width/height, ensure cross is symmetric in square area
	armLength := min(width, height) / 3

	// Calculate boundaries
	left := centerX - armLength
	right := centerX + armLength
	top := centerY - armLength
	bottom := centerY + armLength

	// Arm thickness (half of center square)
	armThickness := armLength / 2

	for y := 0; y < height; y++ {
		shape[y] = make([]bool, width)
		for x := 0; x < width; x++ {
			// Center square area
			if x >= left && x <= right && y >= top && y <= bottom {
				shape[y][x] = true
				continue
			}

			// Vertical arm (up-down extension) - within center vertical range but outside center square
			if x >= centerX-armThickness && x <= centerX+armThickness {
				if y < top || y > bottom {
					shape[y][x] = true
					continue
				}
			}

			// Horizontal arm (left-right extension) - within center horizontal range but outside center square
			if y >= centerY-armThickness && y <= centerY+armThickness {
				if x < left || x > right {
					shape[y][x] = true
					continue
				}
			}
		}
	}

	return shape
}

// extractPuzzleImage extracts puzzle part from background image
func extractPuzzleImage(bgImage *image.RGBA, x, y int, shape [][]bool, width, height int) *image.RGBA {
	puzzleImg := image.NewRGBA(image.Rect(0, 0, width, height))

	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			if py < len(shape) && px < len(shape[py]) && shape[py][px] {
				bgX := x + px
				bgY := y + py

				// Check boundaries
				if bgX >= 0 && bgX < bgImage.Bounds().Dx() && bgY >= 0 && bgY < bgImage.Bounds().Dy() {
					puzzleImg.Set(px, py, bgImage.At(bgX, bgY))
				}
			} else {
				// Transparent background
				puzzleImg.Set(px, py, color.Transparent)
			}
		}
	}

	return puzzleImg
}

// drawPuzzleHole draws puzzle outline on background image
func drawPuzzleHole(bgImage *image.RGBA, x, y int, shape [][]bool, width, height int) *image.RGBA {
	// Create copy of background image
	bgCopy := image.NewRGBA(bgImage.Bounds())
	draw.Draw(bgCopy, bgCopy.Bounds(), bgImage, image.Point{}, draw.Src)

	// Draw semi-transparent shadow at puzzle position
	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			if py < len(shape) && px < len(shape[py]) && shape[py][px] {
				bgX := x + px
				bgY := y + py

				// Check boundaries
				if bgX >= 0 && bgX < bgCopy.Bounds().Dx() && bgY >= 0 && bgY < bgCopy.Bounds().Dy() {
					// Get original pixel and darken it
					original := bgCopy.At(bgX, bgY)
					r, g, b, a := original.RGBA()
					// Darken by 20%
					r = uint32(float64(r) * 0.8)
					g = uint32(float64(g) * 0.8)
					b = uint32(float64(b) * 0.8)
					bgCopy.Set(bgX, bgY, color.NRGBA{uint8(r / 256), uint8(g / 256), uint8(b / 256), uint8(a / 256)})
				}
			}
		}
	}

	return bgCopy
}

// imageToBase64 converts image to Base64 string
func imageToBase64(img *image.RGBA) (string, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return "", err
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// randInt generates random integer
func randInt(max int) int {
	if max <= 0 {
		return 0
	}

	b := make([]byte, 4)
	_, err := rand.Read(b)
	if err != nil {
		return 0
	}

	return int(b[0]) % max
}

// ResendVerificationEmail resends verification email
func ResendVerificationEmail(c *gin.Context) {
	userContext, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if userMap, ok := userContext.(map[string]interface{}); ok {
		if userID, ok := userMap["id"].(float64); ok {
			var user model.User
			if err := db.First(&user, uint(userID)).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
				return
			}

			if user.EmailVerified {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Email already verified"})
				return
			}

			mailer := utils.NewMailer(db)
			enabled, err := mailer.IsEmailEnabled()
			if err != nil || !enabled {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Email service not enabled"})
				return
			}

			// Generate new verification token
			token, err := mailer.GenerateVerificationToken(user.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
				return
			}

			// Build verification link
			verificationLink := c.Request.Host + "/verify-email?token=" + token
			if err := mailer.SendVerificationEmail(user.Email, user.Username, verificationLink); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Verification email has been resent, please check your inbox",
			})
			return
		}
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
}
