package llmconfig

import "testing"

func TestLoadOpenAIConfig(t *testing.T) {
	env := map[string]string{
		"MODEL_PROVIDER": "openai",
		"MODEL_BASE_URL": "https://models.example.com/v1",
		"MODEL_API_KEY":  "secret",
		"MODEL_NAME":     "ops-model",
	}

	got, err := LoadFromMap(env)
	if err != nil {
		t.Fatalf("LoadFromMap returned error: %v", err)
	}

	if got.Provider != ProviderOpenAI {
		t.Fatalf("Provider = %q, want %q", got.Provider, ProviderOpenAI)
	}
	if got.Timeout.String() != "30s" {
		t.Fatalf("Timeout = %s, want 30s", got.Timeout)
	}
}

func TestLoadAnthropicConfig(t *testing.T) {
	env := map[string]string{
		"MODEL_PROVIDER": "anthropic",
		"MODEL_BASE_URL": "https://claude.example.com",
		"MODEL_API_KEY":  "secret",
		"MODEL_NAME":     "claude-compatible",
		"MODEL_TIMEOUT":  "7s",
	}

	got, err := LoadFromMap(env)
	if err != nil {
		t.Fatalf("LoadFromMap returned error: %v", err)
	}

	if got.Provider != ProviderAnthropic {
		t.Fatalf("Provider = %q, want %q", got.Provider, ProviderAnthropic)
	}
	if got.Timeout.String() != "7s" {
		t.Fatalf("Timeout = %s, want 7s", got.Timeout)
	}
}

func TestLoadRejectsMissingRequiredConfig(t *testing.T) {
	_, err := LoadFromMap(map[string]string{"MODEL_PROVIDER": "openai"})
	if err == nil {
		t.Fatal("LoadFromMap returned nil error for missing required config")
	}
}

func TestLoadRejectsUnknownProvider(t *testing.T) {
	_, err := LoadFromMap(map[string]string{
		"MODEL_PROVIDER": "unknown",
		"MODEL_BASE_URL": "https://models.example.com/v1",
		"MODEL_API_KEY":  "secret",
		"MODEL_NAME":     "ops-model",
	})
	if err == nil {
		t.Fatal("LoadFromMap returned nil error for unknown provider")
	}
}
