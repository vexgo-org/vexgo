// Package verification implements email verification and sliding-puzzle
// captcha generation/verification.
package verification

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"
	"time"

	"vexgo/backend/internal/model"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler.
var (
	// ErrUserNotFound means the user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrEmailAlreadyVerified means the email is already verified.
	ErrEmailAlreadyVerified = errors.New("email already verified")
	// ErrEmailServiceDisabled means SMTP is not enabled.
	ErrEmailServiceDisabled = errors.New("email service not enabled")
	// ErrCaptchaNotFound means the captcha does not exist or has expired.
	ErrCaptchaNotFound = errors.New("captcha not found")
	// ErrCaptchaUsed means the captcha was already used.
	ErrCaptchaUsed = errors.New("captcha already used")
	// ErrCaptchaExpired means the captcha has expired.
	ErrCaptchaExpired = errors.New("captcha has expired")
	// ErrCaptchaMismatch means the submitted puzzle position is wrong.
	ErrCaptchaMismatch = errors.New("captcha mismatch")
	// ErrEncodeBgImage means the background image could not be encoded.
	ErrEncodeBgImage = errors.New("encode background image")
	// ErrEncodePuzzleImage means the puzzle image could not be encoded.
	ErrEncodePuzzleImage = errors.New("encode puzzle image")
	// ErrGenerateToken means the verification token could not be generated.
	ErrGenerateToken = errors.New("generate verification token")
	// ErrSendVerificationEmail means the verification email could not be sent.
	ErrSendVerificationEmail = errors.New("send verification email")
)

// Deps holds the dependencies required by the verification domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Mailer    model.Mailer
}

// Service contains the business logic of the verification domain.
type Service struct {
	repo   Repository
	mailer model.Mailer
}

// NewService creates a verification service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), mailer: deps.Mailer}
}

// VerifyEmail verifies an email address. Tokens prefixed with "email-change-"
// confirm a pending email change; all other tokens verify the initial email.
// It returns whether the token was an email change and the user's new email
// (only meaningful for email changes).
func (s *Service) VerifyEmail(ctx context.Context, token string) (emailChange bool, newEmail string, err error) {
	if strings.HasPrefix(token, model.TokenPrefixEmailChange) {
		logrus.Debug("[VerifyEmail] Detected email change token, calling ConfirmEmailChange")
		// The pending email must be read before the token is consumed:
		// ConfirmEmailChange clears verification_token together with
		// pending_email, so a lookup afterwards can never find the user.
		user, err := s.repo.FindUserByToken(ctx, token)
		if err != nil {
			logrus.WithError(err).Debug("[VerifyEmail] FindUserByToken failed")
			return false, "", err
		}
		pendingEmail := user.PendingEmail

		if err := s.mailer.ConfirmEmailChange(token); err != nil {
			logrus.WithError(err).Debug("[VerifyEmail] ConfirmEmailChange failed")
			return false, "", err
		}
		logrus.Debug("[VerifyEmail] ConfirmEmailChange succeeded")
		return true, pendingEmail, nil
	}

	logrus.Debug("[VerifyEmail] Normal email verification token, calling VerifyEmail")
	if err := s.mailer.VerifyEmail(token); err != nil {
		logrus.WithError(err).Debug("[VerifyEmail] VerifyEmail failed")
		return false, "", err
	}
	logrus.Debug("[VerifyEmail] VerifyEmail succeeded")
	return false, "", nil
}

// VerificationStatus returns the email verification status of a user.
func (s *Service) VerificationStatus(ctx context.Context, userID uint) (emailVerified bool, email string, err error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, "", ErrUserNotFound
		}
		return false, "", err
	}
	return user.EmailVerified, user.Email, nil
}

// ResendVerificationEmail generates a new verification token for the user and
// sends it to their email address.
func (s *Service) ResendVerificationEmail(ctx context.Context, userID uint, host string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrUserNotFound
		}
		return err
	}

	if user.EmailVerified {
		return ErrEmailAlreadyVerified
	}

	enabled, err := s.mailer.IsEmailEnabled()
	if err != nil || !enabled {
		return ErrEmailServiceDisabled
	}

	// Generate new verification token
	token, err := s.mailer.GenerateVerificationToken(user.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGenerateToken, err)
	}

	// Build verification link
	verificationLink := host + "/verify-email?token=" + token
	if err := s.mailer.SendVerificationEmail(user.Email, user.Username, verificationLink); err != nil {
		return fmt.Errorf("%w: %v", ErrSendVerificationEmail, err)
	}

	return nil
}

// IsCaptchaEnabled reports whether captcha verification is enabled in the
// general settings (disabled by default when no settings row exists).
func (s *Service) IsCaptchaEnabled(ctx context.Context) (bool, error) {
	settings, err := s.repo.GetGeneralSettings(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Not enabled by default
			return false, nil
		}
		return false, err
	}
	return settings.CaptchaEnabled, nil
}

// Captcha is the result of generating a sliding puzzle captcha. The correct X
// coordinate is intentionally not exposed to the client.
type Captcha struct {
	ID        string
	Token     string
	X         int
	Y         int
	BgImage   string
	PuzzleImg string
	ExpiresAt time.Time
}

// GenerateCaptcha creates a sliding puzzle captcha, persists it, and returns
// the client-facing information.
func (s *Service) GenerateCaptcha(ctx context.Context) (*Captcha, error) {
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
		return nil, fmt.Errorf("%w: %v", ErrEncodeBgImage, err)
	}

	puzzleImageBase64, err := imageToBase64(puzzleImage)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodePuzzleImage, err)
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

	if err := s.repo.CreateCaptcha(ctx, &captcha); err != nil {
		return nil, err
	}

	return &Captcha{
		ID:        captchaID,
		Token:     token,
		X:         x,
		Y:         y,
		BgImage:   bgImageBase64,
		PuzzleImg: puzzleImageBase64,
		ExpiresAt: captcha.ExpiresAt,
	}, nil
}

// VerifyCaptcha verifies a sliding puzzle submission and marks the captcha as
// used.
func (s *Service) VerifyCaptcha(ctx context.Context, id, token string, x int) error {
	// Query captcha
	captcha, err := s.repo.FindCaptcha(ctx, id, token)
	if err != nil {
		return ErrCaptchaNotFound
	}

	// Check if already used
	if captcha.Used {
		return ErrCaptchaUsed
	}

	// Check if expired
	if time.Now().After(captcha.ExpiresAt) {
		return ErrCaptchaExpired
	}

	// Verify position (allow certain tolerance)
	tolerance := 10 // allow 10 pixel tolerance
	if math.Abs(float64(x-captcha.X)) > float64(tolerance) {
		return ErrCaptchaMismatch
	}

	// Mark as used
	captcha.Used = true
	if err := s.repo.SaveCaptcha(ctx, captcha); err != nil {
		return err
	}

	return nil
}

// createGradientBackground creates a simple gradient background
func createGradientBackground(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := range height {
		for x := range width {
			// Create a simple blue gradient
			r := uint8(100 + x*155/width)
			g := uint8(150 + y*105/height)
			b := uint8(200)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	// Add some simple decorations
	for i := range 5 {
		x := i * width / 5
		for y := range height {
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

	for y := range height {
		shape[y] = make([]bool, width)
		for x := range width {
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

	for py := range height {
		for px := range width {
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
	for py := range height {
		for px := range width {
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
