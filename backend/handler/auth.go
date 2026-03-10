package handler

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"vexgo/backend/config"
	"vexgo/backend/model"
	"vexgo/backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Filter user information based on privacy settings
func FilterUserByPrivacy(user *model.User, viewerID uint, viewerRole string) {
	// If the user has set privacy, need to decide whether to hide information based on viewer's identity
	// Current implementation: the user themselves and administrators can see all information
	// Other users see information according to privacy settings

	// Check if viewer is the user themselves or an admin
	isSelf := viewerID == user.ID
	isAdmin := viewerRole == model.RoleAdmin || viewerRole == model.RoleSuperAdmin

	// If not self and not admin, filter according to privacy settings
	if !isSelf && !isAdmin {
		// First check profile visibility setting
		if user.ProfileVisibility == "private" {
			// If set to private, hide all personal information
			user.Email = ""
			user.Birthday = ""
			user.Bio = ""
		} else {
			// If public, filter according to individual hide settings
			if user.HideEmail {
				user.Email = ""
			}
			if user.HideBirthday {
				user.Birthday = ""
			}
			if user.HideBio {
				user.Bio = ""
			}
		}
	}
}

// Public function to verify sliding puzzle captcha
func verifyCaptcha(captchaID, captchaToken string, captchaX int, markAsUsed bool) (*model.Captcha, error) {
	// Query captcha
	var captcha model.Captcha
	if err := db.Where("id = ? AND token = ?", captchaID, captchaToken).First(&captcha).Error; err != nil {
		return nil, fmt.Errorf("invalid captcha")
	}

	// Check if captcha has been used
	if captcha.Used {
		return nil, fmt.Errorf("captcha already used")
	}

	// Check if captcha has expired
	if time.Now().After(captcha.ExpiresAt) {
		return nil, fmt.Errorf("captcha has expired")
	}

	// Verify position (allow certain tolerance)
	tolerance := 10 // allow 5 pixel tolerance
	if math.Abs(float64(captchaX-captcha.X)) > float64(tolerance) {
		return nil, fmt.Errorf("verification failed, please try again")
	}

	// If need to mark as used
	if markAsUsed {
		captcha.Used = true
		if err := db.Save(&captcha).Error; err != nil {
			return nil, fmt.Errorf("captcha verification failed")
		}
	}

	return &captcha, nil
}

// Login: issue JWT based on email and password
func Login(c *gin.Context) {
	var req struct {
		Email        string `json:"email" binding:"required"`
		Password     string `json:"password" binding:"required"`
		CaptchaID    string `json:"captcha_id"`
		CaptchaToken string `json:"captcha_token"`
		CaptchaX     int    `json:"captcha_x"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if captcha verification is enabled
	captchaEnabled, err := IsCaptchaEnabled()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check captcha settings"})
		return
	}

	// If captcha verification is enabled, verify captcha
	if captchaEnabled {
		if req.CaptchaID == "" || req.CaptchaToken == "" || req.CaptchaX == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Please complete the captcha verification"})
			return
		}
		// Query captcha
		var captcha model.Captcha
		if err := db.Where("id = ? AND token = ?", req.CaptchaID, req.CaptchaToken).First(&captcha).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Captcha does not exist or has expired"})
			return
		}

		// Check if expired
		if time.Now().After(captcha.ExpiresAt) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Captcha has expired"})
			return
		}

		// Verify position (allow certain tolerance)
		tolerance := 10
		if math.Abs(float64(req.CaptchaX-captcha.X)) > float64(tolerance) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Verification failed, please try again"})
			return
		}

		// If captcha has not been used yet, mark it as used
		if !captcha.Used {
			captcha.Used = true
			if err := db.Save(&captcha).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Captcha verification failed"})
				return
			}
		}
		// If captcha already used, pre-verification successful, pass directly
	}

	var user model.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid email or password"})
		return
	}

	// Use bcrypt to compare hashed password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid email or password"})
		return
	}

	// Check if SMTP is enabled, if so verify email status
	mailer := utils.NewMailer(db)
	enabled, err := mailer.IsEmailEnabled()
	if err == nil && enabled && !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{
			"message":        "Please verify your email address first. Check your inbox and click the verification link, or request to resend the verification email.",
			"email_verified": false,
		})
		return
	}

	// Generate token
	claims := jwt.MapClaims{
		"user_id":          user.ID,
		"username":         user.Username,
		"role":             user.Role,
		"password_version": user.PasswordVersion,
		"exp":              time.Now().Add(24 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),                                                  // Timestamp
		"jti":              fmt.Sprintf("%d-%s", user.ID, time.Now().Format(time.RFC3339Nano)), // Unique identifier
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(config.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": ss,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// Get current user information
func GetCurrentUser(c *gin.Context) {
	if uid, ok := c.Get("userID"); ok {
		var user model.User
		if err := db.First(&user, uid).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": user})
		return
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
}

// Update post (requires authentication, only author or admin can modify)
func UpdatePost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post
	if err := db.Preload("Tags").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
		return
	}

	// Permission check
	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err == nil {
		if user.Role != "admin" && user.Role != "super_admin" && post.AuthorID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to modify this post"})
			return
		}
	}

	// reuse postRequest definition from post.go by redeclaring locally
	var req struct {
		Title      string      `json:"title"`
		Content    string      `json:"content"`
		Category   interface{} `json:"category"`
		Tags       []string    `json:"tags"`
		Excerpt    string      `json:"excerpt"`
		CoverImage string      `json:"coverImage"`
		Status     string      `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != "" {
		post.Title = req.Title
	}
	if req.Content != "" {
		post.Content = req.Content
	}
	if req.Category != nil {
		switch v := req.Category.(type) {
		case string:
			post.Category = v
		case float64:
			post.Category = strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			post.Category = strconv.Itoa(v)
		case int64:
			post.Category = strconv.FormatInt(v, 10)
		default:
			post.Category = fmt.Sprintf("%v", v)
		}
	}
	if req.Excerpt != "" {
		post.Excerpt = req.Excerpt
	}
	if req.CoverImage != "" {
		post.CoverImage = req.CoverImage
	}
	if req.Status != "" {
		post.Status = req.Status
	}

	if len(req.Tags) > 0 {
		tags, err := resolveTags(req.Tags)
		if err == nil {
			db.Model(&post).Association("Tags").Replace(tags)
			post.Tags = tags
		}
	}

	db.Save(&post)
	c.JSON(http.StatusOK, gin.H{"message": "Post updated successfully", "post": post})
}

// Extract all image URLs from HTML content
func extractImageURLs(content string) []string {
	var urls []string

	// Match src attribute of <img> tags
	// Regex matches src="..." or src='...' or src=...
	re := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["'][^>]*>`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			url := strings.TrimSpace(match[1])
			if url != "" {
				urls = append(urls, url)
			}
		}
	}

	return urls
}

// Delete file (must be uploader or admin)
func deleteImageFile(url string) error {
	// Extract filename from URL, e.g.: /uploads/abc123.jpg -> abc123.jpg
	filename := filepath.Base(url)
	if filename == "" || filename == "/" {
		return fmt.Errorf("invalid filename: %s", url)
	}

	// Build full path using DataDir defined in upload.go
	path := filepath.Join(DataDir, "media", filename)

	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		// File exists, delete it
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to delete file: %w", err)
		}
	}

	return nil
}

// Delete post (requires authentication, only author or admin can delete)
func DeletePost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post
	if err := db.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post does not exist"})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err == nil {
		if user.Role != "admin" && user.Role != "super_admin" && post.AuthorID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this post"})
			return
		}
	}

	// Collect image URLs to delete
	var imagesToDelete []string

	// 1. Add cover image
	if post.CoverImage != "" {
		imagesToDelete = append(imagesToDelete, post.CoverImage)
	}

	// 2. Extract all images from post content
	contentImages := extractImageURLs(post.Content)
	imagesToDelete = append(imagesToDelete, contentImages...)

	// 3. Delete all collected image files (after deduplication)
	uniqueImages := make(map[string]bool)
	for _, url := range imagesToDelete {
		uniqueImages[url] = true
	}

	for url := range uniqueImages {
		if err := deleteImageFile(url); err != nil {
			// Log error but continue execution to avoid post deletion failure
			fmt.Printf("Failed to delete image %s: %v\n", url, err)
		}
	}

	// Delete post (GORM automatically deletes associated comments, likes, etc.)
	db.Delete(&post)
	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

// Update user profile
func UpdateProfile(c *gin.Context) {
	var req struct {
		Username *string `json:"username"`
		Avatar   *string `json:"avatar"`
		Birthday *string `json:"birthday"`
		Bio      *string `json:"bio"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	// If updating avatar, delete old avatar
	if req.Avatar != nil && *req.Avatar != user.Avatar && user.Avatar != "" {
		// Delete old avatar file
		if err := deleteImageFile(user.Avatar); err != nil {
			// Log error but continue execution to avoid avatar update failure
			fmt.Printf("Failed to delete old avatar %s: %v\n", user.Avatar, err)
		}
		user.Avatar = *req.Avatar
	} else if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}

	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Birthday != nil {
		user.Birthday = *req.Birthday
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	db.Save(&user)
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// Change password
func ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt password"})
		return
	}

	// Increment password version to invalidate old tokens
	user.Password = string(hashed)
	user.PasswordVersion++
	db.Save(&user)
	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// Update user settings (privacy settings, etc.)
func UpdateSettings(c *gin.Context) {
	var req struct {
		ProfileVisibility *string `json:"profile_visibility"`
		HideEmail         *bool   `json:"hide_email"`
		HideBirthday      *bool   `json:"hide_birthday"`
		HideBio           *bool   `json:"hide_bio"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	if req.ProfileVisibility != nil {
		user.ProfileVisibility = *req.ProfileVisibility
	}
	if req.HideEmail != nil {
		user.HideEmail = *req.HideEmail
	}
	if req.HideBirthday != nil {
		user.HideBirthday = *req.HideBirthday
	}
	if req.HideBio != nil {
		user.HideBio = *req.HideBio
	}

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Settings updated successfully",
		"user":    user,
	})
}

// Update email
func UpdateEmail(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	// Check if new email is the same as current email
	if req.Email == user.Email {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New email cannot be the same as current email"})
		return
	}

	// Check if new email is already used by another user
	var existingUser model.User
	if err := db.Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This email is already used by another user"})
		return
	}

	// Check if SMTP is enabled
	mailer := utils.NewMailer(db)
	enabled, err := mailer.IsEmailEnabled()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check mail configuration"})
		return
	}

	if enabled {
		// If SMTP enabled, generate email change verification token and send confirmation email
		token, err := mailer.GenerateEmailChangeToken(userID, req.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
			return
		}

		// Build verification link
		protocol := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			protocol = "https"
		}
		host := c.Request.Host
		verificationLink := fmt.Sprintf("%s://%s/verify-email?token=%s", protocol, host, token)

		// Send confirmation email
		if err := mailer.SendEmailChangeEmail(user.Email, user.Username, req.Email, verificationLink); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Verification email sent. Please check your inbox and click the link to complete email change.",
			"pending": true,
		})
	} else {
		// If SMTP not enabled, update email directly
		if err := db.Model(&user).Update("email", req.Email).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update email"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Email updated successfully",
			"pending": false,
			"user": gin.H{
				"email": req.Email,
			},
		})
	}
}

// Request password reset
func RequestPasswordReset(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user
	var user model.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// For security reasons, return success even if user doesn't exist to avoid information leakage
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, reset link has been sent"})
		return
	}

	// Check if SMTP is enabled
	mailer := utils.NewMailer(db)
	enabled, err := mailer.IsEmailEnabled()
	if err != nil || !enabled {
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, reset link has been sent"})
		return
	}

	// Generate password reset token
	token, err := mailer.GeneratePasswordResetToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}

	// Build reset link - use request protocol and hostname
	protocol := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		protocol = "https"
	}
	host := c.Request.Host
	resetLink := fmt.Sprintf("%s://%s/reset-password?token=%s", protocol, host, token)

	// Send email
	if err := mailer.SendPasswordResetEmail(user.Email, user.Username, resetLink); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, reset link has been sent"})
}

// Reset password
func ResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user with this token
	var user model.User
	if err := db.Where("verification_token = ?", req.Token).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reset token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}

	// Check if token has expired
	if user.TokenExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reset token has expired"})
		return
	}

	// Generate hash for new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt password"})
		return
	}

	// Update password and clear reset token
	if err := db.Model(&user).Updates(map[string]interface{}{
		"password":           string(hashed),
		"verification_token": "",
		"token_expires_at":   time.Time{},
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}
