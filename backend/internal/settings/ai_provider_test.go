package settings

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/public"

	"gorm.io/gorm"
)

// aiStubService returns a settings service pointed at a stub AI provider that
// answers the model list, then the chat completion with chatStatus/chatBody.
// The provider endpoint is admin-configurable, so these stubs stand in for a
// misconfigured or hostile provider.
func aiStubService(t *testing.T, chatStatus int, chatBody []byte) *Service {
	t.Helper()
	const modelName = "test-model"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"data":[{"id":%q}]}`, modelName)
		case "/v1/chat/completions":
			w.WriteHeader(chatStatus)
			_, _ = w.Write(chatBody)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	db := newTestDB(t)
	svc := NewService(Deps{
		DB:     db,
		Themes: public.NewRenderer(db, "http://localhost", t.TempDir()),
		Mailer: mailer.NewService(mailer.Deps{DB: db}),
	})
	if err := db.Create(&model.AIConfig{
		Enabled:     true,
		Provider:    "openai",
		ApiEndpoint: server.URL,
		ApiKey:      "sk-test",
		ModelName:   modelName,
	}).Error; err != nil {
		t.Fatalf("seed AI config: %v", err)
	}
	return svc
}

// A provider response larger than the cap must be rejected instead of being
// buffered whole: the endpoint is admin-configurable, so its size is not
// something the server controls.
func TestAITest_RejectsOversizeProviderResponse(t *testing.T) {
	oversize := bytes.Repeat([]byte("x"), maxAIProviderBodyBytes+1)
	svc := aiStubService(t, http.StatusOK, oversize)

	_, err := svc.TestAI(context.Background())
	if err == nil {
		t.Fatal("expected an error for an oversize provider response")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected the cap to be reported, got %v", err)
	}
}

// The failing-response body is quoted into an error message that reaches both
// the admin response and the server log, so the quote must stay small.
func TestAITest_ErrorBodyQuoteIsBounded(t *testing.T) {
	oversize := bytes.Repeat([]byte("x"), maxAIProviderBodyBytes+1)
	svc := aiStubService(t, http.StatusBadGateway, oversize)

	_, err := svc.TestAI(context.Background())
	if err == nil {
		t.Fatal("expected an error for a non-200 provider response")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("expected the status code in the message, got %v", err)
	}
	if got := len(err.Error()); got > 4*aiErrorSnippetBytes {
		t.Errorf("error message quotes %d bytes of the provider body; cap is %d", got, aiErrorSnippetBytes)
	}
}

// wrappedNotFoundRepository returns the not-found sentinel wrapped with context,
// the way a repository that adds context to its errors does. Embedding
// Repository leaves the lookups these tests do not exercise unimplemented.
type wrappedNotFoundRepository struct {
	Repository
}

func (wrappedNotFoundRepository) GetAIConfig(context.Context) (model.AIConfig, error) {
	return model.AIConfig{}, fmt.Errorf("load ai config: %w", gorm.ErrRecordNotFound)
}

func (wrappedNotFoundRepository) GetSMTPConfig(context.Context) (model.SMTPConfig, error) {
	return model.SMTPConfig{}, fmt.Errorf("load smtp config: %w", gorm.ErrRecordNotFound)
}

// A wrapped gorm.ErrRecordNotFound must still be read as "no row stored yet".
// A bare == comparison would not match, silently turning the documented
// first-run defaults into a 500 once a repository starts wrapping its errors.
func TestNotFoundRecognizedThroughWrapping(t *testing.T) {
	svc := &Service{repo: wrappedNotFoundRepository{}}

	if _, err := svc.AIModels(context.Background()); !errors.Is(err, ErrAINotConfigured) {
		t.Errorf("AIModels: expected ErrAINotConfigured through a wrapped not-found, got %v", err)
	}

	config, err := svc.GetSMTPConfig(context.Background())
	if err != nil {
		t.Fatalf("GetSMTPConfig: expected the default config, got %v", err)
	}
	if config.Port != 587 || config.FromName != "VexGo" {
		t.Errorf("GetSMTPConfig: expected first-run defaults, got %+v", config)
	}
}
