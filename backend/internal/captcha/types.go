// Wire types for the captcha domain. These are the shapes the
// JSON responses use; swag reads the struct tags to populate
// the OpenAPI spec and orval turns them into TypeScript
// interfaces on the frontend.
package captcha

// CaptchaGenerateResponse is the body of GET /api/captcha. The
// shape maps directly onto the go-captcha-react Slide
// component's data prop.
type CaptchaGenerateResponse struct {
	// ID is the unique captcha handle; the client sends it back
	// to /api/captcha/verify to prove it solved the puzzle.
	ID string `json:"id" example:"ck_2f4e..."`

	// Token is the verification token, separate from the id. The
	// /verify endpoint requires both.
	Token string `json:"token" example:"ct_2f4e..."`

	// ThumbX / ThumbY are the on-screen coordinates where the
	// client should place the puzzle piece. The puzzle is
	// always anchored at the right edge of the image; thumbX
	// is the horizontal offset.
	ThumbX int `json:"thumbX" example:"40"`

	// ThumbY is the vertical offset.
	ThumbY int `json:"thumbY" example:"120"`

	// ThumbWidth / ThumbHeight are the puzzle piece dimensions.
	ThumbWidth  int `json:"thumbWidth" example:"50"`
	ThumbHeight int `json:"thumbHeight" example:"50"`

	// Image is the base64-encoded background PNG (data URL).
	Image string `json:"image" example:"data:image/png;base64,..."`

	// Thumb is the base64-encoded puzzle piece PNG (data URL).
	Thumb string `json:"thumb" example:"data:image/png;base64,..."`

	// ExpiresAt is the RFC3339 timestamp after which the
	// captcha is no longer accepted. The client should request
	// a new captcha past this time.
	ExpiresAt string `json:"expires_at" example:"2026-09-06T15:00:00Z"`
}

// CaptchaVerifyRequest is the body of POST /api/captcha/verify.
type CaptchaVerifyRequest struct {
	ID    string `json:"id" binding:"required" example:"ck_2f4e..."`
	Token string `json:"token" binding:"required" example:"ct_2f4e..."`
	// X is the horizontal pixel offset where the user dropped
	// the puzzle piece.
	X int `json:"x" binding:"required" example:"42"`
	// Y is the vertical pixel offset where the user dropped the
	// puzzle piece.
	Y int `json:"y" binding:"required" example:"118"`
}

// CaptchaVerifyResponse is the body of POST /api/captcha/verify
// on success.
type CaptchaVerifyResponse struct {
	// Success is always true on a 200 OK; on failure the
	// endpoint returns a 4xx with an ErrorResponse body instead.
	Success bool `json:"success" example:"true"`
	// Message is a human-readable confirmation.
	Message string `json:"message" example:"Verification successful"`
}
