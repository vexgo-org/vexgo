package api

import "time"

// CaptchaGenerateResponse is the body of GET /api/captcha. The shape
// maps onto the go-captcha-react Slide component's data prop; the
// puzzle answer is never included. ExpiresAt is a time.Time so huma
// emits `format: date-time`; the original handler relied on Gin's
// time.Time → RFC3339 marshaling, which is preserved.
type CaptchaGenerateResponse struct {
	ID         string    `json:"id"`
	Token      string    `json:"token"`
	ThumbX     int       `json:"thumbX"`
	ThumbY     int       `json:"thumbY"`
	ThumbWidth int       `json:"thumbWidth"`
	ThumbHeight int      `json:"thumbHeight"`
	Image      string    `json:"image"`
	Thumb      string    `json:"thumb"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// CaptchaVerifyRequest is the body of POST /api/captcha/verify.
type CaptchaVerifyRequest struct {
	ID    string `json:"id" required:"" doc:"Captcha ID"`
	Token string `json:"token" required:"" doc:"Verification token"`
	X     int    `json:"x" required:"" doc:"Puzzle X coordinate"`
	Y     int    `json:"y" required:"" doc:"Puzzle Y coordinate"`
}

// CaptchaVerifyResponse is the body of POST /api/captcha/verify.
type CaptchaVerifyResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
