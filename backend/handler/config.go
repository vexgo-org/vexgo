package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"vexgo/backend/model"
	"vexgo/backend/public"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// IsCaptchaEnabled checks if captcha verification is enabled
func IsCaptchaEnabled() (bool, error) {
	var settings model.GeneralSettings
	if err := db.First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Not enabled by default
			return false, nil
		}
		return false, err
	}
	return settings.CaptchaEnabled, nil
}

// GetSMTPConfig gets SMTP configuration
func GetSMTPConfig(c *gin.Context) {
	var config model.SMTPConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default configuration
			c.JSON(http.StatusOK, model.SMTPConfig{
				Enabled:   false,
				Host:      "",
				Port:      587,
				Username:  "",
				Password:  "",
				FromEmail: "",
				FromName:  "VexGo",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SMTP config"})
		return
	}

	// Don't return password field
	config.Password = ""
	c.JSON(http.StatusOK, config)
}

// UpdateSMTPConfig updates SMTP configuration
func UpdateSMTPConfig(c *gin.Context) {
	var req struct {
		Enabled   bool   `json:"enabled"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"` // if empty, don't update password
		FromEmail string `json:"fromEmail"`
		FromName  string `json:"fromName"`
		TestEmail string `json:"testEmail"` // test email recipient
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing configuration
	var config model.SMTPConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new configuration
			config = model.SMTPConfig{
				Enabled:   req.Enabled,
				Host:      req.Host,
				Port:      req.Port,
				Username:  req.Username,
				Password:  req.Password,
				FromEmail: req.FromEmail,
				FromName:  req.FromName,
				TestEmail: req.TestEmail,
			}
			if err := db.Create(&config).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SMTP config"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SMTP config"})
			return
		}
	} else {
		// Update existing configuration
		config.Enabled = req.Enabled
		config.Host = req.Host
		config.Port = req.Port
		config.Username = req.Username
		config.FromEmail = req.FromEmail
		config.FromName = req.FromName
		config.TestEmail = req.TestEmail

		// Only update password if new password is provided
		if req.Password != "" {
			config.Password = req.Password
		}

		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update SMTP config"})
			return
		}
	}

	// Return configuration without password
	config.Password = ""
	c.JSON(http.StatusOK, config)
}

// TestSMTP tests SMTP configuration
func TestSMTP(c *gin.Context) {
	// Get current admin user email (from JWT token)
	userContext, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get SMTP configuration
	var config model.SMTPConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Please configure SMTP settings first"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SMTP config"})
		return
	}

	// Check if enabled
	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP is not enabled, please enable and save configuration first"})
		return
	}

	// Check required fields
	if config.Host == "" || config.Port == 0 || config.Username == "" || config.Password == "" || config.FromEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please fill in all SMTP configuration fields"})
		return
	}

	// Get recipient email: use configured test email first, otherwise use current admin email
	var recipientEmail string
	if config.TestEmail != "" {
		recipientEmail = config.TestEmail
	} else {
		// Fallback to admin email
		if userMap, ok := userContext.(map[string]interface{}); ok {
			if email, ok := userMap["email"].(string); ok {
				recipientEmail = email
			}
		}
	}
	if recipientEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please fill in test email address first"})
		return
	}

	// Send test email
	subject := "SMTP Configuration Test Email"
	textBody := fmt.Sprintf(`
Dear %s,

This is a test email to verify your SMTP configuration is working correctly.

If you receive this email, it means your SMTP configuration is successful!

Configuration details:
- SMTP Server: %s:%d
- Sender: %s <%s>

Time: %s
	`, config.FromName, config.Host, config.Port, config.FromName, config.FromEmail, time.Now().Format("2006-01-02 15:04:05"))

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background-color: #f9f9f9; }
        .success { color: #4CAF50; font-weight: bold; }
        .info { background-color: #e3f2fd; padding: 15px; border-radius: 4px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>SMTP Configuration Test</h1>
        </div>
        <div class="content">
            <p>Dear %s,</p>
            <p class="success">✓ Test email sent successfully!</p>
            <p>Your SMTP configuration is working correctly. You can now use email verification features.</p>

            <div class="info">
                <strong>Configuration details:</strong><br>
                SMTP Server: %s:%d<br>
                Sender: %s <%s>
            </div>

            <p>Time: %s</p>
        </div>
    </div>
</body>
</html>
	`, config.FromName, config.Host, config.Port, config.FromName, config.FromEmail, time.Now().Format("2006-01-02 15:04:05"))

	// Build email message
	from := fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	to := recipientEmail

	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary\""

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n"
	message += "--boundary\r\n"
	message += "Content-Type: text/plain; charset=UTF-8\r\n\r\n"
	message += strings.TrimSpace(textBody) + "\r\n\r\n"
	message += "--boundary\r\n"
	message += "Content-Type: text/html; charset=UTF-8\r\n\r\n"
	message += strings.TrimSpace(htmlBody) + "\r\n\r\n"
	message += "--boundary--\r\n"

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{to}, []byte(message)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send test email: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test email has been sent to your inbox",
		"to":      recipientEmail,
	})
}

// GetGeneralSettings gets general settings
func GetGeneralSettings(c *gin.Context) {
	var config model.GeneralSettings
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default configuration
			c.JSON(http.StatusOK, model.GeneralSettings{
				CaptchaEnabled:      false,
				RegistrationEnabled: true,
				SiteName:            "VexGo",
				SiteDescription:     "",
				ItemsPerPage:        20,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get general settings"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateGeneralSettings updates general settings
func UpdateGeneralSettings(c *gin.Context) {
	var req struct {
		CaptchaEnabled      bool   `json:"captchaEnabled"`
		RegistrationEnabled bool   `json:"registrationEnabled"`
		SiteName            string `json:"siteName"`
		SiteDescription     string `json:"siteDescription"`
		ItemsPerPage        int    `json:"itemsPerPage"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing configuration
	var config model.GeneralSettings
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new configuration
			config = model.GeneralSettings{
				CaptchaEnabled:      req.CaptchaEnabled,
				RegistrationEnabled: req.RegistrationEnabled,
				SiteName:            req.SiteName,
				SiteDescription:     req.SiteDescription,
				ItemsPerPage:        req.ItemsPerPage,
			}
			if err := db.Create(&config).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create general settings"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get general settings"})
			return
		}
	} else {
		// Update existing configuration
		config.CaptchaEnabled = req.CaptchaEnabled
		config.RegistrationEnabled = req.RegistrationEnabled
		config.SiteName = req.SiteName
		config.SiteDescription = req.SiteDescription
		config.ItemsPerPage = req.ItemsPerPage

		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update general settings"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "General settings updated successfully",
		"generalSettings": config,
	})
}

// GetAIConfig gets AI configuration
func GetAIConfig(c *gin.Context) {
	var config model.AIConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default configuration
			c.JSON(http.StatusOK, model.AIConfig{
				Enabled:     false,
				Provider:    "openai",
				ApiEndpoint: "",
				ApiKey:      "",
				ModelName:   "gpt-3.5-turbo",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get AI config"})
		return
	}

	// Don't return API key field
	config.ApiKey = ""
	c.JSON(http.StatusOK, config)
}

// UpdateAIConfig updates AI configuration
func UpdateAIConfig(c *gin.Context) {
	var req struct {
		Enabled     bool   `json:"enabled"`
		Provider    string `json:"provider"`
		ApiEndpoint string `json:"apiEndpoint"`
		ApiKey      string `json:"apiKey"` // if empty, don't update API key
		ModelName   string `json:"modelName"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing configuration
	var config model.AIConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new configuration
			config = model.AIConfig{
				Enabled:     req.Enabled,
				Provider:    req.Provider,
				ApiEndpoint: req.ApiEndpoint,
				ApiKey:      req.ApiKey,
				ModelName:   req.ModelName,
			}
			if err := db.Create(&config).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create AI config"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get AI config"})
			return
		}
	} else {
		// Update existing configuration
		config.Enabled = req.Enabled
		config.Provider = req.Provider
		config.ApiEndpoint = req.ApiEndpoint
		config.ModelName = req.ModelName

		// Only update API key if new API key is provided
		if req.ApiKey != "" {
			config.ApiKey = req.ApiKey
		}

		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update AI config"})
			return
		}
	}

	// Return configuration without API key
	config.ApiKey = ""
	c.JSON(http.StatusOK, gin.H{
		"message":  "AI config updated successfully",
		"aiConfig": config,
	})
}

// TestAI tests AI configuration connection
func TestAI(c *gin.Context) {
	// Get current admin user information (from JWT token)
	_, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get AI configuration
	var config model.AIConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Please configure AI settings first"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get AI config"})
		return
	}

	// Check if enabled
	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI is not enabled, please enable and save configuration first"})
		return
	}

	// Check required fields
	if config.ApiEndpoint == "" || config.ApiKey == "" || config.ModelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please fill in all AI configuration fields (endpoint, API key, model name)"})
		return
	}

	// Build base API URL
	// Remove trailing slash to ensure consistent format
	baseURL := strings.TrimSuffix(config.ApiEndpoint, "/")

	// If user entered full chat endpoint (ending with /chat/completions), extract base part
	if strings.HasSuffix(baseURL, "/v1/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
		baseURL = strings.TrimSuffix(baseURL, "/v1")
		baseURL = baseURL + "/v1"
	} else if strings.HasSuffix(baseURL, "/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	}

	// Ensure baseURL ends with /v1
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}

	// Build chat completion endpoint
	chatCompletionsURL := baseURL + "/chat/completions"

	// Build model validation endpoint
	modelsURL := baseURL + "/models"

	// Get test prompt: use simple test question
	testPrompt := "Say this is a test"

	// Step 1: Verify model exists (optional but recommended)
	modelExists, modelErr := checkModelExists(modelsURL, config.ApiKey, config.ModelName)
	if modelErr != nil {
		// Model check failed, but continue testing chat completion, endpoint may not support model listing
		fmt.Printf("Model validation warning (will continue test): %v\n", modelErr)
	} else if !modelExists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Model '%s' does not exist or is not available, please check model name", config.ModelName),
		})
		return
	}

	// Step 2: Test chat completion functionality
	requestBody := map[string]interface{}{
		"model": config.ModelName,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": testPrompt,
			},
		},
		"max_tokens":  100,
		"temperature": 0.7,
	}

	// Send HTTP request to AI API
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal request"})
		return
	}

	req, err := http.NewRequest("POST", chatCompletionsURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.ApiKey)

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to AI API: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("AI API returned status %d: %s", resp.StatusCode, string(body)),
		})
		return
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse AI response"})
		return
	}

	// Check for error field
	if errorMsg, ok := result["error"]; ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("AI API error: %v", errorMsg),
		})
		return
	}

	// Extract AI response content
	var aiResponse string
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					aiResponse = content
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "AI connection test successful!",
		"response": aiResponse,
	})
}

// GetAIModels gets available AI model list
func GetAIModels(c *gin.Context) {
	// Get AI configuration
	var config model.AIConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Please configure AI settings first"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get AI config"})
		return
	}

	// Check if enabled
	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI is not enabled, please enable and save configuration first"})
		return
	}

	// Check required fields
	if config.ApiEndpoint == "" || config.ApiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please fill in all AI configuration fields (endpoint, API key)"})
		return
	}

	// Build base API URL (consistent with TestAI function)
	baseURL := strings.TrimSuffix(config.ApiEndpoint, "/")

	// If user entered full chat endpoint (ending with /chat/completions), extract base part
	if strings.HasSuffix(baseURL, "/v1/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
		baseURL = strings.TrimSuffix(baseURL, "/v1")
		baseURL = baseURL + "/v1"
	} else if strings.HasSuffix(baseURL, "/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	}

	// Ensure baseURL ends with /v1
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}

	// Build models list endpoint
	modelsURL := baseURL + "/models"

	// Send request to fetch models list
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create request: %v", err)})
		return
	}

	req.Header.Set("Authorization", "Bearer "+config.ApiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch models: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to fetch models, status: %d, response: %s", resp.StatusCode, string(body)),
		})
		return
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse models response"})
		return
	}

	// Check for errors
	if errorMsg, ok := result["error"]; ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("API error: %v", errorMsg),
		})
		return
	}

	// Extract models list
	var models []map[string]interface{}
	if data, ok := result["data"].([]interface{}); ok {
		for _, model := range data {
			if modelMap, ok := model.(map[string]interface{}); ok {
				modelInfo := map[string]interface{}{
					"id":       modelMap["id"],
					"object":   modelMap["object"],
					"created":  modelMap["created"],
					"owned_by": modelMap["owned_by"],
				}
				models = append(models, modelInfo)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Models fetched successfully",
		"models":  models,
	})
}

// checkModelExists checks if model exists
func checkModelExists(modelsURL, apiKey, modelName string) (bool, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to connect to models endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("models endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to parse models response: %v", err)
	}

	// Check models list
	if data, ok := result["data"].([]interface{}); ok {
		for _, model := range data {
			if modelMap, ok := model.(map[string]interface{}); ok {
				if id, ok := modelMap["id"].(string); ok && id == modelName {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// GetThemes returns all available themes
func GetThemes(c *gin.Context) {
	themes := public.GetAvailableThemes()
	c.JSON(http.StatusOK, gin.H{
		"themes": themes,
	})
}

// GetThemeConfig returns the currently active theme stored in the database
func GetThemeConfig(c *gin.Context) {
	var config model.ThemeConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{"activeTheme": public.DefaultTheme})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get theme config"})
		return
	}
	activeTheme := config.ActiveTheme
	if activeTheme == "" {
		activeTheme = public.DefaultTheme
	}
	c.JSON(http.StatusOK, gin.H{"activeTheme": activeTheme})
}

// UpdateThemeConfig sets the globally active theme in the database
func UpdateThemeConfig(c *gin.Context) {
	var req struct {
		ActiveTheme string `json:"activeTheme" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate that the requested theme actually exists
	if !public.ThemeExists(req.ActiveTheme) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Theme not found"})
		return
	}

	var config model.ThemeConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			config = model.ThemeConfig{ActiveTheme: req.ActiveTheme}
			if err := db.Create(&config).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save theme config"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get theme config"})
			return
		}
	} else {
		config.ActiveTheme = req.ActiveTheme
		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update theme config"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Theme updated successfully",
		"activeTheme": config.ActiveTheme,
	})
}

// SetupThemeProvider injects a DB-backed theme provider into the public package.
// This must be called after InitDB so that db is available.
func SetupThemeProvider() {
	public.ActiveThemeProvider = func() string {
		var config model.ThemeConfig
		if err := db.First(&config).Error; err != nil {
			return public.DefaultTheme
		}
		if config.ActiveTheme == "" {
			return public.DefaultTheme
		}
		return config.ActiveTheme
	}
}
