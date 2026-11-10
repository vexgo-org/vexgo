package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

const (
	// maxAIProviderBodyBytes caps how much of an AI provider response is held
	// in memory. The endpoint is admin-configurable, so a misconfigured or
	// hostile provider could otherwise stream an arbitrarily large JSON
	// document into the process. Model lists and the bounded test completion
	// are far below this.
	maxAIProviderBodyBytes = 4 << 20 // 4 MiB

	// aiErrorSnippetBytes caps how much of a failing provider response is
	// quoted into the error message. That message reaches both the admin
	// response and the server log, so quoting a whole error page would be
	// worse than useless.
	aiErrorSnippetBytes = 512
)

// AIResult carries the outcome of an AI connection test or model listing.
type AIResult struct {
	Message  string
	Response any
}

// AIConfigRequest carries the fields accepted when updating the AI config.
type AIConfigRequest struct {
	Enabled     bool
	Provider    string
	ApiEndpoint string
	ApiKey      string // if empty, don't update API key
	ModelName   string
}

// GetAIConfig returns the stored AI configuration with the API key masked,
// or the defaults when no row exists.
func (s *Service) GetAIConfig(ctx context.Context) (model.AIConfig, error) {
	config, err := s.repo.GetAIConfig(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return default configuration
			return model.AIConfig{
				Enabled:     false,
				Provider:    "openai",
				ApiEndpoint: "",
				ApiKey:      "",
				ModelName:   "gpt-3.5-turbo",
			}, nil
		}
		return config, err
	}

	// Don't return API key field
	config.ApiKey = ""
	return config, nil
}

// UpdateAIConfig creates or updates the AI configuration and returns it with
// the API key masked.
func (s *Service) UpdateAIConfig(ctx context.Context, req AIConfigRequest) (model.AIConfig, error) {
	config, err := s.repo.GetAIConfig(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new configuration
			apiKey, encErr := s.encryptSecret(req.ApiKey, "ai_config.api_key")
			if encErr != nil {
				return model.AIConfig{}, encErr
			}
			config = model.AIConfig{
				Enabled:     req.Enabled,
				Provider:    req.Provider,
				ApiEndpoint: req.ApiEndpoint,
				ApiKey:      apiKey,
				ModelName:   req.ModelName,
			}
			if err := s.repo.CreateAIConfig(ctx, &config); err != nil {
				return config, fmt.Errorf("failed to create AI config: %w", err)
			}
		} else {
			return config, fmt.Errorf("failed to get AI config: %w", err)
		}
	} else {
		// Update existing configuration
		config.Enabled = req.Enabled
		config.Provider = req.Provider
		config.ApiEndpoint = req.ApiEndpoint
		config.ModelName = req.ModelName

		// Only update API key if new API key is provided
		if req.ApiKey != "" {
			apiKey, encErr := s.encryptSecret(req.ApiKey, "ai_config.api_key")
			if encErr != nil {
				return config, encErr
			}
			config.ApiKey = apiKey
		}

		if err := s.repo.SaveAIConfig(ctx, &config); err != nil {
			return config, fmt.Errorf("failed to update AI config: %w", err)
		}
	}

	// Return configuration without API key
	config.ApiKey = ""
	return config, nil
}

// TestAI verifies the AI configuration by calling the chat completions
// endpoint with a test prompt.
func (s *Service) TestAI(ctx context.Context) (*AIResult, error) {
	config, err := s.repo.GetAIConfig(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAINotConfigured
		}
		return nil, err
	}

	// The API key is stored encrypted; decrypt it for the outbound call.
	config.ApiKey = s.decryptSecret(config.ApiKey, "ai_config.api_key")

	// Check if enabled
	if !config.Enabled {
		return nil, ErrAIDisabled
	}

	// Check required fields
	if config.ApiEndpoint == "" || config.ApiKey == "" || config.ModelName == "" {
		return nil, ErrAIIncomplete
	}

	// Build base API URL
	// Remove trailing slash to ensure consistent format
	baseURL := strings.TrimSuffix(config.ApiEndpoint, "/")

	// If user entered full chat endpoint (ending with /chat/completions), extract base part
	if strings.HasSuffix(baseURL, "/v1/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
		baseURL = strings.TrimSuffix(baseURL, "/v1")
		baseURL = baseURL + "/v1"
	} else if before, ok := strings.CutSuffix(baseURL, "/chat/completions"); ok {
		baseURL = before
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
		slog.Warn("model validation warning (will continue test)", "err", modelErr)
	} else if !modelExists {
		return nil, fmt.Errorf("model '%s' does not exist or is not available, please check model name", config.ModelName)
	}

	// Step 2: Test chat completion functionality
	requestBody := map[string]any{
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
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", chatCompletionsURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.ApiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI API returned status %d: %s", resp.StatusCode, aiErrorBody(resp))
	}

	// Parse response
	body, err := readAIProviderBody(resp)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	// Check for error field
	if errorMsg, ok := result["error"]; ok {
		return nil, fmt.Errorf("AI API error: %v", errorMsg)
	}

	// Extract AI response content
	var aiResponse string
	if choices, ok := result["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok {
					aiResponse = content
				}
			}
		}
	}

	return &AIResult{
		Message:  "AI connection test successful!",
		Response: aiResponse,
	}, nil
}

// AIModels returns the list of models available from the configured AI
// endpoint.
func (s *Service) AIModels(ctx context.Context) (*AIResult, error) {
	config, err := s.repo.GetAIConfig(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAINotConfigured
		}
		return nil, err
	}

	// The API key is stored encrypted; decrypt it for the outbound call.
	config.ApiKey = s.decryptSecret(config.ApiKey, "ai_config.api_key")

	// Check if enabled
	if !config.Enabled {
		return nil, ErrAIDisabled
	}

	// Check required fields
	if config.ApiEndpoint == "" || config.ApiKey == "" {
		return nil, ErrAIIncompleteModels
	}

	// Build base API URL (consistent with TestAI function)
	baseURL := strings.TrimSuffix(config.ApiEndpoint, "/")

	// If user entered full chat endpoint (ending with /chat/completions), extract base part
	if strings.HasSuffix(baseURL, "/v1/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
		baseURL = strings.TrimSuffix(baseURL, "/v1")
		baseURL = baseURL + "/v1"
	} else if before, ok := strings.CutSuffix(baseURL, "/chat/completions"); ok {
		baseURL = before
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
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.ApiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch models, status: %d, response: %s", resp.StatusCode, aiErrorBody(resp))
	}

	// Parse response
	body, err := readAIProviderBody(resp)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	// Check for errors
	if errorMsg, ok := result["error"]; ok {
		return nil, fmt.Errorf("API error: %v", errorMsg)
	}

	// Extract models list
	var models []map[string]any
	if data, ok := result["data"].([]any); ok {
		for _, model := range data {
			if modelMap, ok := model.(map[string]any); ok {
				modelInfo := map[string]any{
					"id":       modelMap["id"],
					"object":   modelMap["object"],
					"created":  modelMap["created"],
					"owned_by": modelMap["owned_by"],
				}
				models = append(models, modelInfo)
			}
		}
	}

	return &AIResult{
		Message:  "Models fetched successfully",
		Response: models,
	}, nil
}

// readAIProviderBody reads a provider response body, capped at
// maxAIProviderBodyBytes. An oversize body is reported as an error rather than
// silently truncated, so a partial payload is never mistaken for a valid one.
// Callers decode the returned bytes with json.Unmarshal instead of streaming
// the body through json.Decoder, because the cap must apply while reading.
func readAIProviderBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAIProviderBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read AI provider response: %w", err)
	}
	if len(body) > maxAIProviderBodyBytes {
		return nil, fmt.Errorf("AI provider response exceeds %d bytes", maxAIProviderBodyBytes)
	}
	return body, nil
}

// aiErrorBody reads a bounded snippet of a failing provider response for
// quoting into an error message. A read failure is folded into the snippet
// instead of discarding the status code: the status is the signal that matters,
// and dropping it would be a worse outcome than losing the body.
func aiErrorBody(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, aiErrorSnippetBytes))
	if err != nil {
		return fmt.Sprintf("(body unreadable: %v)", err)
	}
	return string(body)
}

// checkModelExists checks if model exists
func checkModelExists(modelsURL, apiKey, modelName string) (bool, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to connect to models endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("models endpoint returned status %d: %s", resp.StatusCode, aiErrorBody(resp))
	}

	body, err := readAIProviderBody(resp)
	if err != nil {
		return false, err
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("failed to parse models response: %w", err)
	}

	// Check models list
	if data, ok := result["data"].([]any); ok {
		for _, model := range data {
			if modelMap, ok := model.(map[string]any); ok {
				if id, ok := modelMap["id"].(string); ok && id == modelName {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
