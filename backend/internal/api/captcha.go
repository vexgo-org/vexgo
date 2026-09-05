package api

// VerifyCaptchaRequest is the body of POST /api/captcha/verify.
type VerifyCaptchaRequest struct {
	ID    string `json:"id" binding:"required"`
	Token string `json:"token" binding:"required"`
	X     int    `json:"x" binding:"required"`
	Y     int    `json:"y" binding:"required"`
}
