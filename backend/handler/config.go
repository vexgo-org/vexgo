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

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// IsCaptchaEnabled 检查是否启用了滑块验证
func IsCaptchaEnabled() (bool, error) {
	var settings model.GeneralSettings
	if err := db.First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 默认不启用
			return false, nil
		}
		return false, err
	}
	return settings.CaptchaEnabled, nil
}

// GetSMTPConfig 获取 SMTP 配置
func GetSMTPConfig(c *gin.Context) {
	var config model.SMTPConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回默认配置
			c.JSON(http.StatusOK, model.SMTPConfig{
				Enabled:   false,
				Host:      "",
				Port:      587,
				Username:  "",
				Password:  "",
				FromEmail: "",
				FromName:  "Blog System",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SMTP config"})
		return
	}

	// 不返回密码字段
	config.Password = ""
	c.JSON(http.StatusOK, config)
}

// UpdateSMTPConfig 更新 SMTP 配置
func UpdateSMTPConfig(c *gin.Context) {
	var req struct {
		Enabled   bool   `json:"enabled"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"` // 如果为空则不更新密码
		FromEmail string `json:"fromEmail"`
		FromName  string `json:"fromName"`
		TestEmail string `json:"testEmail"` // 测试邮件收件人邮箱
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取现有配置
	var config model.SMTPConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新配置
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
		// 更新现有配置
		config.Enabled = req.Enabled
		config.Host = req.Host
		config.Port = req.Port
		config.Username = req.Username
		config.FromEmail = req.FromEmail
		config.FromName = req.FromName
		config.TestEmail = req.TestEmail

		// 只有当提供了新密码时才更新密码
		if req.Password != "" {
			config.Password = req.Password
		}

		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update SMTP config"})
			return
		}
	}

	// 返回配置，但不包含密码
	config.Password = ""
	c.JSON(http.StatusOK, config)
}

// TestSMTP 测试 SMTP 配置
func TestSMTP(c *gin.Context) {
	// 获取当前管理员用户邮箱（从 JWT token 中获取）
	userContext, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	// 获取 SMTP 配置
	var config model.SMTPConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "请先配置 SMTP 设置"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SMTP config"})
		return
	}

	// 检查是否启用
	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP 未启用，请先启用并保存配置"})
		return
	}

	// 检查必要字段
	if config.Host == "" || config.Port == 0 || config.Username == "" || config.Password == "" || config.FromEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请完整填写 SMTP 配置信息"})
		return
	}

	// 获取收件人邮箱：优先使用配置的测试邮箱，否则使用当前管理员邮箱
	var recipientEmail string
	if config.TestEmail != "" {
		recipientEmail = config.TestEmail
	} else {
		// 回退到使用管理员邮箱
		if userMap, ok := userContext.(map[string]interface{}); ok {
			if email, ok := userMap["email"].(string); ok {
				recipientEmail = email
			}
		}
	}
	if recipientEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先填写测试邮箱地址"})
		return
	}

	// 发送测试邮件
	subject := "SMTP 配置测试邮件"
	textBody := fmt.Sprintf(`
尊敬的 %s，

这是一封测试邮件，用于验证您的 SMTP 配置是否正确工作。

如果您收到此邮件，说明 SMTP 配置成功！

配置信息：
- SMTP 服务器: %s:%d
- 发件人: %s <%s>

时间: %s
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
            <h1>SMTP 配置测试</h1>
        </div>
        <div class="content">
            <p>尊敬的 %s，</p>
            <p class="success">✓ 测试邮件发送成功！</p>
            <p>您的 SMTP 配置已正确工作，可以开始使用邮件验证功能了。</p>
            
            <div class="info">
                <strong>配置信息：</strong><br>
                SMTP 服务器: %s:%d<br>
                发件人: %s <%s>
            </div>
            
            <p>时间: %s</p>
        </div>
    </div>
</body>
</html>
	`, config.FromName, config.Host, config.Port, config.FromName, config.FromEmail, time.Now().Format("2006-01-02 15:04:05"))

	// 构建邮件
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

	// 连接 SMTP 服务器
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{to}, []byte(message)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("发送测试邮件失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "测试邮件已发送到您的邮箱",
		"to":      recipientEmail,
	})
}

// GetGeneralSettings 获取通用设置
func GetGeneralSettings(c *gin.Context) {
	var config model.GeneralSettings
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回默认配置
			c.JSON(http.StatusOK, model.GeneralSettings{
				CaptchaEnabled:      false,
				RegistrationEnabled: true,
				SiteName:            "Blog System",
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

// UpdateGeneralSettings 更新通用设置
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

	// 获取现有配置
	var config model.GeneralSettings
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新配置
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
		// 更新现有配置
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

// GetAIConfig 获取 AI 配置
func GetAIConfig(c *gin.Context) {
	var config model.AIConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回默认配置
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

	// 不返回API密钥字段
	config.ApiKey = ""
	c.JSON(http.StatusOK, config)
}

// UpdateAIConfig 更新 AI 配置
func UpdateAIConfig(c *gin.Context) {
	var req struct {
		Enabled     bool   `json:"enabled"`
		Provider    string `json:"provider"`
		ApiEndpoint string `json:"apiEndpoint"`
		ApiKey      string `json:"apiKey"` // 如果为空则不更新API密钥
		ModelName   string `json:"modelName"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取现有配置
	var config model.AIConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新配置
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
		// 更新现有配置
		config.Enabled = req.Enabled
		config.Provider = req.Provider
		config.ApiEndpoint = req.ApiEndpoint
		config.ModelName = req.ModelName

		// 只有当提供了新API密钥时才更新
		if req.ApiKey != "" {
			config.ApiKey = req.ApiKey
		}

		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update AI config"})
			return
		}
	}

	// 返回配置，但不包含API密钥
	config.ApiKey = ""
	c.JSON(http.StatusOK, gin.H{
		"message":  "AI config updated successfully",
		"aiConfig": config,
	})
}

// TestAI 测试 AI 配置连接
func TestAI(c *gin.Context) {
	// 获取当前管理员用户信息（从 JWT token 中获取）
	_, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	// 获取 AI 配置
	var config model.AIConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "请先配置 AI 设置"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get AI config"})
		return
	}

	// 检查是否启用
	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI 未启用，请先启用并保存配置"})
		return
	}

	// 检查必要字段
	if config.ApiEndpoint == "" || config.ApiKey == "" || config.ModelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请完整填写 AI 配置信息（端点、API密钥、模型名称）"})
		return
	}

	// 构建基础API URL
	// 移除末尾斜杠，确保格式一致
	baseURL := strings.TrimSuffix(config.ApiEndpoint, "/")

	// 如果用户输入的是完整聊天端点（以 /chat/completions 结尾），提取基础部分
	if strings.HasSuffix(baseURL, "/v1/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
		baseURL = strings.TrimSuffix(baseURL, "/v1")
		baseURL = baseURL + "/v1"
	} else if strings.HasSuffix(baseURL, "/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	}

	// 确保baseURL以 /v1 结尾
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}

	// 构建聊天完成端点
	chatCompletionsURL := baseURL + "/chat/completions"

	// 构建模型验证端点
	modelsURL := baseURL + "/models"

	// 获取测试提示词：使用简单的测试问题
	testPrompt := "Say this is a test"

	// 步骤1: 验证模型是否存在（可选，但推荐）
	modelExists, modelErr := checkModelExists(modelsURL, config.ApiKey, config.ModelName)
	if modelErr != nil {
		// 模型检查失败，但继续测试聊天完成，可能只是该端点不支持模型列表
		fmt.Printf("模型验证警告 (将继续测试): %v\n", modelErr)
	} else if !modelExists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("模型 '%s' 不存在或不可用，请检查模型名称", config.ModelName),
		})
		return
	}

	// 步骤2: 测试聊天完成功能
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

	// 发送 HTTP 请求到 AI API
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

	// 解析响应
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse AI response"})
		return
	}

	// 检查是否有错误字段
	if errorMsg, ok := result["error"]; ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("AI API error: %v", errorMsg),
		})
		return
	}

	// 提取AI回复内容
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
		"message":  "AI 连接测试成功！",
		"response": aiResponse,
	})
}

// GetAIModels 获取可用的AI模型列表
func GetAIModels(c *gin.Context) {
	// 获取 AI 配置
	var config model.AIConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "请先配置 AI 设置"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get AI config"})
		return
	}

	// 检查是否启用
	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI 未启用，请先启用并保存配置"})
		return
	}

	// 检查必要字段
	if config.ApiEndpoint == "" || config.ApiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请完整填写 AI 配置信息（端点、API密钥）"})
		return
	}

	// 构建基础API URL（与TestAI函数保持一致）
	baseURL := strings.TrimSuffix(config.ApiEndpoint, "/")

	// 如果用户输入的是完整聊天端点（以 /chat/completions 结尾），提取基础部分
	if strings.HasSuffix(baseURL, "/v1/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
		baseURL = strings.TrimSuffix(baseURL, "/v1")
		baseURL = baseURL + "/v1"
	} else if strings.HasSuffix(baseURL, "/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	}

	// 确保baseURL以 /v1 结尾
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}

	// 构建模型列表端点
	modelsURL := baseURL + "/models"

	// 发送请求获取模型列表
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

	// 解析响应
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse models response"})
		return
	}

	// 检查是否有错误
	if errorMsg, ok := result["error"]; ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("API error: %v", errorMsg),
		})
		return
	}

	// 提取模型列表
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

// checkModelExists 检查模型是否存在
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

	// 检查模型列表
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
