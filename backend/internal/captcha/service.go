// Package captcha implements sliding-puzzle captcha generation and
// verification.
package captcha

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/google/uuid"
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/slide"
	"gorm.io/gorm"
)

const (
	// captchaImageWidth and captchaImageHeight are the pixel dimensions of
	// the generated master image.
	captchaImageWidth  = 320
	captchaImageHeight = 160
	// verifyPadding is the allowed deviation in pixels per axis when
	// validating a submitted drop position.
	verifyPadding = 5
	// captchaTTL is how long a generated challenge stays valid.
	captchaTTL = 5 * time.Minute
)

// Sentinel errors mapped to HTTP responses by the handler.
var (
	// ErrCaptchaNotFound means the captcha does not exist or has expired.
	ErrCaptchaNotFound = errors.New("captcha not found")
	// ErrCaptchaUsed means the captcha was already used.
	ErrCaptchaUsed = errors.New("captcha already used")
	// ErrCaptchaExpired means the captcha has expired.
	ErrCaptchaExpired = errors.New("captcha has expired")
	// ErrCaptchaMismatch means the submitted puzzle position is wrong.
	ErrCaptchaMismatch = errors.New("captcha mismatch")
	// ErrCaptchaFailed means the captcha could not be verified due to an
	// internal error, as opposed to a wrong submission.
	ErrCaptchaFailed = errors.New("captcha verification failed")
	// ErrEncodeBgImage means the background image could not be encoded.
	ErrEncodeBgImage = errors.New("encode background image")
	// ErrEncodePuzzleImage means the puzzle image could not be encoded.
	ErrEncodePuzzleImage = errors.New("encode puzzle image")
)

// Deps holds the dependencies required by the captcha domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	// RateLimitPerMinute caps the requests per client IP per minute on the
	// captcha endpoints; <= 0 disables the limit.
	RateLimitPerMinute int
	// RateLimit stores the per-IP request budget. nil keeps it in-process; a
	// distributed store shares one budget across instances.
	RateLimit middleware.RateLimitStore
}

// Service contains the business logic of the captcha domain.
type Service struct {
	repo Repository
}

// NewService creates a captcha service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB)}
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

// Captcha is the result of generating a sliding puzzle captcha. The correct
// drop position is intentionally not exposed to the client.
type Captcha struct {
	ID          string
	Token       string
	ThumbX      int
	ThumbY      int
	ThumbWidth  int
	ThumbHeight int
	Image       string
	Thumb       string
	ExpiresAt   time.Time
}

// VerifyArgs carries the inputs for one captcha verification.
type VerifyArgs struct {
	ID    string
	Token string
	X     int
	Y     int
}

var (
	builderOnce  sync.Once
	slideBuilder slide.Builder
	builderErr   error
)

// getBuilder lazily builds the slide captcha generator with the embedded
// asset pack, so the background and tile images are decoded only once per
// process.
func getBuilder() (slide.Builder, error) {
	builderOnce.Do(func() {
		backgrounds, err := imagesv2.GetImages()
		if err != nil {
			builderErr = fmt.Errorf("load captcha backgrounds: %w", err)
			return
		}
		assetTiles, err := tiles.GetTiles()
		if err != nil {
			builderErr = fmt.Errorf("load captcha tiles: %w", err)
			return
		}
		graphImages := make([]*slide.GraphImage, 0, len(assetTiles))
		for _, assetTile := range assetTiles {
			graphImages = append(graphImages, &slide.GraphImage{
				OverlayImage: assetTile.OverlayImage,
				ShadowImage:  assetTile.ShadowImage,
				MaskImage:    assetTile.MaskImage,
			})
		}
		builder := slide.NewBuilder(
			slide.WithImageSize(option.Size{
				Width:  captchaImageWidth,
				Height: captchaImageHeight,
			}),
		)
		builder.SetResources(
			slide.WithBackgrounds(backgrounds),
			slide.WithGraphImages(graphImages),
		)
		slideBuilder = builder
	})
	return slideBuilder, builderErr
}

// GenerateCaptcha creates a sliding puzzle captcha, persists it, and returns
// the client-facing information.
func (s *Service) GenerateCaptcha(ctx context.Context) (*Captcha, error) {
	builder, err := getBuilder()
	if err != nil {
		return nil, err
	}

	// security: go-captcha derives the answer position from a math/rand-based
	// PRNG (time-seeded pool) rather than crypto/rand. This is acceptable
	// here because the PRNG output is never observable — answers are not
	// returned and cannot be recovered from the rendered image — and one-shot
	// invalidation leaves a single-bit verification oracle per challenge, so
	// the seed-space predictability cannot be exploited remotely. Revisit if
	// the verify flow ever exposes more feedback.
	capt, err := builder.Make().Generate()
	if err != nil {
		return nil, fmt.Errorf("generate slide captcha: %w", err)
	}
	block := capt.GetData()
	if block == nil {
		return nil, slide.GenerateDataErr
	}

	masterImage, err := capt.GetMasterImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodeBgImage, err)
	}
	tileImage, err := capt.GetTileImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncodePuzzleImage, err)
	}

	// block.X/Y is the hole the client must drop the tile on — that is the
	// answer. block.DX/DY is only the tile's initial display position and
	// must never be used for validation. The images are returned to the
	// client directly and are not persisted.
	captcha := model.Captcha{
		ID:        uuid.New().String(),
		Token:     uuid.New().String(),
		X:         block.X,
		Y:         block.Y,
		Width:     block.Width,
		Height:    block.Height,
		ExpiresAt: time.Now().Add(captchaTTL),
		Used:      false,
	}
	if err := s.repo.CreateCaptcha(ctx, &captcha); err != nil {
		return nil, fmt.Errorf("persist captcha: %w", err)
	}

	// Opportunistic housekeeping: drop challenges that are no longer valid,
	// bounding the table to roughly the challenges of the last TTL window.
	if err := s.repo.DeleteExpiredCaptchas(ctx); err != nil {
		slog.Warn("failed to clean up expired captchas", "err", err)
	}

	return &Captcha{
		ID:          captcha.ID,
		Token:       captcha.Token,
		ThumbX:      block.DX,
		ThumbY:      block.DY,
		ThumbWidth:  captcha.Width,
		ThumbHeight: captcha.Height,
		Image:       masterImage,
		Thumb:       tileImage,
		ExpiresAt:   captcha.ExpiresAt,
	}, nil
}

// VerifyCaptcha verifies a sliding puzzle submission and marks the captcha as
// used.
func (s *Service) VerifyCaptcha(ctx context.Context, args VerifyArgs) error {
	// Query captcha
	captcha, err := s.repo.FindCaptcha(ctx, args.ID, args.Token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCaptchaNotFound
		}
		// Unexpected lookup failure: report it as an internal error instead
		// of masquerading as a missing challenge.
		slog.Error("captcha lookup failed", "err", err)
		return ErrCaptchaFailed
	}

	// Check if already used
	if captcha.Used {
		return ErrCaptchaUsed
	}

	// Check if expired
	if time.Now().After(captcha.ExpiresAt) {
		return ErrCaptchaExpired
	}

	// Verify the drop position on both axes within the tolerance padding.
	// The challenge is one-shot: the first failed attempt invalidates it so
	// the answer cannot be brute-forced within its lifetime.
	if !slide.Validate(args.X, args.Y, captcha.X, captcha.Y, verifyPadding) {
		if _, err := s.repo.MarkCaptchaUsed(ctx, args.ID, args.Token); err != nil {
			return err
		}
		return ErrCaptchaMismatch
	}

	// Atomically claim the challenge; losing the race means a concurrent
	// request consumed it first.
	claimed, err := s.repo.MarkCaptchaUsed(ctx, args.ID, args.Token)
	if err != nil {
		return err
	}
	if !claimed {
		return ErrCaptchaUsed
	}

	return nil
}
