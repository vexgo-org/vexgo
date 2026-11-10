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
	"github.com/vexgo-org/vexgo/backend/internal/secrets"
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
		Themes: public.NewService(public.Deps{DB: db, BaseURL: "http://localhost", DataDir: t.TempDir()}),
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

func TestGetAIConfig_Default(t *testing.T) {
	svc, _ := newTestService(t)
	config, err := svc.GetAIConfig(context.Background())
	if err != nil {
		t.Fatalf("GetAIConfig error: %v", err)
	}
	if config.Provider != "openai" || config.ModelName != "gpt-3.5-turbo" {
		t.Errorf("expected default AI config, got %+v", config)
	}
}

func TestUpdateAIConfig_KeyPreserved(t *testing.T) {
	svc, db := newTestService(t)

	config, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     true,
		Provider:    "openai",
		ApiEndpoint: "https://api.openai.com",
		ApiKey:      "sk-secret",
		ModelName:   "gpt-4",
	})
	if err != nil {
		t.Fatalf("UpdateAIConfig error: %v", err)
	}
	if config.ApiKey != "" {
		t.Errorf("expected api key masked in response")
	}

	// update without key preserves it
	if _, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     false,
		Provider:    "openai",
		ApiEndpoint: "https://api.openai.com",
		ApiKey:      "",
		ModelName:   "gpt-4",
	}); err != nil {
		t.Fatalf("UpdateAIConfig error: %v", err)
	}
	var stored model.AIConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if stored.ApiKey != "sk-secret" {
		t.Errorf("expected api key preserved, got %q", stored.ApiKey)
	}
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

// TC-ENC-012: with a cipher wired, the AI API key is stored as enc:v1:
// ciphertext while responses stay masked.
func TestUpdateAIConfig_EncryptsApiKeyWithCipher(t *testing.T) {
	svc, db, cipher := newTestServiceWithCipher(t)

	resp, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     true,
		Provider:    "openai",
		ApiEndpoint: "https://api.example.com/v1",
		ApiKey:      "sk-test-key",
		ModelName:   "gpt-3.5-turbo",
	})
	if err != nil {
		t.Fatalf("UpdateAIConfig error: %v", err)
	}
	if resp.ApiKey != "" {
		t.Errorf("expected API key masked in response, got %q", resp.ApiKey)
	}

	var stored model.AIConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	if !secrets.IsEncrypted(stored.ApiKey) {
		t.Errorf("expected stored API key to carry the encrypted marker, got %q", stored.ApiKey)
	}

	decrypted, err := cipher.Decrypt(stored.ApiKey)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != "sk-test-key" {
		t.Errorf("expected stored ciphertext to decrypt to the original, got %q", decrypted)
	}
}

// TC-ENC-013: an update without an API key keeps the stored ciphertext, and
// the preserved value still decrypts.
func TestUpdateAIConfig_EmptyApiKeyKeepsStoredValueDecryptable(t *testing.T) {
	svc, db, cipher := newTestServiceWithCipher(t)

	if _, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     true,
		Provider:    "openai",
		ApiEndpoint: "https://api.example.com/v1",
		ApiKey:      "sk-test-key",
		ModelName:   "gpt-3.5-turbo",
	}); err != nil {
		t.Fatalf("UpdateAIConfig error: %v", err)
	}

	if _, err := svc.UpdateAIConfig(context.Background(), AIConfigRequest{
		Enabled:     false,
		Provider:    "openai",
		ApiEndpoint: "https://api2.example.com/v1",
		ApiKey:      "",
		ModelName:   "gpt-4",
	}); err != nil {
		t.Fatalf("UpdateAIConfig (no key) error: %v", err)
	}

	var stored model.AIConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	decrypted, err := cipher.Decrypt(stored.ApiKey)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != "sk-test-key" {
		t.Errorf("expected preserved API key to stay decryptable, got %q", decrypted)
	}
}

// TC-ENC-014: an undecryptable stored AI key (wrong/rotated key) must not
// crash the server; the key is treated as unset and the test call reports the
// incomplete-configuration error.
func TestTestAI_UndecryptableKeyTreatedAsUnset(t *testing.T) {
	svc, db, _ := newTestServiceWithCipher(t)

	// Store a value encrypted under a different passphrase.
	other, err := secrets.New("other-key")
	if err != nil {
		t.Fatalf("failed to create other cipher: %v", err)
	}
	encrypted, err := other.Encrypt("sk-rotated")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if err := db.Create(&model.AIConfig{
		Enabled:     true,
		Provider:    "openai",
		ApiEndpoint: "https://api.example.com/v1",
		ApiKey:      encrypted,
		ModelName:   "gpt-3.5-turbo",
	}).Error; err != nil {
		t.Fatalf("failed to seed AI config: %v", err)
	}

	if _, err := svc.TestAI(context.Background()); !errors.Is(err, ErrAIIncomplete) {
		t.Errorf("expected ErrAIIncomplete for undecryptable key, got %v", err)
	}
}
