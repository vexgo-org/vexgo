package comment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// llmTimeout bounds each moderation LLM call so comment creation cannot hang
// on an unresponsive endpoint.
const llmTimeout = 15 * time.Second

// llmMaxResponseBytes caps how much of a chat completion response body is
// read, so a misbehaving endpoint cannot grow server memory without bound.
const llmMaxResponseBytes = 64 << 10 // 64 KiB

// moderationVerdict is the strict JSON decision the moderation prompt asks
// the model to return.
type moderationVerdict struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason"`
}

// ModerationTestResult carries the outcome of a moderation LLM connectivity
// test triggered from the admin configuration form.
type ModerationTestResult struct {
	Message  string `json:"message"`
	Response string `json:"response"`
}

// reviewWithLLM asks the configured OpenAI-compatible endpoint to moderate
// the comment and parses the verdict. Any transport failure, non-200 status,
// or non-JSON reply is an error so the caller can fail closed to the manual
// queue instead of publishing.
func (s *Service) reviewWithLLM(
	ctx context.Context,
	content string,
	config model.CommentModerationConfig,
) (moderationVerdict, error) {
	reply, err := s.callLLM(ctx, buildModerationPrompt(config.ModerationPrompt, content), config)
	if err != nil {
		return moderationVerdict{}, err
	}
	return parseModerationVerdict(reply)
}

// TestModerationLLM verifies the stored moderation configuration by sending a
// short test comment through the real review path.
func (s *Service) TestModerationLLM(ctx context.Context) (*ModerationTestResult, error) {
	config, err := s.moderationConfig(ctx)
	if err != nil {
		return nil, err
	}
	if config.ApiEndpoint == "" || config.ApiKey == "" || config.ModelName == "" {
		return nil, ErrLLMConfigIncomplete
	}

	const testContent = "This is a VexGo comment moderation connectivity test."
	reply, err := s.callLLM(ctx, buildModerationPrompt(config.ModerationPrompt, testContent), config)
	if err != nil {
		return nil, err
	}
	verdict, err := parseModerationVerdict(reply)
	if err != nil {
		return nil, err
	}
	return &ModerationTestResult{
		Message: "LLM moderation endpoint reachable",
		Response: fmt.Sprintf("model %s replied: approved=%t, reason=%s",
			config.ModelName, verdict.Approved, verdict.Reason),
	}, nil
}

// callLLM posts one chat completion request to the configured endpoint and
// returns the model's reply content.
func (s *Service) callLLM(
	ctx context.Context,
	prompt string,
	config model.CommentModerationConfig,
) (string, error) {
	requestBody, err := json.Marshal(map[string]any{
		"model":       config.ModelName,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0,
	})
	if err != nil {
		return "", fmt.Errorf("marshal LLM moderation request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, chatCompletionsURL(config.ApiEndpoint), bytes.NewReader(requestBody),
	)
	if err != nil {
		return "", fmt.Errorf("create LLM moderation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.ApiKey)

	resp, err := s.llmClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call LLM moderation endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("LLM moderation endpoint returned status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, llmMaxResponseBytes)).Decode(&result); err != nil {
		return "", fmt.Errorf("parse LLM moderation response: %w", err)
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", errors.New("LLM moderation response has no message content")
	}
	return result.Choices[0].Message.Content, nil
}

// buildModerationPrompt substitutes the comment content into the configured
// prompt; a prompt without the {{content}} placeholder gets the content
// appended so the model always reviews the actual comment.
func buildModerationPrompt(prompt, content string) string {
	if prompt == "" {
		prompt = defaultModerationPrompt
	}
	if !strings.Contains(prompt, "{{content}}") {
		prompt += "\n\nComment content:\n{{content}}"
	}
	return strings.ReplaceAll(prompt, "{{content}}", content)
}

// parseModerationVerdict parses the strict JSON verdict the moderation prompt
// asks for. Markdown code fences, which many models wrap around JSON, are
// tolerated; anything else is a failure so callers can fail closed.
func parseModerationVerdict(reply string) (moderationVerdict, error) {
	trimmed := strings.TrimSpace(reply)
	if after, ok := strings.CutPrefix(trimmed, "```"); ok {
		after = strings.TrimPrefix(after, "json")
		if end := strings.LastIndex(after, "```"); end >= 0 {
			after = after[:end]
		}
		trimmed = strings.TrimSpace(after)
	}

	var verdict moderationVerdict
	if err := json.Unmarshal([]byte(trimmed), &verdict); err != nil {
		return moderationVerdict{}, fmt.Errorf("LLM moderation reply is not a JSON verdict: %w", err)
	}
	return verdict, nil
}

// chatCompletionsURL normalizes a stored API endpoint into the chat
// completions URL. A full ".../chat/completions" or ".../v1/chat/completions"
// value is accepted; a bare base URL gets "/v1/chat/completions" appended.
func chatCompletionsURL(endpoint string) string {
	base := strings.TrimSuffix(endpoint, "/")
	if before, ok := strings.CutSuffix(base, "/chat/completions"); ok {
		base = before
	}
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	return base + "/chat/completions"
}
