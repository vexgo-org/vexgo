package comment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// chatCompletionRequest mirrors the wire format of an OpenAI-compatible chat
// completion request body.
type chatCompletionRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// fakeLLMServer is an OpenAI-compatible chat completion server that records
// every request and replies with a canned message content and status.
type fakeLLMServer struct {
	*httptest.Server

	mu        sync.Mutex
	requests  int
	lastAuth  string
	lastModel string
	lastBody  chatCompletionRequest
}

// newFakeLLMServer starts a fake server replying with the given choices[0]
// message content and HTTP status. An empty reply means an empty choices
// array.
func newFakeLLMServer(t *testing.T, reply string, status int) *fakeLLMServer {
	t.Helper()
	fake := &fakeLLMServer{}
	fake.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.mu.Lock()
		fake.requests++
		fake.lastAuth = r.Header.Get("Authorization")
		body := chatCompletionRequest{}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			fake.lastBody = body
			if len(body.Messages) > 0 {
				fake.lastModel = body.Model
			}
		}
		fake.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if reply == "" && status == http.StatusOK {
			_, _ = w.Write([]byte(`{"choices":[]}`))
			return
		}
		response := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": reply}},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(fake.Close)
	return fake
}

func (f *fakeLLMServer) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests
}

// configureLLM enables LLM review against the fake server and returns the
// service and fake for assertions.
func configureLLM(t *testing.T, svc *Service, manual bool, endpoint string) {
	t.Helper()
	if _, err := svc.UpdateModerationConfig(context.Background(), UpdateModerationConfigRequest{
		ManualReviewEnabled: manual,
		LLMReviewEnabled:    true,
		ApiKey:              "test-key",
		ApiEndpoint:         endpoint,
		ModelName:           "test-model",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}
}

// TC-CMOD-006/007/008/009: LLM verdict handling across the manual-review
// switch — approve publishes (manual off) or holds (manual on), reject
// rejects with the model's reason persisted.
func TestCreate_LLMVerdicts(t *testing.T) {
	cases := []struct {
		name       string
		reply      string
		manual     bool
		wantStatus model.CommentStatus
		wantReason string
	}{
		{"approve, manual off", `{"approved": true, "reason": ""}`, false, model.CommentStatusPublished, ""},
		{"approve, manual on", `{"approved": true, "reason": "fine"}`, true, model.CommentStatusPending, ""},
		{"reject", `{"approved": false, "reason": "harassment"}`, false, model.CommentStatusRejected, "harassment"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			svc, notifier, db := newTestService(t)
			fake := newFakeLLMServer(t, tt.reply, http.StatusOK)
			configureLLM(t, svc, tt.manual, fake.URL)
			author := seedUser(t, db, "author", model.RoleContributor)
			post := seedPost(t, db, author.ID)
			commenter := seedUser(t, db, "commenter", model.RoleGuest)

			comment, _, err := svc.Create(context.Background(), CreateRequest{
				PostID: post.ID, UserID: commenter.ID, Content: "some comment",
			})
			if err != nil {
				t.Fatalf("Create error: %v", err)
			}
			if comment.Status != tt.wantStatus {
				t.Errorf("expected %s, got %s", tt.wantStatus, comment.Status)
			}
			if comment.ModerationReason != tt.wantReason {
				t.Errorf("expected reason %q, got %q", tt.wantReason, comment.ModerationReason)
			}

			// TC-CMOD-014: only published comments notify.
			wantNotifications := 0
			if tt.wantStatus == model.CommentStatusPublished {
				wantNotifications = 1
			}
			if len(notifier.calls) != wantNotifications {
				t.Errorf("expected %d notifications, got %v", wantNotifications, notifier.calls)
			}
		})
	}
}

// TC-CMOD-010/011/012: any LLM failure (HTTP error, timeout, non-JSON reply)
// fails closed to pending, even with manual review off.
func TestCreate_LLMFailures_PendingEvenWithoutManualReview(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, svc *Service) *fakeLLMServer
	}{
		{
			name: "http 500",
			setup: func(t *testing.T, svc *Service) *fakeLLMServer {
				fake := newFakeLLMServer(t, `{"approved": true, "reason": ""}`, http.StatusInternalServerError)
				configureLLM(t, svc, false, fake.URL)
				return fake
			},
		},
		{
			name: "non-JSON reply",
			setup: func(t *testing.T, svc *Service) *fakeLLMServer {
				fake := newFakeLLMServer(t, "This comment looks fine to me!", http.StatusOK)
				configureLLM(t, svc, false, fake.URL)
				return fake
			},
		},
		{
			name: "timeout",
			setup: func(t *testing.T, svc *Service) *fakeLLMServer {
				fake := newFakeLLMServer(t, `{"approved": true, "reason": ""}`, http.StatusOK)
				// Respond slower than the injected client timeout.
				fake.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(200 * time.Millisecond)
					_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"approved\":true}"}}]}`))
				})
				svc.llmClient = &http.Client{Timeout: 50 * time.Millisecond}
				configureLLM(t, svc, false, fake.URL)
				return fake
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, db := newTestService(t)
			tt.setup(t, svc)
			author := seedUser(t, db, "author", model.RoleContributor)
			post := seedPost(t, db, author.ID)
			commenter := seedUser(t, db, "commenter", model.RoleGuest)

			comment, _, err := svc.Create(context.Background(), CreateRequest{
				PostID: post.ID, UserID: commenter.ID, Content: "some comment",
			})
			if err != nil {
				t.Fatalf("Create error: %v", err)
			}
			if comment.Status != model.CommentStatusPending {
				t.Errorf("expected fail-closed pending, got %s", comment.Status)
			}
			if comment.ModerationReason == "" {
				t.Errorf("expected a moderation reason explaining the hold")
			}
		})
	}
}

// TC-CMOD-005: when the keyword filter and LLM review are both on, a keyword
// hit rejects the comment and short-circuits the LLM — zero HTTP requests
// reach the fake server.
func TestCreate_KeywordHit_ShortCircuitsLLM(t *testing.T) {
	svc, _, db := newTestService(t)
	fake := newFakeLLMServer(t, `{"approved": true, "reason": ""}`, http.StatusOK)
	if _, err := svc.UpdateModerationConfig(context.Background(), UpdateModerationConfigRequest{
		KeywordFilterEnabled: true,
		LLMReviewEnabled:     true,
		BlockKeywords:        "spam",
		ApiKey:               "test-key",
		ApiEndpoint:          fake.URL,
		ModelName:            "test-model",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	comment, _, err := svc.Create(context.Background(), CreateRequest{
		PostID: post.ID, UserID: commenter.ID, Content: "buy my spam",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.Status != model.CommentStatusRejected {
		t.Errorf("expected rejected, got %s", comment.Status)
	}
	if got := fake.requestCount(); got != 0 {
		t.Errorf("expected zero LLM requests after a keyword hit, got %d", got)
	}
}

// TC-CMOD-013: the LLM request carries the stored key as a bearer token, the
// configured model name, and the comment content substituted into the prompt.
func TestCreate_LLMRequestShape(t *testing.T) {
	svc, _, db := newTestService(t)
	fake := newFakeLLMServer(t, `{"approved": true, "reason": ""}`, http.StatusOK)
	if _, err := svc.UpdateModerationConfig(context.Background(), UpdateModerationConfigRequest{
		LLMReviewEnabled: true,
		ApiKey:           "test-key",
		ApiEndpoint:      fake.URL,
		ModelName:        "test-model",
		ModerationPrompt: "Moderate this: {{content}}",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, _, err := svc.Create(context.Background(), CreateRequest{
		PostID: post.ID, UserID: commenter.ID, Content: "the comment body",
	}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if fake.lastAuth != "Bearer test-key" {
		t.Errorf("expected bearer authorization, got %q", fake.lastAuth)
	}
	if fake.lastModel != "test-model" {
		t.Errorf("expected model test-model, got %q", fake.lastModel)
	}
	if len(fake.lastBody.Messages) != 1 || fake.lastBody.Messages[0].Role != "user" {
		t.Fatalf("expected a single user message, got %+v", fake.lastBody.Messages)
	}
	if fake.lastBody.Messages[0].Content != "Moderate this: the comment body" {
		t.Errorf("expected {{content}} substituted, got %q", fake.lastBody.Messages[0].Content)
	}
}

// TC-CMOD-016/017: the config test endpoint succeeds against a reachable
// fake endpoint and reports incomplete configuration with
// ErrLLMConfigIncomplete.
func TestTestModerationLLM(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, _, _ := newTestService(t)
		fake := newFakeLLMServer(t, `{"approved": true, "reason": "ok"}`, http.StatusOK)
		configureLLM(t, svc, false, fake.URL)

		result, err := svc.TestModerationLLM(context.Background())
		if err != nil {
			t.Fatalf("TestModerationLLM error: %v", err)
		}
		if !strings.Contains(result.Response, "approved=true") {
			t.Errorf("expected verdict summary in response, got %q", result.Response)
		}
	})

	t.Run("incomplete config", func(t *testing.T) {
		svc, _, _ := newTestService(t)
		if _, err := svc.TestModerationLLM(context.Background()); err != ErrLLMConfigIncomplete {
			t.Errorf("expected ErrLLMConfigIncomplete, got %v", err)
		}
	})

	t.Run("unreachable endpoint", func(t *testing.T) {
		svc, _, _ := newTestService(t)
		configureLLM(t, svc, false, "http://127.0.0.1:1")
		if _, err := svc.TestModerationLLM(context.Background()); err == nil {
			t.Errorf("expected an error for an unreachable endpoint, got nil")
		}
	})
}

// TC-CMOD-015: enabling LLM review without a stored or provided API key and
// endpoint is rejected; a stored key satisfies the requirement on update.
func TestUpdateModerationConfig_LLMRequiresCredentials(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestService(t)

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		LLMReviewEnabled: true,
		ApiEndpoint:      "https://llm.example.com",
	}); err != ErrLLMConfigIncomplete {
		t.Errorf("expected ErrLLMConfigIncomplete without api key, got %v", err)
	}
	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		LLMReviewEnabled: true,
		ApiKey:           "test-key",
	}); err != ErrLLMConfigIncomplete {
		t.Errorf("expected ErrLLMConfigIncomplete without endpoint, got %v", err)
	}

	var stored model.CommentModerationConfig
	if err := db.First(&stored).Error; err == nil {
		t.Errorf("no config row must be written for a rejected update")
	}

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		LLMReviewEnabled: true,
		ApiKey:           "test-key",
		ApiEndpoint:      "https://llm.example.com",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	// A later update without a key keeps the stored one and stays valid.
	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		LLMReviewEnabled: true,
		ApiEndpoint:      "https://llm.example.com",
	}); err != nil {
		t.Errorf("stored api key should satisfy the credential check, got %v", err)
	}
}

func TestChatCompletionsURL(t *testing.T) {
	cases := []struct{ endpoint, want string }{
		{"https://api.example.com", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/v1", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/chat/completions", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/v1/chat/completions", "https://api.example.com/v1/chat/completions"},
	}
	for _, tt := range cases {
		if got := chatCompletionsURL(tt.endpoint); got != tt.want {
			t.Errorf("chatCompletionsURL(%q) = %q, want %q", tt.endpoint, got, tt.want)
		}
	}
}

func TestParseModerationVerdict(t *testing.T) {
	cases := []struct {
		name    string
		reply   string
		want    moderationVerdict
		wantErr bool
	}{
		{"plain json", `{"approved": true, "reason": "ok"}`, moderationVerdict{true, "ok"}, false},
		{"fenced json", "```json\n{\"approved\": false, \"reason\": \"spam\"}\n```", moderationVerdict{false, "spam"}, false},
		{"prose reply", "This comment is fine.", moderationVerdict{}, true},
		{"json array", `[1,2,3]`, moderationVerdict{}, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseModerationVerdict(tt.reply)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseModerationVerdict(%q) error = %v, wantErr %v", tt.reply, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("parseModerationVerdict(%q) = %+v, want %+v", tt.reply, got, tt.want)
			}
		})
	}
}

func TestBuildModerationPrompt(t *testing.T) {
	t.Run("substitutes placeholder", func(t *testing.T) {
		got := buildModerationPrompt("Check {{content}} please", "hello")
		if got != "Check hello please" {
			t.Errorf("unexpected prompt %q", got)
		}
	})
	t.Run("appends missing content", func(t *testing.T) {
		got := buildModerationPrompt("Check this please", "hello")
		if !strings.Contains(got, "Check this please") || !strings.Contains(got, "hello") {
			t.Errorf("expected content appended, got %q", got)
		}
	})
	t.Run("default prompt", func(t *testing.T) {
		got := buildModerationPrompt("", "hello")
		if !strings.Contains(got, `"approved"`) || !strings.Contains(got, "hello") {
			t.Errorf("expected default JSON prompt with content, got %q", got)
		}
	})
}

// A moderation reason that exceeds the moderation_reason column limit (an
// oversized model reply or keyword) must be truncated so comment persistence
// cannot fail on strict databases.
func TestCreate_ModerationReasonTruncatedToColumnLimit(t *testing.T) {
	svc, _, db := newTestService(t)
	oversized := strings.Repeat("语", 600) // 600 runes, far past the 500-rune column
	fake := newFakeLLMServer(t, `{"approved": false, "reason": "`+oversized+`"}`, http.StatusOK)
	configureLLM(t, svc, false, fake.URL)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	comment, _, err := svc.Create(context.Background(), CreateRequest{
		PostID: post.ID, UserID: commenter.ID, Content: "some comment",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.Status != model.CommentStatusRejected {
		t.Errorf("expected rejected, got %s", comment.Status)
	}
	if got := len([]rune(comment.ModerationReason)); got != maxModerationReason {
		t.Errorf("expected reason truncated to %d runes, got %d", maxModerationReason, got)
	}

	// The persisted row must match (the insert itself must not error).
	var stored model.Comment
	if err := db.First(&stored, comment.ID).Error; err != nil {
		t.Fatalf("load stored comment: %v", err)
	}
	if got := len([]rune(stored.ModerationReason)); got != maxModerationReason {
		t.Errorf("expected stored reason truncated to %d runes, got %d", maxModerationReason, got)
	}
}
