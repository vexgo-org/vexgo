// Package captcha implements sliding-puzzle captcha generation and
// verification. HTTP handlers are registered with huma and live
// alongside the service layer in this package.
//
// The service layer (service.go) is unchanged from the previous
// gin-based implementation; only the HTTP boundary moves to huma.
// The handler functions take (context.Context, *Input) and return
// (*Output, error); the service is still driven by the request
// context.
package captcha

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/vexgo-org/vexgo/backend/internal/api"
)

// generateInput is empty: GET /api/captcha takes no body, query, or
// path parameters.
type generateInput struct{}

// generateOutput wraps the captcha response. huma renders the Body
// field as the JSON response body.
type generateOutput struct {
	Body api.CaptchaGenerateResponse
}

// verifyInput is the POST /api/captcha/verify body. huma binds the
// JSON body to the Body field.
type verifyInput struct {
	Body api.CaptchaVerifyRequest
}

// verifyOutput wraps the verification result.
type verifyOutput struct {
	Body api.CaptchaVerifyResponse
}

// RegisterRoutes registers the captcha domain operations on the
// given huma.API. The rate-limit middleware is still a gin middleware
// — humagin runs gin middleware before dispatching to the huma
// handler, so attaching it to the gin group at the caller is the
// only change needed.
func (h *Handler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "generate-captcha",
		Method:      http.MethodGet,
		Path:        "/captcha",
		Summary:     "Generate a sliding puzzle captcha",
		Tags:        []string{"captcha"},
	}, h.generate)

	huma.Register(api, huma.Operation{
		OperationID: "verify-captcha",
		Method:      http.MethodPost,
		Path:        "/captcha/verify",
		Summary:     "Verify a captcha solution",
		Tags:        []string{"captcha"},
	}, h.verify)
}

// generate is the huma handler for GET /api/captcha.
func (h *Handler) generate(ctx context.Context, _ *generateInput) (*generateOutput, error) {
	c, err := h.svc.GenerateCaptcha(ctx)
	if err != nil {
		switch {
		case errors.Is(err, ErrEncodeBgImage):
			return nil, huma.NewError(500, "Failed to encode background image")
		case errors.Is(err, ErrEncodePuzzleImage):
			return nil, huma.NewError(500, "Failed to encode puzzle image")
		default:
			return nil, huma.NewError(500, "Failed to generate captcha")
		}
	}
	return &generateOutput{
		Body: api.CaptchaGenerateResponse{
			ID:          c.ID,
			Token:       c.Token,
			ThumbX:      c.ThumbX,
			ThumbY:      c.ThumbY,
			ThumbWidth:  c.ThumbWidth,
			ThumbHeight: c.ThumbHeight,
			Image:       c.Image,
			Thumb:       c.Thumb,
			ExpiresAt:   c.ExpiresAt,
		},
	}, nil
}

// verify is the huma handler for POST /api/captcha/verify.
func (h *Handler) verify(ctx context.Context, in *verifyInput) (*verifyOutput, error) {
	err := h.svc.VerifyCaptcha(ctx, VerifyArgs{
		ID:    in.Body.ID,
		Token: in.Body.Token,
		X:     in.Body.X,
		Y:     in.Body.Y,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrCaptchaNotFound):
			return nil, huma.NewError(404, "Captcha does not exist or has expired")
		case errors.Is(err, ErrCaptchaUsed):
			return nil, huma.NewError(400, "Captcha already used")
		case errors.Is(err, ErrCaptchaExpired):
			return nil, huma.NewError(400, "Captcha has expired")
		case errors.Is(err, ErrCaptchaMismatch):
			return nil, huma.NewError(400, "Verification failed, please try again")
		default:
			return nil, huma.NewError(500, "Captcha verification failed")
		}
	}
	return &verifyOutput{
		Body: api.CaptchaVerifyResponse{Success: true, Message: "Verification successful"},
	}, nil
}
